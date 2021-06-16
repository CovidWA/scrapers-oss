package csg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const ScraperTypeMsOutlook = "msoutlook"

const MsOutlookParamKeyNamePattern = "namepattern"

const MsOutlookDefaultTimezone = "America/Los_Angeles"

var MsOutlookCalFormUrlPattern = regexp.MustCompile(`(?i)https?://outlook.office365.com/owa/calendar/[^/]+/bookings/`)
var MsOutlookDataPayloadPattern = regexp.MustCompile(`(?i)var PageDataPayload = ({[^;]+);`)

const MsOutlookServiceUrl = "https://%s%sGetStaffBookability"
const MsOutlookServiceReq = `{"StaffList":["%s"],"Start":"%sT00:00:00","End":"%sT00:00:00","TimeZone":"%s","ServiceId":"%s"}`
const MsOutlookDateFormat = "2006-01-02"
const MsOutlookDateTimeFormat = "2006-01-02T15:04:05"

type ScraperMsOutlook struct {
	ScraperName      string
	Url              string
	AlternateUrl     string
	Timezone         *time.Location
	NamePattern      *regexp.Regexp
	LimitedThreshold int
	Configured       bool
}

type ScraperMsOutlookFactory struct {
}

func (sf *ScraperMsOutlookFactory) Type() string {
	return ScraperTypeMsOutlook
}

