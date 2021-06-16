package csg

const ScraperTypeStandardHeader = "standard_header"
const ParamKeyUnavailableHeaderName = "unavailable_header_name"
const ParamKeyUnavailableHeaderValue = "unavailable_header_value"

type ScraperStandardHeader struct {
	ScraperName            string
	ScrapeEndpoint         *Endpoint
	UnavailableHeaderName  string
	UnavailableHeaderValue string
}

type ScraperStandardHeaderFactory struct {
}

func (sf *ScraperStandardHeaderFactory) Type() string {
	return ScraperTypeStandardHeader
}

func (sf *ScraperStandardHeaderFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	scraper := new(ScraperStandardHeader)
	scraper.ScraperName = name

	scrapers := map[string]Scraper{name: scraper}
	return scrapers, nil
}

func (s *ScraperStandardHeader) Type() string {
	return ScraperTypeStandardHeader
}

func (s *ScraperStandardHeader) Name() string {
	return s.ScraperName
}

func (s *ScraperStandardHeader) Configure(params map[string]interface{}) error {
	var err error

	s.ScrapeEndpoint, err = getEndpointRequired(params, ParamKeyEndpoint)
	if err != nil {
		return err
	}

	s.UnavailableHeaderName, err = getStringRequired(params, ParamKeyUnavailableHeaderName)
	if err != nil {
		return err
	}

	s.UnavailableHeaderValue, _ = getStringRequired(params, ParamKeyUnavailableHeaderValue)

	return nil
}

func (s *ScraperStandardHeader) Scrape() (status Status, tags TagSet, body []byte, err error) {
	status = StatusUnknown

	body, headers, err := s.ScrapeEndpoint.Fetch(s.Name())

	if err != nil {
		return
	}

	if headerVals, exists := headers[s.UnavailableHeaderName]; exists {
		if len(s.UnavailableHeaderValue) > 0 {
			for _, headerVal := range headerVals {
				if headerVal == s.UnavailableHeaderValue {
					status = StatusNo
					return
				}
			}
		} else {
			status = StatusNo
			return
		}
	}

	status = StatusPossible
	return
}
