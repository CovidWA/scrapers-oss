package csg

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Status string

const (
	StatusYes      Status = "Yes"
	StatusLimited  Status = "Limited"
	StatusCall     Status = "Call"
	StatusWaitList Status = "Waitlist"
	StatusNo       Status = "No"
	StatusPossible Status = "Possible"
	StatusUnknown  Status = "Unknown"
	StatusApiSkip  Status = "APISkip"
	StatusApifail  Status = "APIFail"
)

type Tag string

const (
	TagPfizer  Tag = "pfizer"
	TagModerna Tag = "moderna"
	TagJohnson Tag = "johnson"
)

var PfizerTagPattern = regexp.MustCompile(`(?i)pfizer`)
var ModernaTagPattern = regexp.MustCompile(`(?i)moderna`)
var JohnsonTagPattern = regexp.MustCompile(`(?i)j&j|jnj|johnson|janssen`)

const ParamKeyEndpoint = "endpoint"
const ParamKeyLat = "lat"
const ParamKeyLng = "lng"
const ParamKeyLimitedThreshold = "limited_threshold"
const ParamKeyNumAppts = "num_appts_regexp"
const ParamKeyNumApptsTaken = "num_appts_taken_regexp"

var ListOfNumbersPattern = regexp.MustCompile(`^\d+(?:[,\s]+\d+)*$`)
var ListOfNumbersDelimiterPattern = regexp.MustCompile(`[,\s]+`)

var OfficeFormsAPIUrlPattern = regexp.MustCompile(`(?i)https?://forms.office.com/formapi/api/[^"]+`)

const MagicTimestamp = "##CURRENT_TIMESTAMP##"
const MagicDate = "##CURRENT_DATE##"
const MagicDateTomorrow = "##TOMORROW_DATE##"
const MagicDateNextweek = "##NEXTWEEK_DATE##"
const MagicDateNextmonth = "##NEXTMONTH_DATE##"
const MagicDateTimeGeneric = "##{[^;]+;[0-9]+;[0-9]+}##"

type TagSet struct {
	arr []Tag
}

func (ts TagSet) Contains(tag Tag) bool {
	for _, v := range ts.arr {
		if v == tag {
			return true
		}
	}

	return false
}

func (ts TagSet) Add(tag Tag) TagSet {
	if !ts.Contains(tag) {
		ts.arr = append(ts.arr, tag)
	}

	return ts
}

func (ts TagSet) ParseAndAddVaccineType(str string) TagSet {
	if PfizerTagPattern.MatchString(str) {
		ts = ts.Add(TagPfizer)
	}

	if ModernaTagPattern.MatchString(str) {
		ts = ts.Add(TagModerna)
	}

	if JohnsonTagPattern.MatchString(str) {
		ts = ts.Add(TagJohnson)
	}
	return ts
}

func (ts TagSet) ToStringArray() []string {
	strArr := make([]string, 0, len(ts.arr))
	for _, tag := range ts.arr {
		strArr = append(strArr, string(tag))
	}

	return strArr
}

func (ts TagSet) Merge(ts2 TagSet) TagSet {
	for _, tag := range ts2.arr {
		ts = ts.Add(tag)
	}

	return ts
}

type StatusAndTagSet struct {
	Status Status
	TagSet TagSet
}

type CountAndTagSet struct {
	Count  int
	TagSet TagSet
}

type Scraper interface {
	Type() string
	Name() string
	Configure(params map[string]interface{}) error
	Scrape() (status Status, tags TagSet, body []byte, err error)
}

type UrlScraper interface {
	ScrapeUrls(urls ...string) (status Status, tags TagSet, body []byte, err error)
}

type ScraperFactory interface {
	Type() string
	CreateScrapers(name string) (map[string]Scraper, error)
}

type ClinicsAPIResp struct {
	Timestamp int64    `json:"stamp"`
	Clinics   []Clinic `json:"data"`
}

type Clinic struct {
	Id            string `json:"id"`
	Name          string `json:"name"`
	ApiKey        string `json:"key"`
	Url           string `json:"url"`
	AlternateUrl  string `json:"alternateUrl"`
	ScraperConfig string `json:"scraper_config"`
}

type GeoCoord struct {
	Lat float64
	Lng float64
}

func (c GeoCoord) Zero() bool {
	return c.Lat == 0.0 && c.Lng == 0.0
}

