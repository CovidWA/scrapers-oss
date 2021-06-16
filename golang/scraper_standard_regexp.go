package csg

import (
	"fmt"
	"regexp"
)

const ScraperTypeStandardRegexp = "standard_regexp"
const ParamKeyUnavailableRegexp = "unavailable_regexp"
const ParamKeyAvailableRegexp = "available_regexp"
const ParamKeyErrorRegexp = "error_regexp"
const ParamKeyAvailableStatus = "available_status"

type ScraperStandardRegexp struct {
	ScraperName          string
	ScrapeEndpoint       *Endpoint
	UnavailablePattern   *regexp.Regexp
	AvailablePattern     *regexp.Regexp
	ErrorPattern         *regexp.Regexp
	LimitedThreshold     int
	NumApptsPattern      *regexp.Regexp
	NumApptsTakenPattern *regexp.Regexp
	AvailableStatus      Status
}

type ScraperStandardRegexpFactory struct {
}

func (sf *ScraperStandardRegexpFactory) Type() string {
	return ScraperTypeStandardRegexp
}

func (sf *ScraperStandardRegexpFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	scraper := new(ScraperStandardRegexp)
	scraper.ScraperName = name
	scraper.AvailableStatus = StatusYes

	scrapers := map[string]Scraper{name: scraper}
	return scrapers, nil
}

func (s *ScraperStandardRegexp) Type() string {
	return ScraperTypeStandardRegexp
}

func (s *ScraperStandardRegexp) Name() string {
	return s.ScraperName
}

func (s *ScraperStandardRegexp) Configure(params map[string]interface{}) error {
	var err error

	s.LimitedThreshold, _ = getIntOptionalWithDefault(params, ParamKeyLimitedThreshold, config.LimitedThreshold)

	s.ScrapeEndpoint, err = getEndpointRequired(params, ParamKeyEndpoint)
	if err != nil {
		return err
	}

	s.UnavailablePattern, err = getPatternRequired(params, ParamKeyUnavailableRegexp)
	if err != nil {
		return err
	}

	s.AvailablePattern = getPatternOptional(params, ParamKeyAvailableRegexp)
	s.ErrorPattern = getPatternOptional(params, ParamKeyErrorRegexp)
	s.NumApptsPattern = getPatternOptional(params, ParamKeyNumAppts)
	s.NumApptsTakenPattern = getPatternOptional(params, ParamKeyNumApptsTaken)

	availableStatus, _ := getStringOptional(params, ParamKeyAvailableStatus)
	if len(availableStatus) > 0 {
		s.AvailableStatus = Status(availableStatus)
	}

	return nil
}

func (s *ScraperStandardRegexp) Scrape() (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	body, _, err = s.ScrapeEndpoint.FetchCached(s.Name())
	if err != nil {
		return
	}

	if s.AvailablePattern != nil && s.AvailablePattern.Match(body) {
		if s.LimitedThreshold > 0 && s.NumApptsPattern != nil {
			totalAppointments := GetRegexCount(s.Name(), s.NumApptsPattern, body)

			if s.NumApptsTakenPattern != nil {
				totalAppointments -= GetRegexCount(s.Name(), s.NumApptsTakenPattern, body)
			}

			Log.Debugf("%s: total appointments: %d", s.Name(), totalAppointments)

			if totalAppointments <= s.LimitedThreshold {
				status = StatusLimited
			} else {
				status = s.AvailableStatus
			}
		} else {
			status = s.AvailableStatus
		}
	} else if s.UnavailablePattern.Match(body) {
		status = StatusNo
	} else if s.ErrorPattern != nil && s.ErrorPattern.Match(body) {
		status = StatusUnknown
		err = fmt.Errorf("Error pattern matched")
	} else {
		status = StatusPossible
	}

	return
}
