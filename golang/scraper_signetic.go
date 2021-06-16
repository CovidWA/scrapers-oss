package csg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const ScraperTypeSignetic = "signetic"

const SigneticParamKeyNamePattern = "namepattern"
const SigneticParamKeyLimitedThreshold = "threshold" //minimum amount of total appts to report Yes.  Will report possible if the number of appointments is below this but above 0

const SigneticOrgUrl = "https://api.%s/api/organization/%s"
const SigneticClinicsUrl = "https://api.%s/api/clinics?organizationId=%s"
const SigneticClinicNoOrgUrl = "https://api.%s/api/clinics"
const SigneticTimeslotsUrl = "https://api.%s/api/clinics/%s/clinic-days?healthCareServiceId=%s"
const SigneticTimeslotsNoOrgUrl = "https://api.%s/api/clinicdays/clinics/%s"

const SigneticStatusLive = 153940000

var SigneticHomeUrlPattern = regexp.MustCompile(`(?i)https?://[^\.\s"]+\.signetic\.com/home(?:/[a-z0-9\-]*)?`)
var SigneticDomainPattern = regexp.MustCompile(`(?i)[a-z0-9]+\.signetic\.com`)
var SigneticOrgIdPattern = regexp.MustCompile(`(?i)(?:[a-z0-9]+-){4,}[a-z0-9]+$`)
var SigneticNotAllowedPattern = regexp.MustCompile(`"error":\{"code":405,"message":"Method Not Allowed"`)

type ScraperSignetic struct {
	ScraperName      string
	Url              string
	AlternateUrl     string
	NamePattern      *regexp.Regexp
	LimitedThreshold int
	Configured       bool
}

type ScraperSigneticFactory struct {
}

func (sf *ScraperSigneticFactory) Type() string {
	return ScraperTypeSignetic
}

