package csg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const ScraperTypeWalgreens = "walgreens_direct"

const WalgreensXsrfSubmatchValue = "XSRFVALUE"
const WalgreensXsrfSubmatchHeader = "XSRFHEADER"
const WalgreensXsrfCookieName = "XSRF-TOKEN"

var WalgreensXsrfPattern = regexp.MustCompile(fmt.Sprintf(`(?m)<meta name="_csrf" content="(?P<%s>.+)"\s*/>\s*<meta name="_csrfHeader" content="(?P<%s>.+)"\s*/>`, WalgreensXsrfSubmatchValue, WalgreensXsrfSubmatchHeader))

var Walgreens2FARefIdPattern = regexp.MustCompile(`(?i)refId=([^"]+)`)
var WalgreensNoDataPattern = regexp.MustCompile(`{"errors?":\[{"code":"FC_[0-9]{3}_NoData","message":"[^"]+"}\]}`)

const WalgreensUseProxy = true
const WalgreensCacheTTL = 300
const WalgreensEndpointReuseMax = 200

const WalgreensSensorDataFilePath = "./walgreens_sensor_data.yaml"

const WalgreensParamKeyFineLoc = "fine_loc"
const WalgreensParamKeyCoarseLoc = "coarse_loc"
const WalgreensParamKeyCoarseRadius = "coarse_radius"

const WalgreensMaxLocations = 10

var walgreensFetcherSingletonLock = new(sync.Mutex)
var walgreensFetcherSingletonInstance *WalgreensFetcher = nil

type WalgreensFetcher struct {
	SensorData    *AkamaiSensorData
	XsrfPattern   *regexp.Regexp
	ProxyProvider ProxyProvider
	epCacheKey    string
	mutex         *sync.Mutex
}

type WalgreensFetcherCacheData struct {
	Endpoint      *Endpoint
	ProxyEndpoint ProxyEndpoint
	SensorData    *AkamaiSensorDataItem
	Xsrf          Header
	XsrfCookie    string
}

func NewWalgreensFetcher() *WalgreensFetcher {
	walgreensFetcherSingletonLock.Lock()
	defer walgreensFetcherSingletonLock.Unlock()

	if walgreensFetcherSingletonInstance != nil {
		return walgreensFetcherSingletonInstance
	}

	walgreensFetcherSingletonInstance = new(WalgreensFetcher)
	fetcher := walgreensFetcherSingletonInstance
	fetcher.SensorData = ParseAkamaiSensorData(WalgreensSensorDataFilePath)
	fetcher.XsrfPattern = regexp.MustCompile(fmt.Sprintf(`<meta name="_csrf" content="(?P<%s>.+)"\s*/>\s*<meta name="_csrf_header" content="(?P<%s>.+)"\s*/>`, WalgreensXsrfSubmatchValue, WalgreensXsrfSubmatchHeader))
	fetcher.mutex = new(sync.Mutex)

	if WalgreensUseProxy {
		var err error
		fetcher.ProxyProvider, err = NewProxyRackAuthHttpProxyProviderDefaults()
		if err != nil {
			Log.Errorf("WalgreensFetcher: %v", err)
		}
	}

	return fetcher
}

func (cg *WalgreensFetcher) reportProxyError() {
	if cachedEpData, ok := Cache.Clear(cg.epCacheKey).(*WalgreensFetcherCacheData); ok {
		if WalgreensUseProxy && cachedEpData.ProxyEndpoint != nil {
			cachedEpData.ProxyEndpoint.BlackList()
		}
	}
}

