package csg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const ScraperTypeAthena = "athena"

var AthenaUrlPattern = regexp.MustCompile(`(?i)https://[^/]+\.athena\.io/\?(?:departmentId|locationId)=[a-f0-9\-]+`)
var AthenaDeptIdPattern = regexp.MustCompile(`[0-9]+\-[0-9]+$`)
var AthenaLocationIdPattern = regexp.MustCompile(`[a-f0-9\-]+$`)
var AthenaTokenPattern = regexp.MustCompile(`"token":"([^"]+)"`)

const AthenaDeptIdUrl = "https://framework-backend.scheduling.athena.io/locationId?locationId=%s"
const AthenaTokenUrl = "https://framework-backend.scheduling.athena.io/t"

//TODO: support practitioner id
const AthenaSchedTokenUrl = "https://framework-backend.scheduling.athena.io/u?locationId=%s&practitionerId=&contextId=%s"

const AthenaAPIUrl = "https://framework-backend.scheduling.athena.io/v1/graphql"
const AthenaGetFiltersReq = `{"operationName":"GetFilters","variables":{"locationIds":["%s"],"practitionerIds":[]},"query":"query GetFilters($locationIds: [String!]!, $practitionerIds: [String!]!) {\n  getFilters(locationIds: $locationIds, practitionerIds: $practitionerIds) {\n    ...Filters\n    __typename\n  }\n}\n\nfragment Filters on Filters {\n  patientNewness {\n    text\n    value\n    __typename\n  }\n  specialties {\n    text\n    value\n    patientNewness\n    __typename\n  }\n  visitReasons {\n    text\n    value\n    patientNewness\n    specialties\n    __typename\n  }\n}\n"}`
const AthenaGetAvailReq = `{"operationName":"SearchAvailabilityDates","variables":{"locationIds":["%s"],"practitionerIds":[],"specialty":"%s","serviceTypeTokens":["%s"],"startAfter":"%s","startBefore":"%s"},"query":"query SearchAvailabilityDates($locationIds: [String!], $practitionerIds: [String!], $specialty: String, $serviceTypeTokens: [String!]!, $startAfter: String!, $startBefore: String!, $visitType: VisitType) {\n  searchAvailabilityDates(locationIds: $locationIds, practitionerIds: $practitionerIds, specialty: $specialty, serviceTypeTokens: $serviceTypeTokens, startAfter: $startAfter, startBefore: $startBefore, visitType: $visitType) {\n    date\n    availability\n    __typename\n  }\n}\n"}`
const AthenaTimeFormat = "2006-01-02T15:04:05-07:00"

type ScraperAthena struct {
	ScraperName  string
	Url          string
	AlternateUrl string
	NamePattern  *regexp.Regexp
	Configured   bool
}

type ScraperAthenaFactory struct {
}

func (sf *ScraperAthenaFactory) Type() string {
	return ScraperTypeAthena
}

