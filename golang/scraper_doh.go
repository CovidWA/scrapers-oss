package csg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const ScraperTypeDOH = "doh"

const DOHParamKeyDataSourceName = "dsn"
const DOHParamKeyLocationId = "location_id"

const DOHAPIUrl = "https://apim-vaccs-prod.azure-api.net/graphql"
const DOHAPIBody = `{"query":"{searchLocations(searchInput:{rawDataSourceName:\"%s\",paging:{pageNum:1,pageSize:20000}}){locations{locationId,zipcode,latitude,longitude,vaccineAvailability,updatedAt}}}"}`

var DOHAPIHeaders = []Header{
	Header{
		Name:  "Accept",
		Value: "application/json",
	},
	Header{
		Name:  "Accept-Encoding",
		Value: "gzip, br",
	},
	Header{
		Name:  "Content-Type",
		Value: "application/json",
	},
}

type ScraperDOH struct {
	ScraperName    string
	LocationId     string
	DataSourceName string
}

type ScraperDOHFactory struct {
}

func (sf *ScraperDOHFactory) Type() string {
	return ScraperTypeDOH
}

func (sf *ScraperDOHFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	if name == "doh" {
		//scrapers from airtable

		clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^doh_.+$`))
		if err != nil {
			return nil, err
		}
		scrapers := make(map[string]Scraper)

		for _, clinic := range clinics {
			scraper := new(ScraperDOH)
			scraper.ScraperName = clinic.ApiKey
			scraper.LocationId = clinic.ApiKey[4:]

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
		scraper := new(ScraperDOH)
		scraper.ScraperName = name

		return map[string]Scraper{name: scraper}, nil
	}
}

func (s *ScraperDOH) Type() string {
	return ScraperTypeDOH
}

func (s *ScraperDOH) Name() string {
	return s.ScraperName
}

func (s *ScraperDOH) Configure(params map[string]interface{}) error {
	dsn, _ := getStringOptional(params, DOHParamKeyDataSourceName)
	if len(dsn) > 0 {
		s.DataSourceName = dsn
	}

	locationId, _ := getStringOptional(params, DOHParamKeyLocationId)
	if len(locationId) > 0 {
		s.LocationId = locationId
	}

	return nil
}

func (s *ScraperDOH) Scrape() (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	if len(s.DataSourceName) <= 0 {
		err = fmt.Errorf("DataSourceName (dsn) not configured!")
		return
	}

	if len(s.LocationId) <= 0 {
		err = fmt.Errorf("Location Id is missing!")
		return
	}

	endpoint := new(Endpoint)
	endpoint.Method = "POST"
	endpoint.Url = DOHAPIUrl
	endpoint.Body = fmt.Sprintf(DOHAPIBody, s.DataSourceName)
	endpoint.Headers = DOHAPIHeaders

	body, _, err = endpoint.FetchCached(s.Name())
	if err != nil {
		return
	}

	apiResp := new(DOHApiResp)
	err = json.Unmarshal(body, apiResp)
	if err != nil {
		return
	}

	for _, loc := range apiResp.Data.SearchLocations.Locations {
		if strings.EqualFold(loc.LocationId, s.LocationId) { //case insensitive equal
			updatedAt, parseErr := time.Parse(DOHDateTimeFormat, loc.UpdatedAt)
			if parseErr != nil {
				err = parseErr
				return
			}
			if time.Since(updatedAt) < 5*time.Minute {
				if loc.Availability == DOHAvailable {
					status = StatusYes
					break
				}
				if loc.Availability == DOHUnavailable {
					status = StatusNo
					break
				}
			} else if time.Since(updatedAt) < 24*time.Hour {
				status = StatusApiSkip
			} else {
				status = StatusPossible
			}

			break
		}
	}

	if status == StatusUnknown {
		err = fmt.Errorf("Location id %s not found in DOH data!", s.LocationId)
	}

	return
}