func (sf *ScraperMsOutlookFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	if name == "msoutlook" {
		//scrapers from airtable

		clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^msoutlook_.+$`))
		if err != nil {
			return nil, err
		}
		scrapers := make(map[string]Scraper)

		for _, clinic := range clinics {
			scraper := new(ScraperMsOutlook)
			scraper.ScraperName = clinic.ApiKey
			scraper.Url = clinic.Url
			scraper.AlternateUrl = clinic.AlternateUrl
			//hardcoding timezone because site may have set the wrong tz
			scraper.Timezone, err = time.LoadLocation(MsOutlookDefaultTimezone)
			if err != nil {
				return nil, err
			}

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
		var err error

		scraper := new(ScraperMsOutlook)
		scraper.ScraperName = name
		//hardcoding timezone because site may have set the wrong tz
		scraper.Timezone, err = time.LoadLocation(MsOutlookDefaultTimezone)
		if err != nil {
			return nil, err
		}

		return map[string]Scraper{name: scraper}, nil
	}
}

func (s *ScraperMsOutlook) Type() string {
	return ScraperTypeMsOutlook
}

func (s *ScraperMsOutlook) Name() string {
	return s.ScraperName
}

func (s *ScraperMsOutlook) Configure(params map[string]interface{}) error {
	if s.Configured {
		//only configure once, either from airtable or .yaml
		return nil
	}

	s.Configured = true
	s.LimitedThreshold = config.LimitedThreshold
	s.NamePattern = getPatternOptional(params, MsOutlookParamKeyNamePattern)

	if s.NamePattern != nil {
		Log.Debugf("%s: Configured name pattern: %v", s.Name(), s.NamePattern)
	}

	url, exists := getStringOptional(params, "url")
	if exists && len(url) > 0 {
		s.Url = url
	}

	return nil
}

type MsOutlookFormData struct {
	AriaAppKey            string                   `json:"AriaAppKey"`
	BookingMailboxAddress string                   `json:"BookingMailboxAddress"`
	Services              []MsOutlookService       `json:"Services"`
	Settings              MsOutlookServiceSettings `json:"Settings"`
}

type MsOutlookService struct {
	Id               string                    `json:"Id"`
	Name             string                    `json:"Name"`
	StaffList        []string                  `json:"StaffList"`
	Duration         int                       `json:"DurationMinutes"`
	SchedulingPolicy MsOutlookSchedulingPolicy `json:"SchedulingPolicy"`
}

type MsOutlookSchedulingPolicy struct {
	CapTimeInDays int `json:"CapTimeInDays"`
	LeadTime      int `json:"LeadTimeForBookingsInMinutes"`
	Interval      int `json:"TimeSlotIntervalInMinutes"`
}

type MsOutlookServiceSettings struct {
	ServiceRequestUrl string `json:"ServiceRequestUrl"`
}

type MsOutlookBookAPIResp struct {
	Staff []MsOutlookStaffBookability `json:"StaffBookabilities"`
	Now   string                      `json:"DateTimeNowInTimeZone"`
}

type MsOutlookTimeBlock struct {
	Start string `json:"Start"`
	End   string `json:"End"`
}

type MsOutlookStaffBookability struct {
	Id         string               `json:"Id"`
	TimeBlocks []MsOutlookTimeBlock `json:"BookableTimeBlocks"`
}

func (s *ScraperMsOutlook) Scrape() (status Status, tags TagSet, body []byte, err error) {
	return s.ScrapeUrls(s.AlternateUrl, s.Url)
}

func (s *ScraperMsOutlook) ScrapeUrls(urls ...string) (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	var formUrl string
	formUrl, body, err = ExtractScrapeUrl(s.Name(), MsOutlookCalFormUrlPattern, urls...)
	if err != nil {
		return
	}

	hostSubmatch := HostPattern.FindStringSubmatch(formUrl)
	if len(hostSubmatch) < 2 {
		err = fmt.Errorf("Could not parse host from url: %s", formUrl)
		return
	}
	host := hostSubmatch[1]

	var dataPayloadJsonStr string
	dataPayloadJsonStr, body, err = ExtractScrapeUrl(s.Name(), MsOutlookDataPayloadPattern, formUrl)
	if err != nil {
		return
	}

	formData := new(MsOutlookFormData)
	err = json.Unmarshal([]byte(dataPayloadJsonStr), formData)
	if err != nil {
		return
	}
	if len(formData.Settings.ServiceRequestUrl) == 0 {
		err = fmt.Errorf("Missing service url in form JSON payload")
		return
	}

	apptCount := 0

	for _, svc := range formData.Services {
		Log.Debugf("%s: checking service '%s'", s.Name(), svc.Name)

		if s.NamePattern != nil && !s.NamePattern.MatchString(svc.Name) {
			Log.Debugf("%s: skipping '%s' because it didn't match name pattern", s.Name(), svc.Name)
			continue
		}

		if len(svc.StaffList) == 0 {
			Log.Warnf("%s: skipping '%s' because it didn't have any staff", s.Name(), svc.Name)
			continue
		}

		endpoint := new(Endpoint)
		endpoint.Method = "POST"
		endpoint.Url = fmt.Sprintf(MsOutlookServiceUrl, host, formData.Settings.ServiceRequestUrl)
		endpoint.Headers = []Header{
			Header{
				Name:  "Accept-Encoding",
				Value: "gzip, br",
			},
			Header{
				Name:  "Content-Type",
				Value: "application/json",
			},
		}

		now := time.Now().In(s.Timezone)
		startDateStr := now.Format(MsOutlookDateFormat)
		endDateStr := now.AddDate(0, 0, svc.SchedulingPolicy.CapTimeInDays+1).Format(MsOutlookDateFormat)
		endpoint.Body = fmt.Sprintf(MsOutlookServiceReq, strings.Join(svc.StaffList, `","`), startDateStr, endDateStr, s.Timezone.String(), svc.Id)
		Log.Debugf("%s: request: %s", s.Name(), endpoint.Body)
		body, _, err = endpoint.FetchCached(s.Name())
		if err != nil {
			Log.Errorf("%s: %v", s.Name(), err)
			continue
		}

		apiResp := new(MsOutlookBookAPIResp)
		err = json.Unmarshal(body, apiResp)
		if err != nil {
			return
		}
		now, err = time.ParseInLocation(MsOutlookDateTimeFormat, apiResp.Now, s.Timezone)
		if err != nil {
			return
		}

		windowStart := now.Add(time.Duration(svc.SchedulingPolicy.LeadTime) * time.Minute)
		windowEnd, _ := time.ParseInLocation(MsOutlookDateFormat, endDateStr, s.Timezone)
		scheduleInterval := time.Duration(svc.SchedulingPolicy.Interval) * time.Minute
		Log.Debugf("Window: %v - %v, %v", windowStart, windowEnd, scheduleInterval)

		for _, staff := range apiResp.Staff {
			for _, timeblock := range staff.TimeBlocks {
				if len(timeblock.Start) < len(MsOutlookDateTimeFormat) {
					Log.Warnf("%s: Missing or malformed datetime: %s", s.Name(), timeblock.Start)
					continue
				}
				if len(timeblock.End) < len(MsOutlookDateTimeFormat) {
					Log.Warnf("%s: Missing or malformed datetime: %s", s.Name(), timeblock.End)
					continue
				}

				var start, end time.Time
				start, err = time.ParseInLocation(MsOutlookDateTimeFormat, timeblock.Start[:len(MsOutlookDateTimeFormat)], s.Timezone)
				if err != nil {
					Log.Warnf("%s: %v", s.Name(), err)
					continue
				}
				end, err = time.ParseInLocation(MsOutlookDateTimeFormat, timeblock.End[:len(MsOutlookDateTimeFormat)], s.Timezone)
				if err != nil {
					Log.Warnf("%s: %v", s.Name(), err)
					continue
				}

				for apptTime := start; apptTime.Before(end); apptTime = apptTime.Add(scheduleInterval) {
					if apptTime.Before(windowStart) {
						continue
					}
					if apptTime.After(windowEnd) {
						continue
					}

					apptEnd := apptTime.Add(time.Duration(svc.Duration) * time.Minute)
					if apptEnd.After(end) {
						continue
					}

					Log.Debugf("%s: Available: %v", s.Name(), apptTime)
					apptCount++
					err = nil
					tags = tags.ParseAndAddVaccineType(svc.Name)
				}
			}
		}
	}

	if apptCount > s.LimitedThreshold {
		status = StatusYes
	} else if apptCount > 0 {
		status = StatusLimited
	} else {
		status = StatusNo
	}

	err = nil
	return
}