func (sf *ScraperSigneticFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	if name == "signetic" {
		//scrapers from airtable

		clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^signetic_.+$`))
		if err != nil {
			return nil, err
		}
		scrapers := make(map[string]Scraper)

		for _, clinic := range clinics {
			scraper := new(ScraperSignetic)
			scraper.ScraperName = clinic.ApiKey
			scraper.Url = clinic.Url
			scraper.AlternateUrl = clinic.AlternateUrl

			if len(clinic.ScraperConfig) > 0 {
				scraperConfig := make(map[string]interface{})
				err = json.Unmarshal([]byte(clinic.ScraperConfig), &scraperConfig)
				if err != nil {
					Log.Warnf("Error parsing scraper configuration from airtable: %v", err)
				} else {
					err = scraper.Configure(scraperConfig)
					if err != nil {
						Log.Warnf("Error loading scraper configuration from airtable: %v", err)
					}
				}
			}
			scrapers[scraper.Name()] = scraper
		}

		return scrapers, nil
	} else {
		//scrapers from yaml
		scraper := new(ScraperSignetic)
		scraper.ScraperName = name

		return map[string]Scraper{name: scraper}, nil
	}
}

func (s *ScraperSignetic) Type() string {
	return ScraperTypeSignetic
}

func (s *ScraperSignetic) Name() string {
	return s.ScraperName
}

func (s *ScraperSignetic) Configure(params map[string]interface{}) error {
	if s.Configured {
		//only configure once, either from airtable or .yaml
		return nil
	}

	s.Configured = true
	s.NamePattern = getPatternOptional(params, SigneticParamKeyNamePattern)

	if s.NamePattern != nil {
		Log.Debugf("%s: Configured name pattern: %v", s.Name(), s.NamePattern)
	}

	threshold, _ := getFloatOptional(params, SigneticParamKeyLimitedThreshold)
	if threshold > 0 {
		s.LimitedThreshold = int(threshold)
		Log.Debugf("%s: Configured yes threshold: %v", s.Name(), s.LimitedThreshold)
	}

	url, exists := getStringOptional(params, "url")
	if exists && len(url) > 0 {
		s.Url = url
	}

	return nil
}

type SigneticOrgAPIResp struct {
	Name   string `json:"name"`
	Status int    `json:"smvs_site_group_status"`
}

type SigneticClinicsAPIResp struct {
	Name       string         `json:"msemr_name"`
	LocationId string         `json:"msemr_locationid"`
	Status     int            `json:"smvs_locationstatus"`
	Slots      []SigneticSlot `json:"slots"`
}

type SigneticSlot struct {
	ServiceId         string `json:"_smvs_healthcareservice_value"`
	FirstDoseEligible bool   `json:"smvs_eligible_for_first_appointment"`
	FirstDoseAssoc    string `json:"_smvs_associated_first_appointmentid_value"`
	APIVersion        int    `json:"smvs_version"`
	Availability      int    `json:"smvs_available_slot_count"`
}

type SigneticTimeSlotsAPIResp struct {
	SlotId            string             `json:"msemr_slotid"`
	FirstDoseEligible bool               `json:"smvs_eligible_for_first_appointment"`
	FirstDoseAssoc    string             `json:"_smvs_associated_first_appointmentid_value"`
	APIVersion        int                `json:"smvs_version"`
	TimeSlots         []SigneticTimeSlot `json:"timeslots"`
}

type SigneticTimeSlot struct {
	Id           string `json:"id"`
	Time         string `json:"time"`
	Availability int    `json:"slotAvailable"`
}

func (s *ScraperSignetic) Scrape() (status Status, tags TagSet, body []byte, err error) {
	return s.ScrapeUrls(s.AlternateUrl, s.Url)
}

func (s *ScraperSignetic) ScrapeUrls(urls ...string) (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	// STEP 1: Get Signetic org id out of initial url
	homeUrl, body, err := ExtractScrapeUrl(s.Name(), SigneticHomeUrlPattern, urls...)

	if err != nil {
		return
	}

	signeticOrgId := SigneticOrgIdPattern.FindString(homeUrl)
	if len(signeticOrgId) == 0 {
		Log.Warnf("%s: Could not parse signetic org id from %s", s.Name(), homeUrl)
	}

	signeticDomain := SigneticDomainPattern.FindString(homeUrl)
	if len(signeticDomain) == 0 {
		status = StatusPossible
		Log.Errorf("%s: Could not parse domain from %s", s.Name(), homeUrl)
		return
	}

	if len(signeticOrgId) > 0 {
		Log.Debugf("%s: found signetic org id: %s, domain: %s", s.Name(), signeticOrgId, signeticDomain)
		// STEP 1.5: Check if org is enabled

		endpoint := new(Endpoint)
		endpoint.Url = fmt.Sprintf(SigneticOrgUrl, signeticDomain, signeticOrgId)
		endpoint.Method = "GET"
		body, _, err = endpoint.FetchCached(s.Name())
		if err != nil {
			return
		}

		apiResp := new(SigneticOrgAPIResp)
		err = json.Unmarshal(body, &apiResp)
		if err != nil {
			return
		}
		if apiResp.Status != SigneticStatusLive {
			Log.Debugf("%s: org %s is not live, returning no", s.Name(), signeticOrgId)
			status = StatusNo
			return
		}
	} else {
		Log.Debugf("%s: found signetic domain: %s", s.Name(), signeticDomain)
	}

	// STEP 2: Get list of locations/services from org

	endpoint := new(Endpoint)
	if len(signeticOrgId) > 0 {
		endpoint.Url = fmt.Sprintf(SigneticClinicsUrl, signeticDomain, signeticOrgId)
	} else {
		endpoint.Url = fmt.Sprintf(SigneticClinicNoOrgUrl, signeticDomain)
	}
	endpoint.Method = "GET"
	endpoint.AllowedStatusCodes = []int{405}

	body, _, err = endpoint.FetchCached(s.Name())
	if err != nil {
		return
	}

	if SigneticNotAllowedPattern.Match(body) {
		//if api is disabled, just return no
		Log.Warnf("%s: clinic api returned 405 not allowed", s.Name())

		status = StatusNo
		return
	}

	jsonData := make([]SigneticClinicsAPIResp, 0)
	err = json.Unmarshal(body, &jsonData)
	if err != nil {
		return
	}

	var locationServiceIds = make(map[string]bool)
	for _, location := range jsonData {
		if s.NamePattern != nil && !s.NamePattern.MatchString(location.Name) {
			Log.Debugf("%s: name pattern did not match: %s <> %v", s.Name(), location.Name, s.NamePattern)
			continue
		}

		if location.Status != SigneticStatusLive {
			Log.Debugf("%s: location is not live: %s: %v", s.Name(), location.Name, location.Status)
			continue
		}

		locationId := location.LocationId
		if len(location.Slots) > 0 {
			for _, slot := range location.Slots {
				if slot.Availability > 0 {
					if slot.APIVersion == 1 && len(slot.FirstDoseAssoc) > 0 {
						continue
					} else if slot.APIVersion == 0 && !slot.FirstDoseEligible {
						continue
					}

					combinedId := fmt.Sprintf("%s|%s", locationId, slot.ServiceId)
					locationServiceIds[combinedId] = true
				}
			}
		} else if len(signeticOrgId) == 0 {
			//orgless "old" api, there are no slots, just a location
			locationServiceIds[locationId] = true
		}
	}

	if len(signeticOrgId) > 0 {
		Log.Debugf("%s: found %d service(s) for signetic org id: %s", s.Name(), len(locationServiceIds), signeticOrgId)
	} else {
		Log.Debugf("%s: found %d location(s)", s.Name(), len(locationServiceIds))
	}

	// STEP 3: Get timeslots for each location+service, return yes on first timeslot with availability

	totalAvailability := 0

	for combinedId := range locationServiceIds {
		endpoint := new(Endpoint)
		if strings.ContainsRune(combinedId, '|') {
			ids := strings.Split(combinedId, "|")
			endpoint.Url = fmt.Sprintf(SigneticTimeslotsUrl, signeticDomain, ids[0], ids[1])
		} else {
			endpoint.Url = fmt.Sprintf(SigneticTimeslotsNoOrgUrl, signeticDomain, combinedId)
		}
		endpoint.Method = "GET"

		body, _, err = endpoint.FetchCached(s.Name())
		if err != nil {
			return
		}

		jsonData := make([]SigneticTimeSlotsAPIResp, 0)
		err = json.Unmarshal(body, &jsonData)
		if err != nil {
			return
		}

		for _, resp := range jsonData {
			if resp.APIVersion == 1 && len(resp.FirstDoseAssoc) > 0 {
				continue
			} else if resp.APIVersion == 0 && !resp.FirstDoseEligible {
				continue
			}
			Log.Debugf("%s: Counting appts in slot id %s", s.Name(), resp.SlotId)
			for _, timeslot := range resp.TimeSlots {
				if timeslot.Availability > 0 {
					//Log.Debugf("%s: Found availability at location %s, time %s: %d", s.Name(), combinedId, timeslot.Time, timeslot.Availability)

					totalAvailability += timeslot.Availability
				}
			}

			Log.Debugf("%s: %d appts found so far", s.Name(), totalAvailability)
		}
	}

	Log.Debugf("%s: Total availability: %d", s.Name(), totalAvailability)
	if totalAvailability > 0 {
		status = StatusLimited
		if totalAvailability > s.LimitedThreshold {
			status = StatusYes
		}
	} else {
		status = StatusNo
	}

	return
}
