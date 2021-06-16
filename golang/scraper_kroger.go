package csg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

const ScraperTypeKroger = "kroger"
const KrogerNoAppointments = `Find Your Phase</a> tool.</li><li>Appointment scheduling coming soon</li></ul>`
const KrogerParamKeyZipcode = "zipcode"
const KrogerDefaultDist = 500
const KrogerErrorLimit = 5
const KrogerErrorCooldown = 60

const KrogerUseProxy = true
const KrogerEndpointCacheTTL = 300
const KrogerEndpointReuseMax = 15
const KrogerSensorDataMaxUses = 4

const KrogerSensorDataFilePath = "./kroger_sensor_data.yaml"

const KrogerLookaheadDays = 10

const KrogerCacheKey = "kroger|%s"
const KrogerCacheTTL = 300

var krogerFetcherSingletonLock = new(sync.Mutex)
var krogerFetcherSingletonInstance *KrogerFetcher = nil

type KrogerFetcher struct {
	SensorData          *AkamaiSensorData
	ProxyProvider       ProxyProvider
	errorCount          int
	errorCooldownExpiry int64
	epCacheName         string
	mutex               *sync.Mutex
}

type KrogerFetcherCacheData struct {
	Endpoint       *Endpoint
	ProxyEndpoint  ProxyEndpoint
	SensorDataUses int
}

func NewKrogerFetcher() *KrogerFetcher {
	krogerFetcherSingletonLock.Lock()
	defer krogerFetcherSingletonLock.Unlock()

	if krogerFetcherSingletonInstance != nil {
		return krogerFetcherSingletonInstance
	}

	krogerFetcherSingletonInstance = new(KrogerFetcher)
	fetcher := krogerFetcherSingletonInstance
	fetcher.SensorData = ParseAkamaiSensorData(KrogerSensorDataFilePath)
	fetcher.errorCount = 0
	fetcher.errorCooldownExpiry = 0
	fetcher.epCacheName = "KrogerFetcher"
	fetcher.mutex = new(sync.Mutex)

	if KrogerUseProxy {
		var err error
		fetcher.ProxyProvider, err = NewProxyRackAuthHttpProxyProviderDefaults()
		if err != nil {
			Log.Errorf("KrogerFetcher: %v", err)
		}
	}

	return fetcher
}

func (cg *KrogerFetcher) reportProxyError() {
	if cachedEpData, ok := Cache.Clear(cg.epCacheName).(*KrogerFetcherCacheData); ok {
		if KrogerUseProxy && cachedEpData.ProxyEndpoint != nil {
			cachedEpData.ProxyEndpoint.BlackList()
		}
	}

	cg.errorCount++

	if cg.errorCount > KrogerErrorLimit {
		cg.errorCooldownExpiry = time.Now().Unix() + KrogerErrorCooldown
		cg.errorCount = 0
	}
}

func (cg *KrogerFetcher) resetErrors() {
	cg.errorCount = 0
	cg.errorCooldownExpiry = 0
}

func (cg *KrogerFetcher) inErrorCooldown() bool {
	return cg.errorCooldownExpiry > time.Now().Unix()
}

