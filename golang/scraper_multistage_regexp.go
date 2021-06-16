package csg

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const ScraperTypeMultistageRegexp = "multistage_regexp"
const ParamKeyStages = "stages"
const ParamKeyRecursionType = "recursion_type"
const ParamKeyNextUrlRegexp = "next_url_regexp"
const ParamKeyUseProxy = "use_proxy"

const MagicPrevValue = "##PREVIOUS##"

var MagicPrevPattern = regexp.MustCompile(`##PREV_SUBMATCH_[1-9]+##`)

const MultistageRecursionTypeAny = "any"
const MultistageRecursionTypeFirst = ""

type ScraperMultistageRegexp struct {
	ScraperName     string
	ProxyProvider   ProxyProvider
	AvailableStatus Status
	Stages          []*ScraperMultiStageRegexpStage
}

type ScraperMultiStageRegexpStage struct {
	Endpoint             *Endpoint
	RecursionType        string
	UnavailablePattern   *regexp.Regexp
	AvailablePattern     *regexp.Regexp
	NextUrlPattern       *regexp.Regexp
	ErrorPattern         *regexp.Regexp
	LimitedThreshold     int
	NumApptsPattern      *regexp.Regexp
	NumApptsTakenPattern *regexp.Regexp
}

type ScraperMultistageRegexpFactory struct {
}

func (sf *ScraperMultistageRegexpFactory) Type() string {
	return ScraperTypeMultistageRegexp
}

func (sf *ScraperMultistageRegexpFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	scraper := new(ScraperMultistageRegexp)
	scraper.ScraperName = name
	scraper.AvailableStatus = StatusYes

	scrapers := map[string]Scraper{name: scraper}
	return scrapers, nil
}

func (s *ScraperMultistageRegexp) Type() string {
	return ScraperTypeMultistageRegexp
}

func (s *ScraperMultistageRegexp) Name() string {
	return s.ScraperName
}

func (s *ScraperMultistageRegexp) Configure(params map[string]interface{}) error {
	stages, err := getMapArrayRequired(params, ParamKeyStages)
	if err != nil {
		return err
	}

	_, useProxy := getStringOptional(params, ParamKeyUseProxy)
	if useProxy {
		s.ProxyProvider, err = NewStickyProxyProviderDefaults()
		if err != nil {
			Log.Errorf("%v", err)
		}
	}

	availableStatus, _ := getStringOptional(params, ParamKeyAvailableStatus)
	if len(availableStatus) > 0 {
		s.AvailableStatus = Status(availableStatus)
	}

	s.Stages = make([]*ScraperMultiStageRegexpStage, 0)
	for _, stageParams := range stages {
		var err error
		stage := new(ScraperMultiStageRegexpStage)

		stage.Endpoint, err = getEndpointRequired(stageParams, ParamKeyEndpoint)
		if err != nil {
			return err
		}

		stage.RecursionType, _ = getStringOptional(stageParams, ParamKeyRecursionType)
		stage.UnavailablePattern = getPatternOptional(stageParams, ParamKeyUnavailableRegexp)
		stage.AvailablePattern = getPatternOptional(stageParams, ParamKeyAvailableRegexp)
		stage.NextUrlPattern = getPatternOptional(stageParams, ParamKeyNextUrlRegexp)
		stage.ErrorPattern = getPatternOptional(stageParams, ParamKeyErrorRegexp)
		stage.LimitedThreshold, _ = getIntOptionalWithDefault(stageParams, ParamKeyLimitedThreshold, config.LimitedThreshold)
		stage.NumApptsPattern = getPatternOptional(stageParams, ParamKeyNumAppts)
		stage.NumApptsTakenPattern = getPatternOptional(stageParams, ParamKeyNumApptsTaken)

		s.Stages = append(s.Stages, stage)
	}

	Log.Debugf("%s: Configured %d stages", s.Name(), len(s.Stages))

	return nil
}

func (s *ScraperMultistageRegexp) ScrapeUrls(urls ...string) (status Status, tags TagSet, body []byte, err error) {
	if len(urls) == 0 {
		return s.Scrape()
	} else if len(urls) == 1 {
		if len(s.Stages) > 0 {
			s.Stages[0].Endpoint.Url = urls[0]
		}
		return s.Scrape()
	} else {
		status = StatusPossible
		err = fmt.Errorf("Scraping multiple urls is unsupported")
		return
	}
}

func (s *ScraperMultistageRegexp) Scrape() (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown
	if len(s.Stages) > 0 {
		var proxyEndpoint ProxyEndpoint

		if s.ProxyProvider != nil {
			proxyEndpoint, err = s.ProxyProvider.GetProxy()
			if err != nil {
				return
			}

			for _, stage := range s.Stages {
				stage.Endpoint.HttpClient = proxyEndpoint.GetHttpClient()
			}
		}

		status, body, err = s.ScrapeRecursive(0, nil)
		if status == StatusUnknown && proxyEndpoint != nil {
			proxyEndpoint.BlackList()
		}
		return
	}

	status = StatusPossible
	err = fmt.Errorf("No stages configured!")
	return
}

