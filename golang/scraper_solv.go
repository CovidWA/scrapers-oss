package csg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

const ScraperTypeSolv = "solv_health"

const SolvParamKeyNamePattern = "namepattern"
const SolvAPIUrl = "https://d2ez0zkh6r5hup.cloudfront.net/v1/locations/%s/slots?on_date=%s&origin=react_mobile_app"
const SolvAPIv2Url = "https://d2ez0zkh6r5hup.cloudfront.net/v2/locations/%s?origin=booking_widget"

const SolvAuth = "Bearer 90dd1fcea0074e7eb4b11e3753a0a334"

const SolvApptDateFormat = "2006-01-02T15:04:05-07:00"

var SolvIdPattern = regexp.MustCompile(`(?i)https?://www\.solvhealth\.com/book-online/([a-z0-9]{6})`)

type ScraperSolvHealth struct {
	ScraperName      string
	Url              string
	AlternateUrl     string
	LimitedThreshold int
	NamePattern      *regexp.Regexp
}

type ScraperSolvHealthFactory struct {
}

func (sf *ScraperSolvHealthFactory) Type() string {
	return ScraperTypeSolv
}

func (sf *ScraperSolvHealthFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	if name == "solv" {
		//scrapers from airtable

		clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^solv_.+$`))
		if err != nil {
			return nil, err
		}
		scrapers := make(map[string]Scraper)

		for _, clinic := range clinics {
			scraper := new(ScraperSolvHealth)
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
		scraper := new(ScraperSolvHealth)
		scraper.ScraperName = name

		return map[string]Scraper{name: scraper}, nil
	}
}

func (s *ScraperSolvHealth) Type() string {
	return ScraperTypeSolv
}

func (s *ScraperSolvHealth) Name() string {
	return s.ScraperName
}

func (s *ScraperSolvHealth) Configure(params map[string]interface{}) error {
	s.LimitedThreshold = config.LimitedThreshold
	namePattern := getPatternOptional(params, SolvParamKeyNamePattern)

	if namePattern != nil {
		s.NamePattern = namePattern
		Log.Debugf("%s: Configured name pattern: %v", s.Name(), s.NamePattern)
	}

	url, exists := getStringOptional(params, "url")
	if exists && len(url) > 0 {
		s.Url = url
	}
	return nil
}

func (s *ScraperSolvHealth) Scrape() (status Status, tags TagSet, body []byte, err error) {
	return s.ScrapeUrls(s.AlternateUrl, s.Url)
}

func (s *ScraperSolvHealth) ScrapeUrls(urls ...string) (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	solvIds, body, err := ExtractScrapeUrls(s.Name(), SolvIdPattern, urls...)
	if err != nil {
		return
	}

	for _, solvId := range solvIds {
		status, tags, body, err = s.ScrapeSolvId(solvId)
		if err != nil || status == StatusYes || status == StatusPossible {
			return
		}
	}

	status = StatusNo
	return
}

type SolvAPIResp struct {
	Data SolvAPIData `json:"data"`
}

type SolvAPIData struct {
	BeyondEnabled  bool    `json:"is_beyond_next_day_appointments_enabled"`
	BeyondLimit    float64 `json:"beyond_next_day_limit"`
	Name           string  `json:"name"`
	DisplayName1   string  `json:"display_name_primary"`
	DisplayName2   string  `json:"display_name_secondary"`
	DisplayName3   string  `json:"display_name_tertiary"`
	DisplayNameAlt string  `json:"display_name_alternate"`
}

func (s *ScraperSolvHealth) ScrapeSolvId(solvId string) (status Status, tags TagSet, body []byte, err error) {
	if len(solvId) != 6 {
		Log.Warnf("%s: invalid solv id: %s", s.Name(), solvId)
		status = StatusNo
		return
	} else {
		Log.Debugf("%s: trying solv id: %s", s.Name(), solvId)
	}

	loc, _ := time.LoadLocation("America/Los_Angeles")
	now := time.Now().In(loc)

	auth := Header{
		Name:  "Authorization",
		Value: SolvAuth,
	}

	cookedUrl := fmt.Sprintf(SolvAPIv2Url, solvId)
	endpoint := new(Endpoint)
	endpoint.Url = cookedUrl
	endpoint.Method = "GET"
	endpoint.Body = ""
	endpoint.Headers = []Header{auth}

	body, _, err = endpoint.FetchCached(s.Name())
	if err != nil {
		return
	}

	solvAPIResp := new(SolvAPIResp)

	if err = json.Unmarshal(body, solvAPIResp); err != nil {
		return
	}

	if s.NamePattern != nil {
		match := false
		if s.NamePattern.MatchString(solvAPIResp.Data.Name) {
			match = true
		} else if s.NamePattern.MatchString(solvAPIResp.Data.DisplayName1) {
			match = true
		} else if s.NamePattern.MatchString(solvAPIResp.Data.DisplayName2) {
			match = true
		} else if s.NamePattern.MatchString(solvAPIResp.Data.DisplayName3) {
			match = true
		} else if s.NamePattern.MatchString(solvAPIResp.Data.DisplayNameAlt) {
			match = true
		}

		if !match {
			Log.Debugf("%s: no names matched pattern '%v', skipping...", s.Name(), s.NamePattern)
			status = StatusNo
			return
		}
	}

	daysLookahead := int(solvAPIResp.Data.BeyondLimit) / 86400

	if !solvAPIResp.Data.BeyondEnabled {
		daysLookahead = 2
	}

	totalAvailability := 0
	extraDebug := false

	for i := 0; i < daysLookahead; i++ {
		date := now.AddDate(0, 0, i)
		dateStr := date.Format("2006-01-02")
		cookedUrl = fmt.Sprintf(SolvAPIUrl, solvId, dateStr)

		endpoint.Url = cookedUrl

		body, _, err = endpoint.FetchCached(s.Name())
		if err != nil {
			return
		}

		var jsonData map[string]interface{}

		if err = json.Unmarshal(body, &jsonData); err != nil {
			Log.Errorf("%v", err)
			err = nil
		}

		slotArray := jsonData["data"].([]interface{})
		for _, slotRaw := range slotArray {
			slotCooked := slotRaw.(map[string]interface{})
			availability := slotCooked["availability"].(float64)
			busy := slotCooked["busy"].(bool)
			is_reservations_disabled := slotCooked["is_reservations_disabled"].(bool)
			appointment_date := slotCooked["appointment_date"].(string)

			apptTime, err2 := time.Parse(SolvApptDateFormat, appointment_date)
			if err2 != nil {
				Log.Warnf("%v", err2)
				continue
			}

			if availability > 0 && !busy && !is_reservations_disabled {
				if extraDebug {
					Log.Debugf("%s: Availability detected at %v: %d", s.Name(), apptTime, int(availability))
				}
				totalAvailability += int(availability)
			}
		}
	}

	Log.Debugf("%s: Total availability: %d", s.Name(), totalAvailability)

	if totalAvailability > s.LimitedThreshold {
		status = StatusYes
	} else if totalAvailability > 0 {
		status = StatusLimited
	} else {
		status = StatusNo
	}

	return
}
