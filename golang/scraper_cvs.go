package csg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
)

const ScraperTypeCvs = "cvs"

const CvsUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/89.0.4389.128 Safari/537.36 Edg/89.0.774.77"
const CvsGetCitiesUrl = "https://www.cvs.com/immunizations/covid-19-vaccine.vaccine-status.WA.json?vaccineinfo"
const CvsGetCitiesReferer = "https://www.cvs.com/immunizations/covid-19-vaccine?icid=cvs-home-hero1-link2-coronavirus-vaccine"

var CvsGetCitiesPattern = regexp.MustCompile(`"data":{"WA":(\[[^\]]+\])}`)
var CvsAntiBotPattern = regexp.MustCompile(`(?i)(<title>Oops!</title>)|(is not available to customers or patients who are located outside)|(<title>Access Denied</title>)`)

var CvsGetStoresUrl = `https://www.cvs.com/Services/ICEAGPV1/immunization/1.0.0/getIMZStores`
var CvsGetStoresBody = `{"requestMetaData":{"appName":"CVS_WEB","lineOfBusiness":"RETAIL","channelName":"WEB","deviceType":"DESKTOP","deviceToken":"7777","apiKey":"a2ff75c6-2da7-4299-929d-d670d827ab4a","source":"ICE_WEB","securityType":"apiKey","responseFormat":"JSON","type":"cn-dep"},"requestPayloadData":{"selectedImmunization":["CVD"],"distanceInMiles":35,"imzData":[{"imzType":"CVD","ndc":["59267100002","59267100003","59676058015","80777027399"],"allocationType":"1"}],"searchCriteria":{"addressLine":"%s"}}}`

const CvsBookingUrl = "https://www.cvs.com/vaccine/intake/store/cvd-schedule"

var CvsFullyBookedPattern = regexp.MustCompile(`(?i)(title>vaccine waiting room)`)

type ScraperCvs struct {
	ScraperName      string
	StoreId          string
	ProxyProvider    ProxyProvider
	StoreRegistry    *CvsStoreRegistry
	LimitedThreshold int
}

type CvsStoreRegistry struct {
	StoreIds  map[string]bool
	CheckOnce *sync.Once
}

func (sr *CvsStoreRegistry) CheckForNewStores(proxyProvider ProxyProvider) {
	sr.CheckOnce.Do(func() {
		stores, body, err := CvsGetStores("CvsStoreRegistry", proxyProvider)
		if err != nil {
			Log.Errorf("CvsStoreRegistry: %v", err)
			dumpOutput("CvsStoreRegistry", "", body)
		} else {
			for storeNumber := range stores {
				if _, exists := sr.StoreIds[storeNumber]; !exists {
					Log.Errorf("CvsStoreRegistry: No airtable entry exists for cvs_%s", storeNumber)
				}
			}
		}
	})
}

type ScraperCvsFactory struct {
}

func (sf *ScraperCvsFactory) Type() string {
	return ScraperTypeCvs
}

