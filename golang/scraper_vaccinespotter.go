package csg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const ScraperTypeVaccineSpotter = "vaccinespotter"

const VaccineSpotterParamKeyProviderName = "provider_name"
const VaccineSpotterParamKeyLocationId = "location_id"

const VaccineSpotterTimePattern = "2006-01-02T15:04:05-07:00"
const VaccineSpotterMinDataAge = 300 //seconds

const VaccineSpotterCacheTTL = 60

type ScraperVaccineSpotter struct {
	ScraperName      string
	ProviderName     string
	LocationId       string
	Endpoint         *Endpoint
	LimitedThreshold int
}

type ScraperVaccineSpotterFactory struct {
}

func (sf *ScraperVaccineSpotterFactory) Type() string {
	return ScraperTypeVaccineSpotter
}

func (sf *ScraperVaccineSpotterFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	if name == "vaccinespotter" {
		//scrapers from airtable

		clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^vs(_[^_]+){2,}$`))
		if err != nil {
			return nil, err
		}
		scrapers := make(map[string]Scraper)

		for _, clinic := range clinics {
			scraper := new(ScraperVaccineSpotter)
			scraper.ScraperName = clinic.ApiKey

			keyParts := strings.Split(clinic.ApiKey, "_")
			scraper.ProviderName = strings.Join(keyParts[1:len(keyParts)-1], "_")
			scraper.LocationId = keyParts[len(keyParts)-1]

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
		scraper := new(ScraperVaccineSpotter)
		scraper.ScraperName = name

		return map[string]Scraper{name: scraper}, nil
	}
}

func (s *ScraperVaccineSpotter) Type() string {
	return ScraperTypeVaccineSpotter
}

func (s *ScraperVaccineSpotter) Name() string {
	return s.ScraperName
}

func (s *ScraperVaccineSpotter) Configure(params map[string]interface{}) error {
	s.LimitedThreshold = config.LimitedThreshold

	if providerName, exists := getStringOptional(params, VaccineSpotterParamKeyProviderName); exists {
		s.ProviderName = providerName
	}

	if locationId, exists := getStringOptional(params, VaccineSpotterParamKeyLocationId); exists {
		s.LocationId = locationId
	}

	if endpoint := getEndpointOptional(params, ParamKeyEndpoint); endpoint != nil {
		s.Endpoint = endpoint
	}

	return nil
}

type VSAPIResp struct {
	Features []VSFeature `json:"features"`
}

type VSFeature struct {
	Properties VSFeatureProps `json:"properties"`
}

type VSFeatureProps struct {
	Provider     string          `json:"provider"`
	LocationId   string          `json:"provider_location_id"`
	Appointments []VSAppointment `json:"appointments"`
	VaccineTypes map[string]bool `json:"appointment_vaccine_types"`
	LastFetched  string          `json:"appointments_last_fetched"`
}

type VSAppointment struct {
	Time         string   `json:"time"`
	Type         string   `json:"type"`
	VaccineTypes []string `json:"vaccine_types"`
}

func (s *ScraperVaccineSpotter) Scrape() (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown
	if s.Endpoint == nil {
		err = fmt.Errorf("API endpoint not configured")
		return
	}

	if len(s.ProviderName) < 1 {
		Log.Errorf("%s: Provider Name not found in key or configured", s.Name())
		status = StatusPossible
		return
	}

	if len(s.LocationId) < 1 {
		Log.Errorf("%s: Location Id not found in key or configured", s.Name())
		status = StatusPossible
		return
	}

	const cacheKey = "VaccineSpotterParsedJSON"
	var apiResp *VSAPIResp

	if apiResp, _ = Cache.GetOrLock(cacheKey).(*VSAPIResp); apiResp == nil {
		defer Cache.Unlock(cacheKey)

		body, _, err = s.Endpoint.Fetch(s.Name())
		if err != nil {
			return
		}

		apiResp = new(VSAPIResp)

		err = json.Unmarshal(body, apiResp)
		if err != nil {
			return
		}
		Cache.Put(cacheKey, apiResp, VaccineSpotterCacheTTL, -1)
	}

	for _, location := range apiResp.Features {
		if location.Properties.Provider == s.ProviderName && location.Properties.LocationId == s.LocationId {
			lastFetched, parseErr := time.Parse(VaccineSpotterTimePattern, location.Properties.LastFetched)
			if parseErr != nil {
				Log.Errorf("%s: %v", s.Name(), parseErr)
			} else {
				dataAge := time.Since(lastFetched).Seconds()
				if dataAge > VaccineSpotterMinDataAge {
					Log.Warnf("%s: maximum data age exceeed: %f > %d", s.Name(), dataAge, VaccineSpotterMinDataAge)
					status = StatusApiSkip
					return
				} else {
					appts := len(location.Properties.Appointments)
					Log.Debugf("%s: number of appts: %d, age: %fs", s.Name(), appts, dataAge)
					if appts > s.LimitedThreshold {
						status = StatusYes
					} else if appts > 0 {
						status = StatusLimited
					} else {
						status = StatusNo
					}

					for _, appt := range location.Properties.Appointments {
						for _, vaccineType := range appt.VaccineTypes {
							tags = tags.ParseAndAddVaccineType(vaccineType)
						}
					}

					return
				}
			}
		}
	}

	err = fmt.Errorf("could not get data from vaccinespotter api response")
	status = StatusUnknown

	return
}