// prepares a "pre-authed" endpoint (past akamai filters) that can be used to scrape data
// Returns strings and nils if no sensor data is available
func (cg *WalgreensFetcher) Fetch(lat float64, lng float64, radius int) ([]byte, error) {
	cg.mutex.Lock()
	defer cg.mutex.Unlock()

	cookies := Header{
		Name:  "Cookie",
		Value: "",
	}
	accept := Header{
		Name:  "Accept",
		Value: "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
	}
	acceptEncoding := Header{
		Name:  "Accept-Encoding",
		Value: "gzip, br",
	}
	contentType := Header{
		Name:  "Content-Type",
		Value: "text/plain;charset=UTF-8",
	}
	contentLength := Header{
		Name:  "Content-Length",
		Value: "",
	}
	userAgent := Header{
		Name:  "User-Agent",
		Value: "",
	}

	var cachedData *WalgreensFetcherCacheData
	var err error
	var ok bool

	name := fmt.Sprintf("WalgreensFetcher (%.5f, %.5f, %d)", lat, lng, radius)

	if cachedData, ok = Cache.GetOrLock(cg.epCacheKey).(*WalgreensFetcherCacheData); ok {
		Log.Debugf("Endpoint (cached) uses left: %d", Cache.UsesLeft(cg.epCacheKey))
		userAgent.Value = cachedData.SensorData.UserAgent
	} else {
		defer Cache.Unlock(cg.epCacheKey)
		Log.Debugf("Endpoint (new) uses left: %d", WalgreensEndpointReuseMax)

		cachedData = new(WalgreensFetcherCacheData)
		cachedData.SensorData, err = cg.SensorData.GetSensorData()
		if err != nil {
			return nil, err
		}
		cachedData.Endpoint = new(Endpoint)
		if WalgreensUseProxy && cg.ProxyProvider != nil {
			cachedData.ProxyEndpoint, err = cg.ProxyProvider.GetProxy()
			if err != nil {
				return nil, err
			}
			cachedData.Endpoint.HttpClient = cachedData.ProxyEndpoint.GetHttpClient()
		}

		Cache.Put(cg.epCacheKey, cachedData, WalgreensCacheTTL, WalgreensEndpointReuseMax)

		userAgent.Value = cachedData.SensorData.UserAgent

		// CALL 1: LOGIN PAGE

		endpoint := cachedData.Endpoint
		endpoint.Url = "https://www.walgreens.com/login.jsp"
		endpoint.Method = "GET"
		endpoint.Headers = []Header{acceptEncoding}
		endpoint.Cookies = make(map[string]string)
		endpoint.CookieWhitelist = []string{"*"}

		body, _, err := endpoint.Fetch(name)
		if err != nil {
			return nil, err
		}

		xsrfValues := GetRegexSubmatches(WalgreensXsrfPattern, string(body))

		Log.Debugf("xsrfValues: %v", xsrfValues)

		if _, exists := xsrfValues[WalgreensXsrfSubmatchValue]; !exists {
			return nil, fmt.Errorf("Could not get XSRF Token Value from %s", endpoint.Url)
		}

		if _, exists := xsrfValues[WalgreensXsrfSubmatchHeader]; !exists {
			return nil, fmt.Errorf("Could not get XSRF Token Header from %s", endpoint.Url)
		}

		cachedData.Xsrf = Header{
			Name:  xsrfValues[WalgreensXsrfSubmatchHeader],
			Value: xsrfValues[WalgreensXsrfSubmatchValue],
		}

		if _, exists := endpoint.Cookies[WalgreensXsrfCookieName]; !exists {
			return nil, fmt.Errorf("Could not get XSRF Cookie '%s' from %s", WalgreensXsrfCookieName, endpoint.Url)
		}

		cachedData.XsrfCookie = endpoint.Cookies[WalgreensXsrfCookieName]

		// CALL 2: POST SENSOR DATA

		contentLength.Value = fmt.Sprintf("%d", len(cachedData.SensorData.SensorData))

		endpoint.Url = fmt.Sprintf("https://www.walgreens.com/%s", cachedData.SensorData.SensorDest)
		endpoint.Method = "POST"
		endpoint.Body = cachedData.SensorData.SensorData
		endpoint.Headers = []Header{userAgent, accept, acceptEncoding, contentType, contentLength}
		endpoint.CookieWhitelist = []string{"*"}
		endpoint.AllowedStatusCodes = []int{201}

		success, _, err := endpoint.Fetch(name)
		cachedData.SensorData.MarkUsed()
		if err != nil {
			return nil, err
		}

		if !strings.Contains(string(success), AkamaiSensorSuccess) {
			return nil, fmt.Errorf("Unexpected message: %s, was expecting %s", string(success), AkamaiSensorSuccess)
		}

		// CALL 3: POST CREDS
		creds := `{"username":"dfw6kzqdk5ez2mom9qhi@gmail.com","password":"svykhdc2dg82sb5sr5tz43l1f0f3xh"}`

		contentType.Value = "application/json"
		contentLength.Value = fmt.Sprintf("%d", len(creds))

		endpoint.Url = "https://www.walgreens.com/profile/v1/login"
		endpoint.Method = "POST"
		endpoint.Body = creds
		endpoint.Headers = []Header{userAgent, accept, acceptEncoding, contentType, cachedData.Xsrf, contentLength, cookies}
		endpoint.CookieWhitelist = []string{"*"}
		endpoint.AllowedStatusCodes = []int{}

		body, _, err = endpoint.Fetch(name)
		if err != nil {
			return nil, err
		}

		if _, exists := endpoint.Cookies["jwt"]; !exists {
			// CALL 4: POST SECURITY ANSWER
			refIdMatch := Walgreens2FARefIdPattern.FindStringSubmatch(string(body))
			if len(refIdMatch) < 2 {
				return nil, fmt.Errorf("Could not get Ref ID from %s", body)
			}

			refId := refIdMatch[1]
			creds = fmt.Sprintf(`{"type":"SecurityAnswer","code":"0tvyc8ubt6ra8x0pqzephmlhoain7m3vnm5fihek","refId":"%s","qcode":4}`, refId)
			contentLength.Value = fmt.Sprintf("%d", len(creds))

			endpoint.Url = "https://www.walgreens.com/profile/v1/authenticate"
			endpoint.Method = "POST"
			endpoint.Body = creds
			endpoint.Headers = []Header{userAgent, accept, acceptEncoding, contentType, cachedData.Xsrf, contentLength, cookies}
			endpoint.CookieWhitelist = []string{"*"}

			_, _, err = endpoint.Fetch(name)
			if err != nil {
				return nil, err
			}
		}

		if _, exists := endpoint.Cookies["jwt"]; !exists {
			err = fmt.Errorf("Could not find JWT in login response cookies")
			return nil, err
		}

		Log.Debugf("JWT=%s", endpoint.Cookies["jwt"])
	}

	endpoint := cachedData.Endpoint

	// CALL 5: INVOKE API
	postBody := fmt.Sprintf(`{"position":{"latitude":%s,"longitude":%s},"state":"WA","vaccine":{"productId":""},"appointmentAvailability":{"startDateTime":"##TOMORROW_DATE##"},"radius":%d,"size":150,"serviceId":"99"}`, strconv.FormatFloat(lat, 'f', -1, 64), strconv.FormatFloat(lng, 'f', -1, 64), radius)

	cacheControl := Header{
		Name:  "Cache-Control",
		Value: "no-cache",
	}

	contentType.Value = "application/json"
	contentLength.Value = fmt.Sprintf("%d", len(postBody))

	endpoint.Url = "https://www.walgreens.com/hcschedulersvc/svc/v2/immunizationLocations/timeslots"
	endpoint.Method = "POST"
	endpoint.Headers = []Header{accept, acceptEncoding, contentType, cachedData.Xsrf, cookies, cacheControl}
	endpoint.Body = postBody
	endpoint.Cookies = map[string]string{"jwt": endpoint.Cookies["jwt"], WalgreensXsrfCookieName: cachedData.XsrfCookie}
	endpoint.CookieWhitelist = []string{}
	endpoint.AllowedStatusCodes = []int{404}
	if cachedData.ProxyEndpoint != nil {
		endpoint.HttpClient = cachedData.ProxyEndpoint.GetHttpClient()
	}

	body, _, err := endpoint.Fetch(name)

	return body, err
}

