package csg

import (
	"encoding/json"
	"regexp"
)

const ScraperTypeWalgreensAPI = "walgreens_api"

type ScraperWalgreensAPI struct {
	ScraperName    string
	ScrapeEndpoint *Endpoint
	StoreNumber    string
}

type ScraperWalgreensAPIFactory struct {
}

func (sf *ScraperWalgreensAPIFactory) Type() string {
	return ScraperTypeWalgreensAPI
}

func (sf *ScraperWalgreensAPIFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^walgreens_[0-9]+$`))
	if err != nil {
		return nil, err
	}
	scrapers := make(map[string]Scraper)

	for _, clinic := range clinics {
		scraper := new(ScraperWalgreensAPI)
		scraper.ScraperName = clinic.ApiKey
		scraper.StoreNumber = clinic.ApiKey[10:]

		scrapers[scraper.Name()] = scraper
	}

	return scrapers, nil
}

func (s *ScraperWalgreensAPI) Type() string {
	return ScraperTypeWalgreensAPI
}

func (s *ScraperWalgreensAPI) Name() string {
	return s.ScraperName
}

func (s *ScraperWalgreensAPI) Configure(params map[string]interface{}) error {
	var err error

	s.ScrapeEndpoint, err = getEndpointRequired(params, ParamKeyEndpoint)
	if err != nil {
		return err
	}

	return nil
}

type VaccineSpotterAPIResp struct {
	StoreNumber string `json:"brand_id"`
	Available   bool   `json:"appointments_available"`
}

func (s *ScraperWalgreensAPI) Scrape() (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	body, _, err = s.ScrapeEndpoint.FetchCached(s.Name())

	if err != nil {
		return
	}

	jsonData := make([]VaccineSpotterAPIResp, 0)
	err = json.Unmarshal(body, &jsonData)
	if err != nil {
		return
	}

	for _, store := range jsonData {
		if store.StoreNumber == s.StoreNumber {
			Log.Debugf("Found walgreens store# %s", store.StoreNumber)
			if store.Available {
				status = StatusYes
			} else {
				status = StatusNo
			}
			break
		}
	}

	return
}
