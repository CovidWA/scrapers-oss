package csg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const ScraperTypeSimplyBook = "simplybook"
const SimplyBookParamKeyDomain = "domain"
const SimplyBookParamKeyId = "id"
const SimplyBookParamKeyServiceNamePattern = "service_namepattern"

const SimplyBookPageUrl = `https://%s.simplybook.pro/v2/`
const SimplyBookAPIServiceUrl = `https://%s.simplybook.pro/v2/service/`
const SimplyBookAPITimeSlotUrl = `https://%s.simplybook.pro/v2/booking/time-slots/?from=##TOMORROW_DATE##&to=##NEXTMONTH_DATE##&location=%s&category=&provider=%s&service=%s&count=1&booking_id=`

const SimplyBookCsrfHeaderName = "X-Csrf-Token"

var SimplyBookCsrfPattern = regexp.MustCompile(`(?i)"csrf_token":"([a-f0-9]+)"`)

const SimplyBookCacheTTL = 60

type ScraperSimplyBook struct {
	ScraperName        string
	Domain             string
	Id                 string
	ServiceNamePattern *regexp.Regexp
	LimitedThreshold   int
}

type ScraperSimplyBookFactory struct {
}

func (sf *ScraperSimplyBookFactory) Type() string {
	return ScraperTypeSimplyBook
}