func (c GeoCoord) String() string {
	return fmt.Sprintf("%f,%f", c.Lat, c.Lng)
}

var magicDateTimeGenericRE = regexp.MustCompile(MagicDateTimeGeneric)

func replaceMagic(input string) string {
	//force Pacific Time
	loc, _ := time.LoadLocation("America/Los_Angeles")
	now := time.Now().In(loc)

	return replaceMagicWithTime(input, now)
}

func replaceMagicWithTime(input string, now time.Time) string {
	currentTimestamp := fmt.Sprintf("%d", now.Unix())
	currentDate := now.Format("2006-01-02")
	tomorrowDate := now.AddDate(0, 0, 1).Format("2006-01-02")
	nextWeekDate := now.AddDate(0, 0, 7).Format("2006-01-02")
	nextMonthDate := now.AddDate(0, 1, 0).Format("2006-01-02")

	processed := input
	processed = strings.ReplaceAll(processed, MagicTimestamp, currentTimestamp)
	processed = strings.ReplaceAll(processed, MagicDate, currentDate)
	processed = strings.ReplaceAll(processed, MagicDateTomorrow, tomorrowDate)
	processed = strings.ReplaceAll(processed, MagicDateNextweek, nextWeekDate)
	processed = strings.ReplaceAll(processed, MagicDateNextmonth, nextMonthDate)

	genericMatches := magicDateTimeGenericRE.FindAllString(processed, -1)
	for _, genericMatch := range genericMatches {
		parts := strings.Split(genericMatch[3:len(genericMatch)-3], ";")
		dtFormat := parts[0]
		addMonths, err := strconv.Atoi(parts[1])
		if err != nil {
			addMonths = 0
		}
		addDays, err := strconv.Atoi(parts[2])
		if err != nil {
			addDays = 0
		}

		datetime := url.QueryEscape(now.AddDate(0, addMonths, addDays).Format(dtFormat))
		processed = strings.ReplaceAll(processed, genericMatch, datetime)
	}

	return processed
}

var seedRandOnce sync.Once

func SeedRand() {
	seedRandOnce.Do(func() {
		rand.Seed(time.Now().UnixNano())
	})
}

func getMapRequired(parent map[string]interface{}, key string) (map[string]interface{}, error) {
	if _, exists := parent[key]; !exists {
		return nil, fmt.Errorf("Missing expected configuration key: %s", key)
	}

	if keyTyped, ok := parent[key].(map[string]interface{}); ok {
		return keyTyped, nil
	}

	if typed, ok := parent[key].(map[interface{}]interface{}); !ok {
		return nil, fmt.Errorf("Expecting a map value for key %s, got '%T' instead", key, parent[key])
	} else {
		keyTyped := make(map[string]interface{})

		for k, v := range typed {
			if typedKey, ok := k.(string); !ok {
				return nil, fmt.Errorf("Expecting a string value key in map %s, got '%T' instead", key, k)
			} else {
				keyTyped[typedKey] = v
			}
		}

		return keyTyped, nil
	}
}

func getMapOptional(parent map[string]interface{}, key string) map[string]interface{} {
	if _, exists := parent[key]; exists {
		if value, err := getMapRequired(parent, key); err == nil {
			return value
		} else {
			Log.Warnf("%v", err)
			return nil
		}
	}
	return nil
}

//unused function, get linter to shut up
var _, _ = getIntArrayRequired(map[string]interface{}{"lol": []int{1}}, "lol")

func getIntArrayRequired(parent map[string]interface{}, key string) ([]int, error) {
	if _, exists := parent[key]; !exists {
		return nil, fmt.Errorf("Missing expected configuration key: %s", key)
	}

	if untypedArr, ok := parent[key].([]interface{}); !ok {
		return nil, fmt.Errorf("Expecting an array value for key %s, got '%T' instead", key, parent[key])
	} else {
		typedArray := make([]int, len(untypedArr))

		for idx, obj := range untypedArr {
			typedArray[idx], ok = untypedArr[idx].(int)
			if !ok {
				return nil, fmt.Errorf("Expecting int for index %d of array %s, got '%T' instead", idx, key, obj)
			}
		}

		return typedArray, nil
	}
}

