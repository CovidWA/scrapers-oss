package csg

import (
	"crypto/sha256"
	"encoding/hex"
)

const ScraperTypeStandardHash = "standard_hash"
const ParamKeyUnavailableHash = "unavailable_hash"
const ParamKeyAvailableHash = "available_hash"

type ScraperStandardHash struct {
	ScraperName     string
	ScrapeEndpoint  *Endpoint
	UnavailableHash string
	AvailableHash   string
}

type ScraperStandardHashFactory struct {
}

func (sf *ScraperStandardHashFactory) Type() string {
	return ScraperTypeStandardHash
}

func (sf *ScraperStandardHashFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	scraper := new(ScraperStandardHash)
	scraper.ScraperName = name

	scrapers := map[string]Scraper{name: scraper}
	return scrapers, nil
}

func (s *ScraperStandardHash) Type() string {
	return ScraperTypeStandardHash
}

func (s *ScraperStandardHash) Name() string {
	return s.ScraperName
}

func (s *ScraperStandardHash) Configure(params map[string]interface{}) error {
	var err error

	s.ScrapeEndpoint, err = getEndpointRequired(params, ParamKeyEndpoint)
	if err != nil {
		return err
	}

	s.UnavailableHash, err = getStringRequired(params, ParamKeyUnavailableHash)
	if err != nil {
		return err
	}

	s.AvailableHash, _ = getStringOptional(params, ParamKeyAvailableHash)
	return nil
}

func (s *ScraperStandardHash) Scrape() (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	body, _, err = s.ScrapeEndpoint.FetchCached(s.Name())

	if err != nil {
		return
	}

	hash := sha256.Sum256(body)
	hashString := hex.EncodeToString(hash[:])

	Log.Infof("%s: Hashed: %s", s.Name(), hashString)

	if len(s.AvailableHash) > 0 && hashString == s.AvailableHash {
		status = StatusYes
	} else if hashString == s.UnavailableHash {
		status = StatusNo
	} else {
		status = StatusPossible
	}

	return
}