func (sf *ScraperSimplyBookFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	if name == "simplybook" {
		//scrapers from airtable

		clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^simplybook(_[^_]+){2,}$`))
		if err != nil {
			return nil, err
		}
		scrapers := make(map[string]Scraper)

		for _, clinic := range clinics {
			scraper := new(ScraperSimplyBook)
			scraper.ScraperName = clinic.ApiKey

			keyParts := strings.Split(clinic.ApiKey, "_")
			scraper.Domain = strings.Join(keyParts[1:len(keyParts)-1], "_")
			scraper.Id = keyParts[len(keyParts)-1]

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
		scraper := new(ScraperSimplyBook)
		scraper.ScraperName = name

		return map[string]Scraper{name: scraper}, nil
	}
}

func (s *ScraperSimplyBook) Type() string {
	return ScraperTypeSimplyBook
}

func (s *ScraperSimplyBook) Name() string {
	return s.ScraperName
}

func (s *ScraperSimplyBook) Configure(params map[string]interface{}) error {
	s.LimitedThreshold = config.LimitedThreshold

	if serviceNamePattern := getPatternOptional(params, SimplyBookParamKeyServiceNamePattern); serviceNamePattern != nil {
		s.ServiceNamePattern = serviceNamePattern
	}

	if domain, exists := getStringOptional(params, SimplyBookParamKeyDomain); exists {
		s.Domain = domain
	}

	if id, exists := getStringOptional(params, SimplyBookParamKeyId); exists {
		s.Id = id
	}

	return nil
}

type SimplyBookService struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type SimplyBookSlot struct {
	Id    string `json:"id"`
	Date  string `json:"date"`
	Time  string `json:"time"`
	Type  string `json:"type"`
	Avail int    `json:"available_slots"`
}

func (s *ScraperSimplyBook) Scrape() (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	if len(s.Domain) < 1 {
		Log.Errorf("%s: Domain not found in key or configured", s.Name())
		status = StatusPossible
		return
	}

	if len(s.Id) < 1 {
		Log.Errorf("%s: Id not found in key or configured", s.Name())
		status = StatusPossible
		return
	}

	if s.ServiceNamePattern == nil {
		Log.Errorf("%s: %s not configured", s.Name(), SimplyBookParamKeyServiceNamePattern)
		status = StatusPossible
		return
	}

	// STEP 1: GET CSRF Token + cookie
	var token, cookieName, cookieValue string
	token, cookieName, cookieValue, body, err = s.GetTokenAndCookie()
	if err != nil {
		return
	}

	// STEP 2: GET Service ID(s)
	var validServiceIds map[string]string
	validServiceIds, body, err = s.GetServiceIds(token, cookieName, cookieValue)
	if err != nil {
		return
	}

	if len(validServiceIds) < 1 {
		Log.Warnf("%s: could not find any valid service ids that match name pattern '%v'", s.Name(), s.ServiceNamePattern)
		status = StatusNo
		return
	}

	// STEP 3: fetch timeslots for all services and sum
	totalAvail := 0

	for svcId, svcName := range validServiceIds {
		url := replaceMagic(fmt.Sprintf(SimplyBookAPITimeSlotUrl, s.Domain, s.Id, s.Id, svcId))
		slots := make([]SimplyBookSlot, 0)
		body, err = s.FetchAndUnmarshal(url, token, cookieName, cookieValue, &slots)
		if err != nil {
			return
		}

		for _, slot := range slots {
			if slot.Type != "free" {
				continue
			}

			totalAvail += slot.Avail
			if slot.Avail > 0 {
				tags = tags.ParseAndAddVaccineType(svcName)
			}
		}
	}

	Log.Debugf("%s: Availability: %d", s.Name(), totalAvail)

	if totalAvail <= 0 {
		status = StatusNo
	} else if totalAvail > s.LimitedThreshold {
		status = StatusYes
	} else {
		status = StatusLimited
	}

	return
}

func (s *ScraperSimplyBook) FetchAndUnmarshal(url string, token string, cookieName string, cookieValue string, dataPtr interface{}) (body []byte, err error) {
	endpoint := new(Endpoint)
	endpoint.Url = url
	endpoint.Method = "GET"
	endpoint.Headers = []Header{
		Header{
			Name:  "Accept",
			Value: "gzip, br",
		},
		Header{
			Name:  "Cookie",
			Value: fmt.Sprintf("%s=%s", cookieName, cookieValue),
		},
		Header{
			Name:  SimplyBookCsrfHeaderName,
			Value: token,
		},
	}

	body, _, err = endpoint.Fetch(s.Name())
	if err != nil {
		return
	}

	err = json.Unmarshal(body, dataPtr)
	return
}

func (s *ScraperSimplyBook) GetTokenAndCookie() (token string, cookieName string, cookieValue string, body []byte, err error) {
	cacheKey := fmt.Sprintf("simplybook-tokens-%s", s.Domain)
	cookieName = fmt.Sprintf("sess_user_publicv2_%s", s.Domain)

	if cachedTokens, ok := Cache.GetOrLock(cacheKey).([]string); ok {
		return cachedTokens[0], cookieName, cachedTokens[1], nil, nil
	} else {
		defer Cache.Unlock(cacheKey)

		endpoint := new(Endpoint)
		endpoint.Method = "GET"
		endpoint.Url = fmt.Sprintf(SimplyBookPageUrl, s.Domain)
		endpoint.Cookies = make(map[string]string)
		endpoint.CookieWhitelist = []string{"*"}
		body, _, err = endpoint.Fetch(s.Name())
		if err != nil {
			return
		}

		if _, exists := endpoint.Cookies[cookieName]; !exists {
			err = fmt.Errorf("could not get cookie '%s' from response", cookieValue)
			return
		} else {
			cookieValue = endpoint.Cookies[cookieName]
		}

		if match := SimplyBookCsrfPattern.FindStringSubmatch(string(body)); len(match) < 2 {
			err = fmt.Errorf("could not get Csrf token from response")
			return
		} else {
			token = match[1]
		}

		Cache.Put(cacheKey, []string{token, cookieValue}, SimplyBookCacheTTL, -1)

		return
	}
}

func (s *ScraperSimplyBook) GetServiceIds(token string, cookieName string, cookieValue string) (validServices map[string]string, body []byte, err error) {
	cacheKey := fmt.Sprintf("simplybook-services-%s", s.Domain)

	if cachedServiceIds, ok := Cache.GetOrLock(cacheKey).(map[string]string); ok {
		return cachedServiceIds, nil, nil
	} else {
		defer Cache.Unlock(cacheKey)

		url := fmt.Sprintf(SimplyBookAPIServiceUrl, s.Domain)
		services := make([]SimplyBookService, 0)
		body, err = s.FetchAndUnmarshal(url, token, cookieName, cookieValue, &services)
		if err != nil {
			return
		}

		validServices = make(map[string]string)

		for _, service := range services {
			if s.ServiceNamePattern.MatchString(service.Name) {
				Log.Debugf("%s: service id %s (%s) matched name pattern '%v'", s.Name(), service.Id, service.Name, s.ServiceNamePattern)
				validServices[service.Id] = service.Name
			} else {
				Log.Debugf("%s: service id %s (%s) did not match name pattern '%v'", s.Name(), service.Id, service.Name, s.ServiceNamePattern)
			}
		}

		Cache.Put(cacheKey, validServices, SimplyBookCacheTTL, -1)
		return
	}
}