type ScraperWalgreens struct {
	ScraperName      string
	Fetcher          *WalgreensFetcher
	StoreNumber      string
	CoarseLoc        GeoCoord
	CoarseRadius     int
	FineLoc          GeoCoord
	LimitedThreshold int
}

type ScraperWalgreensFactory struct {
}

func (sf *ScraperWalgreensFactory) Type() string {
	return ScraperTypeWalgreens
}

func (sf *ScraperWalgreensFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	SeedRand()

	clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`^walgreens_[0-9]+$`))
	if err != nil {
		return nil, err
	}
	scrapers := make(map[string]Scraper)

	for _, clinic := range clinics {
		scraper := new(ScraperWalgreens)
		scraper.ScraperName = clinic.ApiKey
		scraper.StoreNumber = clinic.ApiKey[10:]
		scraper.Fetcher = NewWalgreensFetcher()

		if len(clinic.ScraperConfig) > 0 {
			scraperConfig := make(map[string]interface{})
			err = json.Unmarshal([]byte(clinic.ScraperConfig), &scraperConfig)
			if err != nil {
				return nil, err
			} else {
				err = scraper.Configure(scraperConfig)
				if err != nil {
					return nil, err
				}
			}
		}

		scrapers[scraper.Name()] = scraper
	}

	return scrapers, nil
}

func (s *ScraperWalgreens) Type() string {
	return ScraperTypeWalgreens
}

func (s *ScraperWalgreens) Name() string {
	return s.ScraperName
}

