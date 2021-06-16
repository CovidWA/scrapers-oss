package csg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const ScraperTypePrepmod = "prepmod"

var PrepmodUrlPattern = regexp.MustCompile(`(?i)https?:(?:/|(?:\\u002F)){2}prepmod.doh.wa.gov(?:/|(?:\\u002F))+(?:[^\s"'<]+)`)
var PrepmodPagePattern = regexp.MustCompile(`Multi-State Partnership for Prevention`)
var PrepmodUnavailablePattern = regexp.MustCompile(`(?i)div class="danger-alert"`)
var PrepmodWaitlistPattern = regexp.MustCompile(`(?i)Add to Waiting List`)
var PrepmodAvailablePattern = regexp.MustCompile(`(?mi)([0-9]+)\s+appointments available`)

type ScraperPrepmod struct {
	ScraperName     string
	Url             string
	AlternateUrl    string
	LimitedThresold int
}

type ScraperPrepmodFactory struct {
}

func (sf *ScraperPrepmodFactory) Type() string {
	return ScraperTypePrepmod
}

func (sf *ScraperPrepmodFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	if name == "prepmod" {
		//scrapers from airtable

		clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^prepmod_.+$`))
		if err != nil {
			return nil, err
		}
		scrapers := make(map[string]Scraper)

		for _, clinic := range clinics {
			scraper := new(ScraperPrepmod)
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
		scraper := new(ScraperPrepmod)
		scraper.ScraperName = name

		return map[string]Scraper{name: scraper}, nil
	}
}

func (s *ScraperPrepmod) Type() string {
	return ScraperTypePrepmod
}

func (s *ScraperPrepmod) Name() string {
	return s.ScraperName
}

func (s *ScraperPrepmod) Configure(params map[string]interface{}) error {
	s.LimitedThresold = config.LimitedThreshold

	url, exists := getStringOptional(params, "url")
	if exists && len(url) > 0 {
		s.Url = url
	}

	return nil
}

func (s *ScraperPrepmod) Scrape() (status Status, tags TagSet, body []byte, err error) {
	return s.ScrapeUrls(s.AlternateUrl, s.Url)
}

func (s *ScraperPrepmod) ScrapeUrls(urls ...string) (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	urls, body, err = ExtractScrapeUrls(s.Name(), PrepmodUrlPattern, urls...)
	if err != nil {
		return
	}

	for _, url := range urls {
		url = strings.ReplaceAll(url, `\u002F`, `/`)
		url = strings.ReplaceAll(url, `&amp;`, `&`)

		endpoint := new(Endpoint)
		endpoint.Url = url
		endpoint.Method = "GET"
		body, _, err = endpoint.FetchCached(s.Name())
		if err != nil {
			return
		}

		if !PrepmodPagePattern.Match(body) {
			Log.Errorf("%s: page did not match Prepmod appointment page pattern", s.Name())
			if status != StatusYes && status != StatusLimited && status != StatusWaitList {
				status = StatusPossible
			}
		} else {

			var appointmentCount, parsedCount int
			if matches := PrepmodAvailablePattern.FindAllStringSubmatch(string(body), -1); len(matches) > 0 {
				for _, match := range matches {
					if len(match) < 2 {
						err = fmt.Errorf("No submatch for pattern %v", PrepmodAvailablePattern)
						return
					}

					parsedCount, err = strconv.Atoi(match[1])
					if err != nil {
						return
					}

					if parsedCount <= 0 {
						continue
					}

					appointmentCount += parsedCount
				}

				Log.Debugf("%s: appointment count: %d", s.Name(), appointmentCount)
				if appointmentCount > s.LimitedThresold {
					status = StatusYes
				} else if appointmentCount > 0 {
					if status != StatusYes {
						status = StatusLimited
					}
				} else {
					Log.Errorf("%s: Found matching available patterns but appointment count was < 0", s.Name())
					if status != StatusYes && status != StatusLimited && status != StatusWaitList {
						status = StatusPossible
					}
				}
			} else if PrepmodUnavailablePattern.Match(body) {
				if status != StatusYes && status != StatusLimited && status != StatusWaitList && status != StatusPossible {
					status = StatusNo
				}
			} else if PrepmodWaitlistPattern.Match(body) {
				if status != StatusYes && status != StatusLimited {
					status = StatusWaitList
				}
			} else {
				if status != StatusYes && status != StatusLimited && status != StatusWaitList {
					status = StatusPossible
				}
			}

		}
	}

	return
}