func getMapArrayRequired(parent map[string]interface{}, key string) ([]map[string]interface{}, error) {
	if _, exists := parent[key]; !exists {
		return nil, fmt.Errorf("Missing expected configuration key: %s", key)
	}

	if untypedArr, ok := parent[key].([]interface{}); !ok {
		return nil, fmt.Errorf("Expecting an array value for key %s, got '%T' instead", key, parent[key])
	} else {
		typedArray := make([]map[string]interface{}, len(untypedArr))

		for idx, obj := range untypedArr {
			untypedMap, ok := obj.(map[interface{}]interface{})
			typedArray[idx] = make(map[string]interface{})
			for key2, val := range untypedMap {
				keyStr, ok := key2.(string)
				if !ok {
					return nil, fmt.Errorf("Expecting string key for index %d of array %s, got '%T' instead", idx, key, key2)
				}
				typedArray[idx][keyStr] = val
			}

			if !ok {
				return nil, fmt.Errorf("Expecting map for index %d of array %s, got '%T' instead", idx, key, obj)
			}
		}

		return typedArray, nil
	}
}

func getMapArrayOptional(parent map[string]interface{}, key string) []map[string]interface{} {
	if _, exists := parent[key]; exists {
		if value, err := getMapArrayRequired(parent, key); err == nil {
			return value
		} else {
			Log.Warnf("%v", err)
			return nil
		}
	}
	return nil
}

func getStringRequired(parent map[string]interface{}, key string) (string, error) {
	if _, exists := parent[key]; !exists {
		return "", fmt.Errorf("Missing expected configuration key: %s", key)
	}

	if typed, ok := parent[key].(string); !ok {
		return "", fmt.Errorf("Expecting a string value for key %s, got '%T' instead", key, parent[key])
	} else {
		return typed, nil
	}
}

func getStringOptional(parent map[string]interface{}, key string) (string, bool) {
	if _, exists := parent[key]; exists {
		if value, err := getStringRequired(parent, key); err == nil {
			return value, true
		} else {
			Log.Warnf("%v", err)
			return "", false
		}
	}
	return "", false
}

func getPatternRequired(parent map[string]interface{}, key string) (*regexp.Regexp, error) {
	patternStr, err := getStringRequired(parent, key)
	if err != nil {
		return nil, err
	}

	if len(patternStr) == 0 {
		return nil, fmt.Errorf("Found empty regexp pattern for key %s", key)
	}

	pattern, err := regexp.Compile(patternStr)
	if err != nil {
		return nil, err
	}

	return pattern, nil
}

func getPatternOptional(parent map[string]interface{}, key string) *regexp.Regexp {
	if patternStr, err := getStringRequired(parent, key); err == nil {
		if len(patternStr) == 0 {
			Log.Warnf("Found empty regexp pattern for key %s", key)
			return nil
		}

		pattern, err := regexp.Compile(patternStr)
		if err != nil {
			Log.Warnf("%v", err)
			return nil
		}

		return pattern
	} else {
		Log.Warnf("%v", err)
		return nil
	}
}