func (cg *KrogerFetcher) Fetch(group string, dist int) ([]byte, error) {
	cg.mutex.Lock()
	defer cg.mutex.Unlock()

	name := fmt.Sprintf("KrogerFetcher (%s/%d)", group, dist)

	sensorData, err := cg.SensorData.GetSensorData()
	if err != nil {
		return nil, err
	}

	userAgent := Header{
		Name:  "User-Agent",
		Value: sensorData.UserAgent,
	}

	accept := Header{
		Name:  "Accept",
		Value: "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
	}

	acceptEncoding := Header{
		Name:  "Accept-Encoding",
		Value: "gzip, deflate, br",
	}

	var cachedData *KrogerFetcherCacheData
	var ok bool
	if cachedData, ok = Cache.GetOrLock(cg.epCacheName).(*KrogerFetcherCacheData); ok {
		Log.Debugf("Endpoint (cached) uses left: %d", Cache.UsesLeft(cg.epCacheName))
	} else {
		defer Cache.Unlock(cg.epCacheName)
		cachedData = new(KrogerFetcherCacheData)
		cachedData.SensorDataUses = 0

		Log.Debugf("Endpoint (new) uses left: %d", KrogerEndpointReuseMax)

		// CALL 1: LANDING PAGE

		cachedData.Endpoint = new(Endpoint)

		if KrogerUseProxy && cg.ProxyProvider != nil {
			cachedData.ProxyEndpoint, err = cg.ProxyProvider.GetProxy()
			if err != nil {
				return nil, err
			}
			cachedData.Endpoint.HttpClient = cachedData.ProxyEndpoint.GetHttpClient()
		}
		cachedData.Endpoint.Url = "https://www.kroger.com/rx/covid-eligibility"
		cachedData.Endpoint.Method = "GET"
		cachedData.Endpoint.Headers = []Header{userAgent, accept, acceptEncoding}
		cachedData.Endpoint.Cookies = make(map[string]string)
		cachedData.Endpoint.CookieWhitelist = []string{"*"}
		cachedData.Endpoint.AllowedStatusCodes = []int{}

		frontPage, _, err := cachedData.Endpoint.Fetch(name)
		if err != nil {
			return nil, err
		}

		if strings.Contains(string(frontPage), KrogerNoAppointments) {
			return nil, fmt.Errorf("KrogerFetcher: Landing page indicates appointments are disabled")
		}

		time.Sleep(100 * time.Millisecond)

		Cache.Put(cg.epCacheName, cachedData, KrogerEndpointCacheTTL, KrogerEndpointReuseMax)
	}

	reqType := Header{
		Name:  "X-Sec-Clge-Req-Type",
		Value: "ajax",
	}

	cookie := Header{
		Name:  "Cookie",
		Value: "",
	}

	// CALL 2: POST SENSOR DATA (if needed)
	if cachedData.SensorDataUses == 0 || cachedData.SensorDataUses >= KrogerSensorDataMaxUses {
		cachedData.SensorDataUses = 0

		accept = Header{
			Name:  "Accept",
			Value: "*/*",
		}

		contentType := Header{
			Name:  "ContentType",
			Value: "text/plain;charset=UTF-8",
		}

		contentLength := Header{
			Name:  "ContentLength",
			Value: fmt.Sprintf("%d", len(sensorData.SensorData)),
		}

		cachedData.Endpoint.Url = fmt.Sprintf("https://www.kroger.com/%s", sensorData.SensorDest)
		cachedData.Endpoint.Method = "POST"
		cachedData.Endpoint.Body = sensorData.SensorData
		cachedData.Endpoint.Headers = []Header{userAgent, accept, contentType, contentLength, reqType, cookie}
		cachedData.Endpoint.CookieWhitelist = []string{"*"}
		cachedData.Endpoint.AllowedStatusCodes = []int{201}
		if cachedData.ProxyEndpoint != nil {
			cachedData.Endpoint.HttpClient = cachedData.ProxyEndpoint.GetHttpClient()
		}

		success, _, err := cachedData.Endpoint.Fetch(name)
		sensorData.MarkUsed()

		if err != nil {
			return nil, err
		}

		if !strings.Contains(string(success), AkamaiSensorSuccess) {
			return nil, fmt.Errorf("Unexpected message: %s, was expecting %s", string(success), AkamaiSensorSuccess)
		}
	}
	cachedData.SensorDataUses++

	// CALL 3: INVOKE API

	accept = Header{
		Name:  "Accept",
		Value: "application/json, text/plain, */*",
	}

	scrapeUrl := "https://www.kroger.com/rx/api/anonymous/scheduler/slots/locationsearch/%s/##CURRENT_DATE##/##{2006-01-02;0;%d}##/%d?appointmentReason=131,134,137,122,125,129&benefitCode=null"
	scrapeUrl = fmt.Sprintf(scrapeUrl, group, KrogerLookaheadDays, dist)
	scrapeUrl = replaceMagic(scrapeUrl)
	cachedData.Endpoint.Method = "GET"
	cachedData.Endpoint.Url = scrapeUrl
	cachedData.Endpoint.Headers = []Header{userAgent, accept, acceptEncoding, reqType, cookie}
	cachedData.Endpoint.CookieWhitelist = []string{"*"}
	if cachedData.ProxyEndpoint != nil {
		cachedData.Endpoint.HttpClient = cachedData.ProxyEndpoint.GetHttpClient()
	}

	time.Sleep(100 * time.Millisecond)

	body, _, err := cachedData.Endpoint.Fetch(name)

	return body, err
}

type ScraperKroger struct {
	ScraperName      string
	Fetcher          *KrogerFetcher
	Zipcode          string
	LocationNo       string
	Configured       bool
	KnownLocations   map[string]bool
	LimitedThreshold int
	mutex            *sync.Mutex
}

type ScraperKrogerFactory struct {
}

func (sf *ScraperKrogerFactory) Type() string {
	return ScraperTypeKroger
}