func (s *ScraperWalgreens) Configure(params map[string]interface{}) error {
	var err error

	s.LimitedThreshold = config.LimitedThreshold

	if s.FineLoc.Zero() {
		s.FineLoc, err = getGeoCoordRequired(params, WalgreensParamKeyFineLoc)
		if err != nil {
			return err
		}
	}

	if s.CoarseRadius <= 0 {
		var exists bool
		s.CoarseRadius, exists = getIntOptionalWithDefault(params, WalgreensParamKeyCoarseRadius, 0)
		if exists {
			s.CoarseLoc, err = getGeoCoordRequired(params, WalgreensParamKeyCoarseLoc)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *ScraperWalgreens) Scrape() (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	var apiResp *WalgreensAPIResp

	if s.CoarseRadius > 0 {
		apiResp, body, err = s.ScrapeCoord(s.CoarseLoc, s.CoarseRadius)
	} else {
		apiResp, body, err = s.ScrapeCoord(s.FineLoc, 1)
	}
	if err != nil {
		return
	}

	countAndTags := s.GetAvailable(s.StoreNumber)
	if countAndTags.Count > s.LimitedThreshold {
		status = StatusYes
		tags = tags.Merge(countAndTags.TagSet)
		return
	} else if countAndTags.Count > 0 {
		status = StatusLimited
		tags = tags.Merge(countAndTags.TagSet)
		return
	}

	if len(apiResp.Locations) >= WalgreensMaxLocations {
		if len(apiResp.Locations) > WalgreensMaxLocations {
			Log.Errorf("%s: Walgreens API returned more than the expected number of locations: %d > %d", s.Name(), len(apiResp.Locations), WalgreensMaxLocations)
		}

		_, body, err = s.ScrapeCoord(s.FineLoc, 1)
		if err != nil {
			return
		}

		countAndTags := s.GetAvailable(s.StoreNumber)
		if countAndTags.Count > s.LimitedThreshold {
			status = StatusYes
			tags = tags.Merge(countAndTags.TagSet)
			return
		} else if countAndTags.Count > 0 {
			status = StatusLimited
			tags = tags.Merge(countAndTags.TagSet)
			return
		}
	}

	status = StatusNo

	return
}

type WalgreensAPIResp struct {
	Locations []WalgreensLocation `json:"locations"`
	ErrorBody string
}

type WalgreensLocation struct {
	StoreNumber   string                  `json:"partnerLocationId"`
	Availability  []WalgreensAvail        `json:"appointmentAvailability"`
	Manufacturers []WalgreensManufacturer `json:"manufacturer"`
}

type WalgreensAvail struct {
	Date       string   `json:"date"`
	Day        string   `json:"day"`
	Slots      []string `json:"slots"`
	Restricted bool     `json:"restricted"`
}

type WalgreensManufacturer struct {
	Name string `json:"name"`
}

func (s *ScraperWalgreens) ScrapeCoord(coord GeoCoord, radius int) (apiResp *WalgreensAPIResp, body []byte, err error) {
	var ok bool
	cacheKey := fmt.Sprintf("walgreens-%s-%d", coord.String(), radius)
	apiResp, ok = Cache.GetOrLock(cacheKey).(*WalgreensAPIResp)

	if !ok || apiResp == nil {
		defer Cache.Unlock(cacheKey)

		body, err = s.Fetcher.Fetch(coord.Lat, coord.Lng, radius)
		if err != nil {
			s.Fetcher.reportProxyError()
			return nil, body, err
		}

		apiResp = new(WalgreensAPIResp)
		err = json.Unmarshal(body, apiResp)
		if err != nil {
			return nil, body, err
		}

		if len(apiResp.Locations) < 1 {
			apiResp.ErrorBody = string(body)
		}

		Cache.Put(cacheKey, apiResp, WalgreensCacheTTL, -1)
	}

	if len(apiResp.Locations) < 1 {
		if !WalgreensNoDataPattern.MatchString(apiResp.ErrorBody) {
			err = fmt.Errorf("Unknown API error: %s", apiResp.ErrorBody)
		}
		return
	}

	for _, loc := range apiResp.Locations {
		totalAvail := 0
		for _, avail := range loc.Availability {
			if !avail.Restricted && len(avail.Slots) > 0 {
				totalAvail += len(avail.Slots)
			}
		}

		if totalAvail > 0 {
			var countAndTags CountAndTagSet
			countAndTags.Count = totalAvail
			for _, mfg := range loc.Manufacturers {
				countAndTags.TagSet = countAndTags.TagSet.ParseAndAddVaccineType(mfg.Name)
			}
			s.SetAvailable(loc.StoreNumber, countAndTags)
		}
	}

	return
}

func (s *ScraperWalgreens) SetAvailable(store string, countAndTags CountAndTagSet) {
	cacheKey := fmt.Sprintf("walgreensStore|%s", store)
	available := Cache.GetOrLock(cacheKey)
	if available == nil {
		defer Cache.Unlock(cacheKey)
		Cache.Put(cacheKey, countAndTags, WalgreensCacheTTL, -1)
	}
}

func (s *ScraperWalgreens) GetAvailable(store string) CountAndTagSet {
	cacheKey := fmt.Sprintf("walgreensStore|%s", store)
	available := Cache.GetOrLock(cacheKey)
	if available == nil {
		defer Cache.Unlock(cacheKey)
		return CountAndTagSet{
			Count: 0,
		}
	} else {
		return available.(CountAndTagSet)
	}
}
