package csg

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

//Wordpress Simply Schedule Appointments scraper

const ScraperTypeWpSsa = "wpssa"

var WpSsaEmbedUrlPattern = regexp.MustCompile(`(?i)iframe src="(https?://[^/]+/wp-json/ssa/v1/embed-inner\?integration=form[^"]+)"`)
var WpSsaNoncesPattern = regexp.MustCompile(`var ssa \= {"api":([^}]+})`)
var WpSsaApptTypesPattern = regexp.MustCompile(`var ssa_appointment_types \= (.+);`)
var IncapsulaAntiBotPattern = regexp.MustCompile(`(?i)<META NAME\="ROBOTS" CONTENT\="NOINDEX,\s?NOFOLLOW">`)

const WpSsaApptsUrl = "https://%s/wp-json/ssa/v1/appointment_types/%s/availability?start_date_min=%s&start_date_max=%s&_=##CURRENT_TIMESTAMP##"
const WpSsaDateFormat = "2006-01-02 15:04:05"

type ScraperWpSsa struct {
	ScraperName  string
	Url          string
	AlternateUrl string
}

type ScraperWpSsaFactory struct {
}

func (sf *ScraperWpSsaFactory) Type() string {
	return ScraperTypeWpSsa
}

func (sf *ScraperWpSsaFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	SeedRand()

	if name == "wpssa" {
		//scrapers from airtable

		clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`(^wpssa_.+$)`))
		if err != nil {
			return nil, err
		}
		scrapers := make(map[string]Scraper)

		for _, clinic := range clinics {
			scraper := new(ScraperWpSsa)
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
		scraper := new(ScraperWpSsa)
		scraper.ScraperName = name

		return map[string]Scraper{name: scraper}, nil
	}
}

func (s *ScraperWpSsa) Type() string {
	return ScraperTypeWpSsa
}

func (s *ScraperWpSsa) Name() string {
	return s.ScraperName
}

func (s *ScraperWpSsa) Configure(params map[string]interface{}) error {
	url, exists := getStringOptional(params, "url")
	if exists && len(url) > 0 {
		s.Url = url
	}

	return nil
}

type WpSsaApptType struct {
	Id                string `json:"id"`
	BookingStart      string `json:"booking_start_date"`
	BookingEnd        string `json:"booking_end_date"`
	AvailabilityStart string `json:"availability_start_date"`
	AvailabilityEnd   string `json:"availability_end_date"`
}

type WpSsaNonces struct {
	WpNonce     string `json:"nonce"`
	PublicNonce string `json:"public_nonce"`
}

type WpSsaAPIResp struct {
	Code         int         `json:"response_code"`
	Error        string      `json:"error"`
	Appointments []WpSsaAppt `json:"data"`
}

type WpSsaAppt struct {
	Start string `json:"start_date"`
}

func (s *ScraperWpSsa) Scrape() (status Status, tags TagSet, body []byte, err error) {
	return s.ScrapeUrls(s.AlternateUrl, s.Url)
}