func (sf *ScraperCvsFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^cvs_.+$`))
	if err != nil {
		return nil, err
	}
	scrapers := make(map[string]Scraper)

	proxyProvider, err := NewStickyProxyProviderDefaults()
	if err != nil {
		Log.Errorf("%v", err)
	}

	storeRegistry := new(CvsStoreRegistry)
	storeRegistry.CheckOnce = new(sync.Once)
	storeRegistry.StoreIds = make(map[string]bool)

	for _, clinic := range clinics {
		scraper := new(ScraperCvs)
		scraper.ScraperName = clinic.ApiKey
		scraper.StoreId = clinic.ApiKey[4:]
		scraper.ProxyProvider = proxyProvider
		scraper.StoreRegistry = storeRegistry
		scrapers[scraper.Name()] = scraper

		storeRegistry.StoreIds[scraper.StoreId] = true
	}

	return scrapers, nil
}

func (s *ScraperCvs) Type() string {
	return ScraperTypeCvs
}

func (s *ScraperCvs) Name() string {
	return s.ScraperName
}

func (s *ScraperCvs) Configure(params map[string]interface{}) error {
	s.LimitedThreshold = 1

	return nil
}

func (s *ScraperCvs) Scrape() (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	endpoint := new(Endpoint)
	endpoint.Method = "GET"
	endpoint.Url = CvsBookingUrl
	endpoint.AllowedStatusCodes = []int{503}
	body, _, err = endpoint.FetchCached(s.Name())
	if err != nil {
		return
	}

	if CvsFullyBookedPattern.Match(body) {
		status = StatusNo
		return
	}

	s.StoreRegistry.CheckForNewStores(s.ProxyProvider)

	var stores map[string]CountAndTagSet
	stores, body, err = CvsGetStores(s.Name(), s.ProxyProvider)
	if err != nil {
		return
	}

	if countAndTags, exists := stores[s.StoreId]; exists {
		if countAndTags.Count > s.LimitedThreshold {
			status = StatusYes
		} else if countAndTags.Count > 0 {
			status = StatusLimited
		} else {
			status = StatusNo
		}

		tags = tags.Merge(countAndTags.TagSet)
	} else {
		status = StatusNo
	}

	return
}

type CvsCity struct {
	City   string `json:"city"`
	State  string `json:"state"`
	Status string `json:"status"`
}

func CvsGetStores(name string, proxyProvider ProxyProvider) (map[string]CountAndTagSet, []byte, error) {
	endpoint := new(Endpoint)
	endpoint.Method = "GET"
	endpoint.Url = CvsGetCitiesUrl
	endpoint.Headers = []Header{
		Header{
			Name:  "user-agent",
			Value: CvsUserAgent,
		},
		Header{
			Name:  "referer",
			Value: CvsGetCitiesReferer,
		},
		Header{
			Name:  "accept-encoding",
			Value: "gzip, br",
		},
	}

	body, cacheMiss, err := endpoint.FetchCached(name)
	if err != nil {
		return nil, body, err
	}

	stores := make(map[string]CountAndTagSet)
	match := CvsGetCitiesPattern.FindSubmatch(body)
	if len(match) >= 2 {
		cities := make([]CvsCity, 0)
		err = json.Unmarshal(match[1], &cities)
		if err != nil {
			return nil, body, err
		}

		for _, city := range cities {
			if city.State != "WA" {
				Log.Errorf("%s: Unexpected state: %s", name, city.State)
			} else {
				cityStr := fmt.Sprintf("%s, %s", city.City, city.State)
				storesByCity, body, err := CvsGetStoresBySearchString(name, cityStr, proxyProvider)
				if err != nil {
					return nil, body, err
				}
				for store, countAndTags := range storesByCity {
					stores[store] = countAndTags
				}
				if cacheMiss {
					Log.Debugf("%s: looking up stores in %s", name, cityStr)
				}
			}
		}

	} else {
		err = fmt.Errorf("Unexpected response: %s", string(body))
		return nil, body, err
	}

	return stores, body, nil
}

type CvsGetStoresApiResp struct {
	MetaData CvsRespMetaData     `json:"responseMetaData"`
	Payload  CvsGetStoresPayload `json:"responsePayloadData"`
}

type CvsRespMetaData struct {
	StatusCode string `json:"statusCode"`
	StatusDesc string `json:"statusDesc"`
}

type CvsGetStoresPayload struct {
	AvailableDates []string   `json:"availableDates"`
	Locations      []CvsStore `json:"locations"`
}

type CvsStore struct {
	StoreNumber    string              `json:"StoreNumber"`
	State          string              `json:"addressState"`
	Availability   CvsAvailability     `json:"immunizationAvailability"`
	VaccineType    string              `json:"mfrName"`
	AdditionalData []CvsAdditionalData `json:"imzAdditionalData"`
}

type CvsAvailability struct {
	Available   []string `json:"available"`
	Unavailable []string `json:"unavailable"`
}

type CvsAdditionalData struct {
	ImzType        string   `json:"imzType"`
	AvailableDates []string `json:"availableDates"`
}

func CvsGetStoresBySearchString(name string, str string, proxyProvider ProxyProvider) (map[string]CountAndTagSet, []byte, error) {
	endpoint := new(Endpoint)
	endpoint.Method = "POST"
	endpoint.Url = CvsGetStoresUrl
	endpoint.Headers = []Header{
		Header{
			Name:  "accept",
			Value: "application/json",
		},
		Header{
			Name:  "accept-encoding",
			Value: "gzip, deflate, br",
		},
		Header{
			Name:  "accept-language",
			Value: "en-US,en;q=0.9",
		},
		Header{
			Name:  "cache-control",
			Value: "no-cache",
		},
		Header{
			Name:  "content-type",
			Value: "application/json",
		},
		Header{
			Name:  "user-agent",
			Value: CvsUserAgent,
		},
	}
	endpoint.Body = fmt.Sprintf(CvsGetStoresBody, str)
	endpoint.AllowedStatusCodes = []int{456, 403}

	var proxyEndpoint ProxyEndpoint
	var err error
	var body []byte
	var resp *CvsGetStoresApiResp
	var cacheKey = fmt.Sprintf("cvs|%s", str)

	respCached := Cache.GetOrLock(cacheKey)
	if respCached != nil {
		resp = respCached.(*CvsGetStoresApiResp)
	} else {
		defer Cache.Unlock(cacheKey)
	}

	if resp == nil {
		resp = new(CvsGetStoresApiResp)
		for retries := 0; ; retries++ {
			if proxyProvider != nil {
				proxyEndpoint, err = proxyProvider.GetProxy()
				if err != nil {
					return nil, nil, err
				}
				endpoint.HttpClient = proxyEndpoint.GetHttpClient()
			}

			body, _, err = endpoint.Fetch(name)
			if err != nil {
				if proxyEndpoint != nil {
					proxyEndpoint.BlackList()
				}
				return nil, body, err
			}

			err = json.Unmarshal(body, resp)
			if err != nil {
				if proxyEndpoint != nil {
					proxyEndpoint.BlackList()
					if CvsAntiBotPattern.Match(body) && retries < 5 {
						Log.Debugf("%s: Anti bot block detected, retrying %d/5", name, retries+1)
						continue
					}
				}
				return nil, body, err
			}

			Cache.Put(cacheKey, resp, 60, 0)
			break
		}
	}

	stores := make(map[string]CountAndTagSet)

	if resp.MetaData.StatusCode != "0000" {
		if resp.MetaData.StatusCode == "1010" {
			//no stores found
			return stores, body, nil
		}
		return nil, body, fmt.Errorf("Unexpected status code: %s, %s", resp.MetaData.StatusCode, resp.MetaData.StatusDesc)
	}

	for _, store := range resp.Payload.Locations {
		if store.State != "WA" {
			continue
		}

		var storeData CountAndTagSet
		storeData.Count = 0
		storeData.TagSet = storeData.TagSet.ParseAndAddVaccineType(store.VaccineType)
		for _, avail := range store.Availability.Available {
			if avail == "CVD" {
				storeData.Count = 1
			}
		}

		for _, additionalData := range store.AdditionalData {
			if additionalData.ImzType == "CVD" {
				storeData.Count = len(additionalData.AvailableDates)
			}
		}

		stores[store.StoreNumber] = storeData
	}

	return stores, body, nil
}