func (s *ScraperMultistageRegexp) ScrapeRecursive(idx int, prevMatch []string) (status Status, body []byte, err error) {
	if idx >= len(s.Stages) {
		//reached the end of all stages and still no yes or no match
		//set status as possible so developer can take a look
		return StatusPossible, body, nil
	}

	stage := s.Stages[idx]
	originalUrl := stage.Endpoint.Url
	decorateEndpoint(s.Name(), stage.Endpoint, prevMatch)

	body, _, err = stage.Endpoint.FetchCached(s.Name())
	if err != nil {
		return StatusUnknown, body, err
	}

	fetchUrl := stage.Endpoint.Url
	stage.Endpoint.Url = originalUrl

	if stage.AvailablePattern != nil && stage.AvailablePattern.Match(body) {
		Log.Debugf("%s: stage %d: Available pattern matched", s.Name(), idx)

		if stage.LimitedThreshold > 0 && stage.NumApptsPattern != nil {
			totalAppointments := GetRegexCount(s.Name(), stage.NumApptsPattern, body)

			if stage.NumApptsTakenPattern != nil {
				totalAppointments -= GetRegexCount(s.Name(), stage.NumApptsTakenPattern, body)
			}

			Log.Debugf("%s: stage %d: total appointments: %d", s.Name(), idx, totalAppointments)

			if totalAppointments <= stage.LimitedThreshold {
				status = StatusLimited
			} else {
				status = s.AvailableStatus
			}
		} else {
			status = s.AvailableStatus
		}

		return
	} else if stage.UnavailablePattern != nil && stage.UnavailablePattern.Match(body) {
		Log.Debugf("%s: stage %d: Unavailable pattern matched", s.Name(), idx)
		return StatusNo, body, nil
	} else if stage.ErrorPattern != nil && stage.ErrorPattern.Match(body) {
		return StatusUnknown, body, fmt.Errorf("Error pattern matched")
	} else if stage.NextUrlPattern != nil {
		matches := stage.NextUrlPattern.FindAllStringSubmatch(string(body), -1)

		if len(matches) == 0 {
			Log.Warnf("%s: stage %d: did not match any next urls", s.Name(), idx)
			return StatusPossible, body, nil
		}

		for _, match := range matches {
			Log.Debugf("%s: stage %d: matched url value: %s", s.Name(), idx, match)

			if s.Stages[idx].RecursionType == MultistageRecursionTypeFirst {
				//default, just return the first recursion branch
				return s.ScrapeRecursive(idx+1, match)
			}
		}

		if s.Stages[idx].RecursionType == MultistageRecursionTypeAny {
			Log.Debugf("%s: stage %d: recursion any", s.Name(), idx)

			var anyNo, anyLimited bool
			var anyNoBody, anyLimitedBody, anyBody []byte

			for _, match := range matches {
				status, body, err = s.ScrapeRecursive(idx+1, match)
				if err != nil {
					return status, body, err
				}

				if anyBody == nil {
					anyBody = body
				}

				if status == s.AvailableStatus {
					return
				}
				if status == StatusLimited {
					anyLimited = true
					anyLimitedBody = body
				}
				if status == StatusNo && !anyNo {
					anyNo = true
					anyNoBody = body
				}
			}

			if anyLimited {
				return StatusLimited, anyLimitedBody, nil
			} else if anyNo {
				return StatusNo, anyNoBody, nil
			} else {
				return StatusPossible, anyBody, nil
			}
		}

		Log.Errorf("%s: Sanity check failed, this should never be reached", s.Name())
		return StatusUnknown, body, nil
	} else {
		Log.Debugf("%s: Stage %d (%s) returning Possible", s.Name(), idx, fetchUrl)
		return StatusPossible, body, nil
	}
}

func decorateEndpoint(name string, endpoint *Endpoint, prevMatch []string) {
	endpoint.Url = replacePrevMagic(name, endpoint.Url, prevMatch)
	endpoint.Body = replacePrevMagic(name, endpoint.Body, prevMatch)
}

func replacePrevMagic(name string, source string, prevMatch []string) string {
	dest := source

	if strings.Contains(source, MagicPrevValue) {
		if len(prevMatch) > 0 {
			//use last submatch
			dest = strings.ReplaceAll(source, MagicPrevValue, prevMatch[len(prevMatch)-1])
		} else {
			Log.Errorf("%s: Found PREVIOUS pattern, but no matches found in previous url", name)
		}
	}

	magicSubmatches := MagicPrevPattern.FindAllString(source, -1)
	for _, magicSubmatch := range magicSubmatches {
		magicSubmatchIndex, err := strconv.Atoi(magicSubmatch[16 : len(magicSubmatch)-2])
		if err != nil {
			Log.Errorf("%s: Malformed PREV_SUBMATCH pattern: %s: %v", name, magicSubmatch, err)
			continue
		}

		if len(prevMatch) < 2 {
			Log.Errorf("%s: Found PREV_SUBMATCH pattern(s), but no submatches found in previous url", name)
			continue
		}

		if magicSubmatchIndex < 1 || magicSubmatchIndex >= len(prevMatch) {
			Log.Errorf("%s: PREV_SUBMATCH index out of range: expected [1,%d), got %d", name, len(prevMatch), magicSubmatchIndex)
			continue
		}

		dest = strings.ReplaceAll(source, magicSubmatch, prevMatch[magicSubmatchIndex])
	}

	return dest
}
