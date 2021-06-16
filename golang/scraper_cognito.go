package csg

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const ScraperTypeCognito = "cognito"

const CognitoFormUrl = "https://www.cognitoforms.com/forms/public?id=%s&isPublicLink=%t&entry=%s&accessToken=%s"

var CognitoSessionScriptPattern = regexp.MustCompile(`https://www.cognitoforms.com/session/script/[a-f0-9\-]+`)
var CognitoFormParamPattern = regexp.MustCompile(`{"id":"[0-9]+","isPublicLink":(?:true|false),"entry":"[^"]*","accessToken":"[^"]*"}`)
var CognitoDateUrlPattern = regexp.MustCompile(`\?\s+"(https://www.cognitoforms.com/[^"]+)"`)
var CognitoSessionTokenPattern = regexp.MustCompile(`sessionToken:"([^"]+)"`)
var CognitoUnavailablePattern = regexp.MustCompile(`(?s)<div class='c-forms-not-available-message'>([^>]+)</div>`)

type ScraperCognito struct {
	ScraperName  string
	Url          string
	AlternateUrl string
}

type ScraperCognitoFactory struct {
}

func (sf *ScraperCognitoFactory) Type() string {
	return ScraperTypeCognito
}

func (sf *ScraperCognitoFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	if name == "cognito" {
		//scrapers from airtable

		clinics, err := GetClinicsByKeyPattern(regexp.MustCompile(`(^cognito_.+$)`))
		if err != nil {
			return nil, err
		}
		scrapers := make(map[string]Scraper)

		for _, clinic := range clinics {
			scraper := new(ScraperCognito)
			scraper.ScraperName = clinic.ApiKey
			scraper.Url = clinic.Url
			scraper.AlternateUrl = clinic.AlternateUrl

			scrapers[scraper.Name()] = scraper
		}

		return scrapers, nil
	} else {
		//scrapers from yaml
		scraper := new(ScraperCognito)
		scraper.ScraperName = name

		return map[string]Scraper{name: scraper}, nil
	}
}

func (s *ScraperCognito) Type() string {
	return ScraperTypeCognito
}

func (s *ScraperCognito) Name() string {
	return s.ScraperName
}

func (s *ScraperCognito) Configure(params map[string]interface{}) error {
	url, exists := getStringOptional(params, "url")
	if exists && len(url) > 0 {
		s.Url = url
	}

	return nil
}

type CognitoFormParams struct {
	Id           string `json:"id"`
	IsPublicLink bool   `json:"isPublicLink"`
	Entry        string `json:"entry"`
	AccessToken  string `json:"accessToken"`
}

func (s *ScraperCognito) Scrape() (status Status, tags TagSet, body []byte, err error) {
	return s.ScrapeUrls(s.AlternateUrl, s.Url)
}

func (s *ScraperCognito) ScrapeUrls(urls ...string) (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	body, err = s.ScrapeForm(urls...)
	matches := CognitoDateUrlPattern.FindAllStringSubmatch(string(body), -1)

	for _, match := range matches {
		dateUrl := match[len(match)-1]

		body, err = s.ScrapeForm(dateUrl)
		if err != nil {
			Log.Warnf("%v", err)
			err = nil
			continue
		}

		if match2 := CognitoUnavailablePattern.FindStringSubmatch(string(body)); match2 != nil {
			site := strings.TrimPrefix(dateUrl, "https://www.cognitoforms.com/")
			unavailMessage := match2[len(match2)-1]
			unavailMessage = strings.Split(unavailMessage, "\n")[0]
			Log.Infof("%s: %s unavailable: %s", s.Name(), site, unavailMessage)
		} else {
			status = StatusYes
			return
		}
	}

	status = StatusNo
	return
}

func (s *ScraperCognito) ScrapeForm(urls ...string) (body []byte, err error) {
	var sessionScriptUrl string
	sessionScriptUrl, body, err = ExtractScrapeUrl(s.Name(), CognitoSessionScriptPattern, urls...)
	if err != nil {
		return
	}

	var sessionToken string
	sessionToken, body, err = ExtractScrapeUrl(s.Name(), CognitoSessionTokenPattern, sessionScriptUrl)
	if err != nil {
		return
	}
	Log.Debugf("%s: Session Token: %s", s.Name(), sessionToken)

	var formParamsJsonStr string
	formParamsJsonStr, body, err = ExtractScrapeUrl(s.Name(), CognitoFormParamPattern, urls...)
	if err != nil {
		return
	}

	jsonData := new(CognitoFormParams)
	err = json.Unmarshal([]byte(formParamsJsonStr), jsonData)
	if err != nil {
		return
	}

	endpoint := new(Endpoint)
	endpoint.Url = fmt.Sprintf(CognitoFormUrl, jsonData.Id, jsonData.IsPublicLink, jsonData.Entry, jsonData.AccessToken)
	endpoint.Method = "GET"
	endpoint.Headers = []Header{
		Header{
			Name:  "X-SessionToken",
			Value: sessionToken,
		},
	}
	body, _, err = endpoint.FetchCached(s.Name())
	return
}
