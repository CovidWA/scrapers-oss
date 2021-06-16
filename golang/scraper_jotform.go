package csg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const ScraperTypeJotform = "jotform"

const JotformParamKeyNamePattern = "namepattern"
const JotformParamKeyPrepmod = "prepmod"
const JotformTimeslotsUrl = "https://%s/server.php?action=getAppointments&formID=%s&timezone=America%%2FLos_Angeles%%20(GMT%s)&ncTz=%d&firstAvailableDates"

var JotformUrlPattern = regexp.MustCompile(`(?i)https?://[^\.\s"]+\.jotform\.com(?:/jsform)?/[0-9]+`)
var JotformDomainPattern = regexp.MustCompile(`(?i)[a-z0-9]+\.jotform\.com`)
var JotformIdPattern = regexp.MustCompile(`(?i)<input type="hidden" name="formID" value="([0-9]+)"\s*/>`)

type ScraperJotform struct {
	ScraperName      string
	Url              string
	AlternateUrl     string
	LimitedThreshold int
	NamePattern      *regexp.Regexp
	Prepmod          *ScraperPrepmod
	Configured       bool
}

type ScraperJotformFactory struct {
}

func (sf *ScraperJotformFactory) Type() string {
	return ScraperTypeJotform
}

func (sf *ScraperJotformFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	if name == "jotform" {
		//scrapers from airtable

		clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^jotform_.+$`))
		if err != nil {
			return nil, err
		}
		scrapers := make(map[string]Scraper)

		for _, clinic := range clinics {
			scraper := new(ScraperJotform)
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
		//scarpers from yaml
		scraper := new(ScraperJotform)
		scraper.ScraperName = name

		return map[string]Scraper{name: scraper}, nil
	}
}

func (s *ScraperJotform) Type() string {
	return ScraperTypeJotform
}

func (s *ScraperJotform) Name() string {
	return s.ScraperName
}

func (s *ScraperJotform) Configure(params map[string]interface{}) error {
	if s.Configured {
		//only configure once, either from airtable or .yaml
		return nil
	}

	s.Configured = true
	s.LimitedThreshold = config.LimitedThreshold
	s.NamePattern = getPatternOptional(params, JotformParamKeyNamePattern)

	if s.NamePattern != nil {
		Log.Debugf("%s: Configured name pattern: %v", s.Name(), s.NamePattern)
	}

	if getBool(params, JotformParamKeyPrepmod) {
		s.Prepmod = new(ScraperPrepmod)
		s.Prepmod.ScraperName = fmt.Sprintf("%s_prepmod", s.Name())
	}

	url, exists := getStringOptional(params, "url")
	if exists && len(url) > 0 {
		s.Url = url
	}

	return nil
}

type JotformTimeslotsAPIResp struct {
	Groups   map[string]interface{} `json:"content"`
	Success  bool                   `json:"success"`
	Duration string                 `json:"duration"`
}

func (s *ScraperJotform) Scrape() (status Status, tags TagSet, body []byte, err error) {
	return s.ScrapeUrls(s.AlternateUrl, s.Url)
}

func (s *ScraperJotform) ScrapeUrls(urls ...string) (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	var jotformUrls []string
	// STEP 1: Get Jotform urls out of initial url
	jotformUrls, body, err = ExtractScrapeUrls(s.Name(), JotformUrlPattern, urls...)
	if err != nil {
		Log.Warnf("%v", err)
		status = StatusNo
		return
	}

	Log.Debugf("Found %d jotform url(s)", len(jotformUrls))

	hasLimitedAvail := false
	hasPossibleAvail := false

	for _, url := range jotformUrls {
		url = strings.ReplaceAll(url, "/jsform", "")

		jotformId, formBody, _ := ExtractScrapeUrl(s.Name(), JotformIdPattern, url)
		if len(jotformId) == 0 {
			Log.Warnf("%s: Could not parse id from %s", s.Name(), url)
			continue
		}

		if s.NamePattern != nil {
			if !s.NamePattern.Match(formBody) {
				Log.Debugf("%s: Page body did not match name pattern, skipping...", s.Name())
				continue
			} else {
				Log.Debugf("%s: Page body matched name pattern: %v", s.Name(), s.NamePattern)
			}
		}

		jotformDomain := JotformDomainPattern.FindString(url)
		if len(jotformDomain) == 0 {
			status = StatusPossible
			Log.Errorf("%s: Could not parse domain from %s", s.Name(), url)
			return
		}

		Log.Debugf("%s: found jotform id: %s, domain: %s", s.Name(), jotformId, jotformDomain)

		if jotformDomain == "form.jotform.com" {
			//hack to make some sites work
			jotformDomain = "hipaa.jotform.com"
		}

		if s.Prepmod != nil {
			status, _, body, err = s.Prepmod.ScrapeUrls(url)
		} else {
			status, body, err = s.ScrapeJotformAPI(jotformDomain, jotformId)
		}
		if err != nil {
			return
		}

		if status == StatusYes {
			return
		} else if status == StatusLimited {
			hasLimitedAvail = true
		} else if status == StatusPossible {
			hasPossibleAvail = true
		}
	}

	if hasLimitedAvail {
		status = StatusLimited
	} else if hasPossibleAvail {
		status = StatusPossible
	} else {
		status = StatusNo
	}

	return
}

func (s *ScraperJotform) ScrapeJotformAPI(jotformDomain string, jotformId string) (status Status, body []byte, err error) {
	status = StatusUnknown

	// STEP 2: Get time slots
	loc, _ := time.LoadLocation("America/Los_Angeles")
	now := time.Now().In(loc)
	_, offset := now.Zone()
	var offsetStr string
	if offset < 0 {
		offsetStr = fmt.Sprintf("-%02d%%3A00", -offset/3600)
	} else {
		offsetStr = fmt.Sprintf("%02d%%3A00", offset/3600)
	}

	endpoint := new(Endpoint)
	endpoint.Url = fmt.Sprintf(JotformTimeslotsUrl, jotformDomain, jotformId, offsetStr, now.Unix())
	endpoint.Method = "GET"
	body, _, err = endpoint.FetchCached(s.Name())
	if err != nil {
		return
	}

	jsonData := new(JotformTimeslotsAPIResp)
	err = json.Unmarshal(body, jsonData)
	if err != nil {
		return
	}

	availableSlots := 0
	extraDebug := false

	for groupId, group := range jsonData.Groups {
		if extraDebug {
			Log.Debugf("%s: Group id: %v", s.Name(), groupId)
		}
		if dates, ok := group.(map[string]interface{}); ok {
			for dateKey, date := range dates {
				if extraDebug {
					Log.Debugf("%s:  Date: %v", dateKey)
				}
				if slots, ok := date.(map[string]interface{}); ok {
					for slotKey, slot := range slots {
						if extraDebug {
							Log.Debugf("%s:    Time: %v", slotKey)
						}
						if available, ok := slot.(bool); ok {
							if available {
								availableSlots++
							}
						} else {
							Log.Warnf("Available not bool")
						}
					}
				} else if slots, ok := date.(bool); ok {
					if slots {
						Log.Warnf("Slots bool but true???")
					}
				} else if slots, ok := date.([]interface{}); ok {
					if len(slots) > 0 {
						Log.Warnf("Slots array but non-empty???")
					}
				} else {
					Log.Warnf("Slots not map, array, or bool")
				}
			}
		} else {
			Log.Warnf("Dates not map")
		}
	}

	Log.Debugf("%s: Total availability: %d", s.Name(), availableSlots)

	if availableSlots > s.LimitedThreshold {
		status = StatusYes
	} else if availableSlots > 0 {
		status = StatusLimited
	} else {
		status = StatusNo
	}

	return
}
