package csg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
)

const ScraperTypeWalmart = "walmart"

const WalmartParamKeyZipcode = "zipcode"
const WalmartCacheTTL = 300

const WalmartAPIUrl = "https://www.walmart.com/pharmacy/v2/clinical-services/stores?search=%s&imzType=COVID&proximity=50"
const WalmartUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/89.0.4389.128 Safari/537.36 Edg/89.0.774.77"

type ScraperWalmart struct {
	ScraperName      string
	StoreNumber      string
	Zipcode          string
	LimitedThreshold int
	ProxyProvider    ProxyProvider
	mutex            *sync.Mutex
}

type ScraperWalmartFactory struct {
}

func (sf *ScraperWalmartFactory) Type() string {
	return ScraperTypeWalmart
}

func (sf *ScraperWalmartFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	if name == "walmart" {
		//scrapers from airtable

		clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^walmart_.+$`))
		if err != nil {
			return nil, err
		}
		scrapers := make(map[string]Scraper)
		mutex := new(sync.Mutex)

		proxyProvider, err := NewStickyProxyProviderDefaults()
		if err != nil {
			Log.Errorf("%v", err)
		}

		for _, clinic := range clinics {
			scraper := new(ScraperWalmart)
			scraper.ScraperName = clinic.ApiKey
			scraper.StoreNumber = clinic.ApiKey[8:]
			scraper.ProxyProvider = proxyProvider
			scraper.mutex = mutex

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
		scraper := new(ScraperWalmart)
		scraper.ScraperName = name

		return map[string]Scraper{name: scraper}, nil
	}
}

func (s *ScraperWalmart) Type() string {
	return ScraperTypeWalmart
}

func (s *ScraperWalmart) Name() string {
	return s.ScraperName
}

func (s *ScraperWalmart) Configure(params map[string]interface{}) error {
	s.LimitedThreshold = config.LimitedThreshold

	zipcode, exists := getStringOptional(params, WalmartParamKeyZipcode)
	if exists {
		s.Zipcode = zipcode
	}

	return nil
}

type WalmartAPIResp struct {
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Data    []WalmartAPIData `json:"data"`
}

type WalmartAPIData struct {
	StoreNumber   string             `json:"number"`
	SlotsAvail    string             `json:"slots"`
	InvAvail      string             `json:"inventory"`
	InventoryInfo []WalmartInventory `json:"inventoryInfo"`
}

type WalmartInventory struct {
	ProductId int    `json:"productMdsFamId"`
	Quantity  int    `json:"quantity"`
	Name      string `json:"shortName"`
}

func (s *ScraperWalmart) Scrape() (status Status, tags TagSet, body []byte, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	status = StatusUnknown

	cacheKey := fmt.Sprintf("walmart-%s", s.StoreNumber)
	statusCached := Cache.GetOrLock(cacheKey)
	if statusCached != nil {
		statusAndTags := statusCached.(StatusAndTagSet)
		status = statusAndTags.Status
		tags = statusAndTags.TagSet
		return
	} else {
		Cache.Unlock(cacheKey)
	}

	if len(s.Zipcode) < 1 {
		status = StatusPossible
		Log.Errorf("%s: zipcode not configured", s.Name())
		return
	}

	if s.ProxyProvider == nil {
		status = StatusPossible
		Log.Errorf("%s: proxy not configured", s.Name())
		return
	}

	var proxyEndpoint ProxyEndpoint
	proxyEndpoint, err = s.ProxyProvider.GetProxy()
	if err != nil {
		return
	}

	endpoint := new(Endpoint)
	endpoint.Method = "GET"
	endpoint.Url = fmt.Sprintf(WalmartAPIUrl, s.Zipcode)
	endpoint.HttpClient = proxyEndpoint.GetHttpClient()
	endpoint.Headers = []Header{
		Header{
			Name:  "User-Agent",
			Value: WalmartUserAgent,
		},
		Header{
			Name:  "Accept",
			Value: "application/json",
		},
		Header{
			Name:  "Accept-Encoding",
			Value: "gzip, br",
		},
		Header{
			Name:  "Cache-Control",
			Value: "no-cache",
		},
	}
	body, _, err = endpoint.FetchCached(s.Name())
	if err != nil {
		proxyEndpoint.BlackList()
		return
	}

	apiResp := new(WalmartAPIResp)
	err = json.Unmarshal(body, apiResp)
	if err != nil {
		return
	}

	if apiResp.Status != "1" {
		err = fmt.Errorf("Unknown status: %s: %s", apiResp.Status, apiResp.Message)
		return
	}

	for _, loc := range apiResp.Data {
		storeCacheKey := fmt.Sprintf("walmart-%s", loc.StoreNumber)
		var storeStatusAndTags StatusAndTagSet
		storeStatusAndTags.Status = StatusNo
		if loc.SlotsAvail == "AVAILABLE" && loc.InvAvail == "AVAILABLE" {
			storeStatusAndTags.Status = StatusYes
			totalInventory := 0
			for _, invInfo := range loc.InventoryInfo {
				totalInventory += invInfo.Quantity
				if invInfo.Quantity > 0 {
					storeStatusAndTags.TagSet = storeStatusAndTags.TagSet.ParseAndAddVaccineType(invInfo.Name)
				}
			}
			if totalInventory <= s.LimitedThreshold {
				storeStatusAndTags.Status = StatusLimited
			}
		}

		Cache.Put(storeCacheKey, storeStatusAndTags, WalmartCacheTTL, -1)

		if loc.StoreNumber == s.StoreNumber {
			status = storeStatusAndTags.Status
			tags = storeStatusAndTags.TagSet
		}
	}

	if status == StatusUnknown {
		err = fmt.Errorf("Store number %s was not found in zip code %s", s.StoreNumber, s.Zipcode)
	}

	return
}