func getEndpointRequired(parent map[string]interface{}, key string) (*Endpoint, error) {
	if params, err := getMapRequired(parent, key); err == nil {
		if endpoint, err := NewEndpoint(params); err == nil {
			return endpoint, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func getEndpointOptional(parent map[string]interface{}, key string) *Endpoint {
	endpoint, err := getEndpointRequired(parent, key)
	if err != nil {
		Log.Warnf("%v", err)
	}

	return endpoint
}

func getIntOptionalWithDefault(parent map[string]interface{}, key string, defaultValue int) (int, bool) {
	if _, exists := parent[key]; exists {
		if value, ok := parent[key].(int); ok {
			return value, true
		} else if value, ok := parent[key].(float64); ok {
			return int(value), true
		} else {
			Log.Warnf("Expecting an int value for key %s, got '%T' instead", key, parent[key])
			return defaultValue, false
		}
	}
	return defaultValue, false
}

func getFloatRequired(parent map[string]interface{}, key string) (float64, error) {
	if _, exists := parent[key]; exists {
		if value, ok := parent[key].(float64); ok {
			return value, nil
		} else {
			return 0, fmt.Errorf("Expecting a float64 value for key %s, got '%T' instead", key, parent[key])
		}
	}
	return 0, fmt.Errorf("Missing expected configuration key: %s", key)
}

func getFloatOptional(parent map[string]interface{}, key string) (float64, bool) {
	if _, exists := parent[key]; exists {
		if value, ok := parent[key].(float64); ok {
			return value, true
		} else {
			Log.Warnf("Expecting a float64 value for key %s, got '%T' instead", key, parent[key])
			return 0, false
		}
	}
	return 0, false
}

func getGeoCoordRequired(parent map[string]interface{}, key string) (GeoCoord, error) {
	coord := GeoCoord{
		Lat: 0,
		Lng: 0,
	}

	genericMap, err := getMapRequired(parent, key)
	if err != nil {
		return coord, err
	}

	lat, err := getFloatRequired(genericMap, ParamKeyLat)
	if err != nil {
		return coord, err
	}

	lng, err := getFloatRequired(genericMap, ParamKeyLng)
	if err != nil {
		return coord, err
	}

	coord.Lat = lat
	coord.Lng = lng
	return coord, nil
}

//unused function
/*
func getGeoCoordOptional(parent map[string]interface{}, key string) (GeoCoord, bool) {
	coord, err := getGeoCoordRequired(parent, key)
	if err != nil {
		Log.Warnf("%v", err)
		return coord, false
	}

	return coord, true
}
*/

func getBool(parent map[string]interface{}, key string) bool {
	if _, exists := parent[key]; exists {
		if value, ok := parent[key].(bool); ok {
			return value
		} else {
			return true
		}
	}
	return false
}

func GetRegexSubmatches(re *regexp.Regexp, str string) (paramsMap map[string]string) {
	submatches := GetAllRegexSubmatches(re, str)
	if len(submatches) < 1 {
		return nil
	} else {
		return submatches[0]
	}
}

func GetAllRegexSubmatches(re *regexp.Regexp, str string) (submatches []map[string]string) {
	submatches = make([]map[string]string, 0)

	allSubmatchesRaw := re.FindAllStringSubmatch(str, -1)
	if len(allSubmatchesRaw) < 1 {
		Log.Warnf("Pattern NOT matched: %v", re)
	}

	for _, submatchesRaw := range allSubmatchesRaw {
		submatchMap := make(map[string]string)
		unnamedCounter := 1
		for i, submatchRaw := range submatchesRaw {
			if i > 0 {
				var name string
				if i >= len(re.SubexpNames()) || len(re.SubexpNames()[i]) < 1 {
					name = fmt.Sprintf("UNNAMED-%d", unnamedCounter)
					unnamedCounter++
				} else {
					name = re.SubexpNames()[i]
				}

				submatchMap[name] = submatchRaw
			}
		}

		submatches = append(submatches, submatchMap)
	}

	return submatches
}

func GetRegexCount(name string, re *regexp.Regexp, body []byte) int {
	if re == nil || body == nil {
		return -1
	}

	total := 0
	if len(re.SubexpNames()) < 2 {
		Log.Debugf("%s: matching single appointments", name)
		matches := re.FindAll(body, -1)
		total = len(matches)
	} else {
		Log.Debugf("%s: matching appointment counts", name)
		matches := GetAllRegexSubmatches(re, string(body))
		for _, match := range matches {
			for _, submatch := range match {
				if ListOfNumbersPattern.MatchString(submatch) {
					submatchCleaned := ListOfNumbersDelimiterPattern.ReplaceAllString(submatch, ",")
					for _, v := range strings.Split(submatchCleaned, ",") {
						n, parseErr := strconv.Atoi(v)
						if parseErr != nil {
							n = 0
							Log.Errorf("%s: %v", name, parseErr)
						}
						total += n
					}
				} else {
					Log.Errorf("%s: could not match number or list of numbers: %v, %s", name, ListOfNumbersPattern, submatch)
				}
			}
		}
	}

	return total
}

func GetClinicsByKeyPattern(re *regexp.Regexp) ([]Clinic, error) {
	if len(config.ApiInternalUrl) == 0 {
		return nil, fmt.Errorf("Internal Get API url (api_internal_url) not configured!")
	}

	endpoint := new(Endpoint)
	endpoint.Url = config.ApiInternalUrl
	endpoint.Method = "POST"
	endpoint.Headers = []Header{
		Header{
			Name:  "Accept-Encoding",
			Value: "gzip, br",
		},
		Header{
			Name:  "Content-Type",
			Value: "application/x-www-form-urlencoded",
		},
	}
	endpoint.Body = fmt.Sprintf("secret=%s", config.ApiSecret)

	var jsonBytes []byte
	var err error
	for i := 0; ; i++ {
		jsonBytes, _, err = endpoint.FetchCached("GetClinicsByKeyPattern")
		if err != nil {
			if i >= 2 {
				return nil, err
			} else {
				Log.Errorf("GetClinicsByKeyPattern: %v", err)
			}
		} else {
			break
		}
	}

	apiResp := ClinicsAPIResp{}
	err = json.Unmarshal(jsonBytes, &apiResp)
	if err != nil {
		return nil, err
	}

	Log.Debugf("Fetched %d total clinics", len(apiResp.Clinics))

	filteredClinics := make([]Clinic, 0)

	for _, clinic := range apiResp.Clinics {
		if re.MatchString(clinic.ApiKey) {
			filteredClinics = append(filteredClinics, clinic)
		}
	}

	Log.Debugf("Pattern matched %d clinics", len(filteredClinics))

	return filteredClinics, nil
}

func ExtractScrapeUrl(name string, pattern *regexp.Regexp, urls ...string) (string, []byte, error) {
	urls, body, err := ExtractScrapeUrls(name, pattern, urls...)
	if len(urls) > 0 {
		return urls[0], body, err
	} else {
		return "", body, err
	}
}

func ExtractScrapeUrlWithEndpoints(name string, pattern *regexp.Regexp, endpoint *Endpoint, proxyEndpoint ProxyEndpoint, urls ...string) (string, []byte, error) {
	urls, body, err := ExtractScrapeUrlsWithEndpoints(name, pattern, endpoint, proxyEndpoint, urls...)
	if len(urls) > 0 {
		return urls[0], body, err
	} else {
		return "", body, err
	}
}

func ExtractScrapeUrls(name string, pattern *regexp.Regexp, urls ...string) ([]string, []byte, error) {
	return ExtractScrapeUrlsWithEndpoints(name, pattern, nil, nil, urls...)
}

func ExtractScrapeUrlsWithEndpoints(name string, pattern *regexp.Regexp, endpoint *Endpoint, proxyEndpoint ProxyEndpoint, urls ...string) ([]string, []byte, error) {
	var body []byte
	var err error

	for _, candidateUrl := range urls {
		if len(candidateUrl) == 0 {
			continue
		}

		Log.Debugf("Trying %s", candidateUrl)

		if pattern.MatchString(candidateUrl) {
			matches := pattern.FindStringSubmatch(candidateUrl)
			return []string{string(matches[len(matches)-1])}, body, err
		} else {
			if endpoint == nil {
				endpoint = new(Endpoint)
				endpoint.Method = "GET"
			}

			endpoint.Url = candidateUrl
			if proxyEndpoint != nil {
				endpoint.HttpClient = proxyEndpoint.GetHttpClient()
			}

			//match scrape url from contents of provided endpoint
			body, _, err = endpoint.FetchCached(name)
			if err != nil {
				if proxyEndpoint != nil {
					proxyEndpoint.BlackList()
				}
				return nil, body, err
			}

			matches := pattern.FindAllSubmatch(body, -1)
			if len(matches) > 0 {
				matchStrings := make([]string, 0, len(matches))
				for _, match := range matches {
					//return first submatch, if any, or the whole match if none
					matchStrings = append(matchStrings, string(match[len(match)-1]))
				}
				return matchStrings, body, err
			} else if officeApiUrl := string(OfficeFormsAPIUrlPattern.Find(body)); len(officeApiUrl) > 0 {
				//if this is an office forms url, get url from api endpoint
				endpoint.Url = officeApiUrl
				body, _, err = endpoint.FetchCached(name)
				if err != nil {
					if proxyEndpoint != nil {
						proxyEndpoint.BlackList()
					}
					return nil, body, err
				}

				matches := pattern.FindAllSubmatch(body, -1)
				if len(matches) > 0 {
					matchStrings := make([]string, 0, len(matches))
					for _, match := range matches {
						//return first submatch, if any, or the whole match if none
						matchStrings = append(matchStrings, string(match[len(match)-1]))
					}
					return matchStrings, body, err
				}
			}
		}
	}

	return nil, body, fmt.Errorf("Could not resolve scrape url from urls: %v", urls)
}