func (sf *ScraperKrogerFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^kroger_.+$`))
	if err != nil {
		return nil, err
	}
	scrapers := make(map[string]Scraper)

	mutex := new(sync.Mutex)
	knownLocations := make(map[string]bool)

	for _, clinic := range clinics {
		scraper := new(ScraperKroger)
		scraper.ScraperName = clinic.ApiKey
		scraper.LocationNo = clinic.ApiKey[7:]
		knownLocations[scraper.LocationNo] = true
		scraper.Fetcher = NewKrogerFetcher()
		scraper.KnownLocations = knownLocations
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
}

func (s *ScraperKroger) Type() string {
	return ScraperTypeKroger
}

func (s *ScraperKroger) Name() string {
	return s.ScraperName
}

func (s *ScraperKroger) Configure(params map[string]interface{}) error {
	if s.Configured {
		return nil
	}

	s.Configured = true

	s.LimitedThreshold = config.LimitedThreshold

	var err error
	s.Zipcode, err = getStringRequired(params, KrogerParamKeyZipcode)
	if err != nil {
		Log.Errorf("%s: %v", s.Name(), err)
	}
	return nil
}

type KrogerAPIResp struct {
	StoreId    string           `json:"loc_no"`
	Dates      []KrogerDate     `json:"dates"`
	LocDetails KrogerLocDetails `json:"facilityDetails"`
}

type KrogerDate struct {
	Date  string       `json:"date"`
	Slots []KrogerSlot `json:"slots"`
}

type KrogerSlot struct {
	StartTime string  `json:"start_time"`
	Id        float64 `json:"ar_id"`
	Reason    string  `json:"ar_reason"`
}

type KrogerLocDetails struct {
	Name       string     `json:"vanityName"`
	Address    KrogerAddr `json:"address"`
	LocationId string     `json:"facilityId"`
}

type KrogerAddr struct {
	Address1 string `json:"Address1"`
	Address2 string `json:"Address2"`
	City     string `json:"city"`
	State    string `json:"state"`
	Zipcode  string `json:"zipCode"`
}

func (s *ScraperKroger) Scrape() (status Status, tags TagSet, body []byte, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	status = StatusUnknown

	cacheKey := fmt.Sprintf(KrogerCacheKey, s.LocationNo)
	cachedStatusAndTagSet := Cache.GetOrLock(cacheKey)
	if cachedStatusAndTagSet != nil {
		statusAndTagSet := cachedStatusAndTagSet.(StatusAndTagSet)
		status = statusAndTagSet.Status
		tags = statusAndTagSet.TagSet
		return
	} else {
		//don't have to defer since we're protected by the mutex
		Cache.Unlock(cacheKey)
	}

	if len(s.Zipcode) < 5 {
		Log.Errorf("%s: Missing or invalid zip code", s.Name())
		status = StatusPossible
		return
	}

	if s.Fetcher.inErrorCooldown() {
		err = fmt.Errorf("Fetcher reported too many errors, in cooldown")
		return
	}

	body, err = s.Fetcher.Fetch(s.Zipcode, KrogerDefaultDist)
	if err != nil {
		s.Fetcher.reportProxyError()
		return
	}

	s.Fetcher.resetErrors()

	resp := make([]KrogerAPIResp, 0)
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return
	}

	storeFound := false
	for _, store := range resp {
		if _, exists := s.KnownLocations[store.StoreId]; !exists {
			if store.LocDetails.Address.State == "WA" {
				s.KnownLocations[store.StoreId] = true
				Log.Errorf("%s: Kroger WA location not found in airtable: %v", s.Name(), store.LocDetails)
			}
		}

		var storeStatusAndTags StatusAndTagSet
		storeStatusAndTags.Status = StatusNo

		storeCacheKey := fmt.Sprintf(KrogerCacheKey, store.StoreId)

		storeAppts := 0

		for _, date := range store.Dates {
			storeAppts += len(date.Slots)
			for _, slot := range date.Slots {
				storeStatusAndTags.TagSet = storeStatusAndTags.TagSet.ParseAndAddVaccineType(slot.Reason)
			}
		}

		if storeAppts > 0 {
			storeStatusAndTags.Status = StatusYes
			if storeAppts <= s.LimitedThreshold {
				storeStatusAndTags.Status = StatusLimited
			}
		}

		if s.LocationNo == store.StoreId {
			storeFound = true
			status = storeStatusAndTags.Status
			tags = storeStatusAndTags.TagSet
			Cache.Put(cacheKey, storeStatusAndTags, KrogerCacheTTL, 0)
		} else {
			Cache.Put(storeCacheKey, storeStatusAndTags, KrogerCacheTTL, 0)
		}
	}

	if !storeFound {
		Log.Errorf("%s: Store ID %s not found in zip code %s!", s.Name(), s.LocationNo, s.Zipcode)
	}

	return
}