func (sf *ScraperAthenaFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	if name == "athena" {
		//scrapers from airtable
		clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^athena_.+$`))
		if err != nil {
			return nil, err
		}
		scrapers := make(map[string]Scraper)

		for _, clinic := range clinics {
			scraper := new(ScraperAthena)
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
		scraper := new(ScraperAthena)
		scraper.ScraperName = name

		return map[string]Scraper{name: scraper}, nil
	}
}

func (s *ScraperAthena) Type() string {
	return ScraperTypeAthena
}

func (s *ScraperAthena) Name() string {
	return s.ScraperName
}

func (s *ScraperAthena) Configure(params map[string]interface{}) error {
	if s.Configured {
		//only configure once, either from airtable or .yaml
		return nil
	}

	s.Configured = true
	s.NamePattern = getPatternOptional(params, SigneticParamKeyNamePattern)

	if s.NamePattern != nil {
		Log.Debugf("%s: Configured name pattern: %v", s.Name(), s.NamePattern)
	}

	url, exists := getStringOptional(params, "url")
	if exists && len(url) > 0 {
		s.Url = url
	}

	return nil
}

type AthenaAPIResp struct {
	Data AthenaAPIData `json:"data"`
}

type AthenaAPIData struct {
	GetFilters   AthenaGetFiltersResp `json:"getFilters"`
	Availability []AthenaAvailability `json:"searchAvailabilityDates"`
}

type AthenaGetFiltersResp struct {
	VisitReasons []AthenaVisitReason `json:"visitReasons"`
}

type AthenaVisitReason struct {
	Text        string   `json:"text"`
	Value       string   `json:"value"`
	Specialties []string `json:"specialties"`
}

type AthenaAvailability struct {
	Date      string `json:"date"`
	Available bool   `json:"availability"`
	Type      string `json:"__typename"`
}

func (s *ScraperAthena) Scrape() (status Status, tags TagSet, body []byte, err error) {
	return s.ScrapeUrls(s.AlternateUrl, s.Url)
}

func (s *ScraperAthena) ScrapeUrls(urls ...string) (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	var url, token, schedToken string
	url, body, err = ExtractScrapeUrl(s.Name(), AthenaUrlPattern, urls...)
	if err != nil {
		return
	}

	deptId := AthenaDeptIdPattern.FindString(url)
	if len(deptId) == 0 {
		locationId := AthenaLocationIdPattern.FindString(url)
		if len(locationId) > 0 {
			deptId, body, err = ExtractScrapeUrl(s.Name(), AthenaDeptIdPattern, fmt.Sprintf(AthenaDeptIdUrl, locationId))
			if err != nil {
				return
			}
		}
	}

	if len(deptId) == 0 {
		status = StatusPossible
		Log.Errorf("%s: Coulld not parse department or location id from %s", s.Name(), url)
		return
	}

	contextId := strings.Split(deptId, "-")[0]

	token, body, err = ExtractScrapeUrl(s.Name(), AthenaTokenPattern, AthenaTokenUrl)
	if err != nil {
		return
	}

	schedTokenUrl := fmt.Sprintf(AthenaSchedTokenUrl, deptId, contextId)
	schedToken, body, err = ExtractScrapeUrl(s.Name(), AthenaTokenPattern, schedTokenUrl)
	if err != nil {
		return
	}

	var apiResp *AthenaAPIResp
	apiResp, err = s.MakeAPIRequest(fmt.Sprintf(AthenaGetFiltersReq, deptId), token, schedToken)
	if err != nil {
		return
	}

	specialties := make(map[string][]string)

	for _, visitReason := range apiResp.Data.GetFilters.VisitReasons {
		if s.NamePattern != nil && !s.NamePattern.MatchString(visitReason.Text) {
			continue
		}

		for _, specialty := range visitReason.Specialties {
			if _, exists := specialties[specialty]; !exists {
				specialties[specialty] = make([]string, 0)
			}

			specialties[specialty] = append(specialties[specialty], visitReason.Value)
		}
	}

	startDateStr := time.Now().Format(AthenaTimeFormat)
	endDateStr := time.Now().AddDate(0, 3, 0).Format(AthenaTimeFormat) //3 months ahead

	for specialty, visitReasons := range specialties {
		reasonList := strings.Join(visitReasons, `","`)

		req := fmt.Sprintf(AthenaGetAvailReq, deptId, specialty, reasonList, startDateStr, endDateStr)
		apiResp, err = s.MakeAPIRequest(req, token, schedToken)
		if err != nil {
			return
		}

		for _, avail := range apiResp.Data.Availability {
			if avail.Type != "Availability" {
				continue
			}

			if avail.Available {
				status = StatusYes
				return
			}
		}
	}

	status = StatusNo
	return
}

func (s *ScraperAthena) MakeAPIRequest(req string, token string, schedToken string) (*AthenaAPIResp, error) {
	endpoint := new(Endpoint)
	endpoint.Method = "POST"
	endpoint.Url = AthenaAPIUrl
	endpoint.Body = req
	endpoint.Headers = []Header{
		Header{
			Name:  "Accept-Encoding",
			Value: "gzip, br",
		},
		Header{
			Name:  "authorization",
			Value: fmt.Sprintf("Bearer %s", token),
		},
		Header{
			Name:  "content-type",
			Value: "application/json",
		},
		Header{
			Name:  "x-scheduling-jwt",
			Value: schedToken,
		},
	}

	body, _, err := endpoint.FetchCached(s.Name())
	if err != nil {
		return nil, err
	}

	apiResp := new(AthenaAPIResp)
	err = json.Unmarshal(body, apiResp)
	if err != nil {
		return nil, err
	}

	return apiResp, nil
}