func (s *ScraperWpSsa) ScrapeUrls(urls ...string) (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	host := ""
	host, _, err = ExtractScrapeUrl(s.Name(), HostPattern, urls...)
	if err != nil {
		return
	}

	endpoint := new(Endpoint)
	endpoint.Method = "GET"
	endpoint.Headers = []Header{
		Header{
			Name:  "Accept-Encoding",
			Value: "gzip, br",
		},
		Header{
			Name:  "User-Agent",
			Value: "Mozilla/5.0 (Unknown; Linux x86_64) CovidWA.com Vaccine Appointments Finder",
		},
	}

	embedUrl := ""
	for retries := 0; ; retries++ {
		embedUrl, body, err = ExtractScrapeUrlWithEndpoints(s.Name(), WpSsaEmbedUrlPattern, endpoint, nil, s.AlternateUrl, s.Url)
		if err != nil {
			if IncapsulaAntiBotPattern.Match(body) {
				Cache.Clear(endpoint.GenerateCacheKey(s.Name()))

				//just keep trying until we get through
				if retries < 20 {
					time.Sleep(time.Second)
					continue
				}
				Log.Errorf("%s: Could not circumvent anti-bot", s.Name())
			}

			return
		} else {
			Log.Debugf("%s: successful after %d retries", s.Name(), retries)
			break
		}
	}

	embedUrl = strings.ReplaceAll(embedUrl, "&#038;", "&")

	apptTypesJsonStr := ""
	for retries := 0; ; retries++ {
		apptTypesJsonStr, body, err = ExtractScrapeUrlWithEndpoints(s.Name(), WpSsaApptTypesPattern, endpoint, nil, embedUrl)
		if err != nil {
			if IncapsulaAntiBotPattern.Match(body) {
				Cache.Clear(endpoint.GenerateCacheKey(s.Name()))

				if retries < 20 {
					time.Sleep(time.Second)
					continue
				}
				Log.Errorf("%s: Could not circumvent anti-bot", s.Name())
			}

			return
		} else {
			Log.Debugf("%s: successful after %d retries", s.Name(), retries)
			break
		}
	}

	var apptTypes []WpSsaApptType
	err = json.Unmarshal([]byte(apptTypesJsonStr), &apptTypes)
	if err != nil {
		return
	}

	noncesJsonStr := ""
	noncesJsonStr, body, err = ExtractScrapeUrlWithEndpoints(s.Name(), WpSsaNoncesPattern, endpoint, nil, embedUrl)
	if err != nil {
		return
	}

	var nonces = new(WpSsaNonces)
	err = json.Unmarshal([]byte(noncesJsonStr), nonces)
	if err != nil {
		return
	}

	endpoint.Headers = append(endpoint.Headers, Header{
		Name:  "X-PUBLIC-Nonce",
		Value: nonces.PublicNonce,
	})

	endpoint.Headers = append(endpoint.Headers, Header{
		Name:  "X-Requested-With",
		Value: "XMLHttpRequest",
	})

	endpoint.Headers = append(endpoint.Headers, Header{
		Name:  "X-WP-Nonce",
		Value: nonces.WpNonce,
	})

	//ssa api times are in UTC
	utc, _ := time.LoadLocation("UTC")

	for _, apptType := range apptTypes {
		now := time.Now().In(utc)
		var bookingStart, bookingEnd time.Time
		bookingStart, err = time.ParseInLocation(WpSsaDateFormat, apptType.BookingStart, utc)
		if err != nil {
			return
		}
		bookingEnd, err = time.ParseInLocation(WpSsaDateFormat, apptType.BookingEnd, utc)
		if err != nil {
			return
		}

		if now.Before(bookingStart) || now.After(bookingEnd) {
			Log.Infof("%s: outside booking window: %v - %v", s.Name(), bookingStart, bookingEnd)
			status = StatusNo
			return
		}

		apiUrl := fmt.Sprintf(WpSsaApptsUrl, host, apptType.Id, url.QueryEscape(apptType.AvailabilityStart), url.QueryEscape(apptType.AvailabilityEnd))
		endpoint.Url = apiUrl

		for retries := 0; ; retries++ {
			body, _, err = endpoint.FetchCached(s.Name())
			if err != nil {
				return
			}

			if IncapsulaAntiBotPattern.Match(body) {
				Cache.Clear(endpoint.GenerateCacheKey(s.Name()))

				if retries < 20 {
					time.Sleep(time.Second)
					continue
				}
				Log.Errorf("%s: Could not circumvent anti-bot", s.Name())
			} else {
				Log.Debugf("%s: successful after %d retries", s.Name(), retries)
				break
			}
		}

		resp := new(WpSsaAPIResp)
		err = json.Unmarshal(body, resp)
		if err != nil {
			return
		}

		if len(resp.Error) > 0 {
			err = fmt.Errorf("API returned error: %s", resp.Error)
			return
		}

		if len(resp.Appointments) > 0 {
			status = StatusYes
			return
		}
	}

	status = StatusNo
	return
}
