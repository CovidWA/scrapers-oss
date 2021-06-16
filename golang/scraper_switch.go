package csg

import (
	"fmt"
	"regexp"
	"strings"
)

const ScraperTypeSwitch = "switch"

const ParamKeySwitchType = "switch_type"
const ParamKeySwitchDefaultStatus = "default_status"
const ParamKeySwitchList = "list"
const ParamKeySwitchPattern = "pattern"
const ParamKeySwitchUrl = "url"
const ParamKeySwitchParams = "params"
const ParamKeySwitchAutoUrl = "auto_url"
const ParamKeySwitchCrawlDepth = "crawl_depth"
const ParamKeySwitchCrawlExternal = "crawl_external"
const ParamKeySwitchCrawlIgnore = "crawl_ignore"

var ProtocolPattern = regexp.MustCompile(`^https?://`)
var SwitchCrawlUrlPattern = regexp.MustCompile(`(?i)<a href=["']([^"']+)["']`)

type ScraperSwitch struct {
	ScraperName   string
	DefaultStatus Status
	Url           string
	List          []ScraperSwitchItem
	CrawlDepth    int
	CrawlExternal bool
	CrawlIgnore   *regexp.Regexp
}

type ScraperSwitchItem struct {
	Pattern *regexp.Regexp
	Scraper Scraper
	AutoUrl bool
}

type ScraperSwitchFactory struct {
}

func (sf *ScraperSwitchFactory) Type() string {
	return ScraperTypeSwitch
}

func (sf *ScraperSwitchFactory) CreateScrapers(name string) (map[string]Scraper, error) {
	scraper := new(ScraperSwitch)
	scraper.ScraperName = name

	scrapers := map[string]Scraper{name: scraper}
	return scrapers, nil
}

func (s *ScraperSwitch) Type() string {
	return ScraperTypeSwitch
}

func (s *ScraperSwitch) Name() string {
	return s.ScraperName
}

var switchScraperFactories = GetScraperFactories()

func (s *ScraperSwitch) Configure(params map[string]interface{}) error {
	items, err := getMapArrayRequired(params, ParamKeySwitchList)
	if err != nil {
		return err
	}

	defaultStatus, err := getStringRequired(params, ParamKeySwitchDefaultStatus)
	if err != nil {
		return err
	}

	s.DefaultStatus = Status(defaultStatus)

	s.Url, err = getStringRequired(params, ParamKeySwitchUrl)
	if err != nil {
		return err
	}

	s.CrawlDepth, _ = getIntOptionalWithDefault(params, ParamKeySwitchCrawlDepth, 0)
	s.CrawlExternal = getBool(params, ParamKeySwitchCrawlExternal)
	s.CrawlIgnore = getPatternOptional(params, ParamKeySwitchCrawlIgnore)

	s.List = make([]ScraperSwitchItem, len(items))

	for i, v := range items {
		s.List[i] = ScraperSwitchItem{
			Pattern: nil,
			Scraper: nil,
		}

		s.List[i].Pattern, err = getPatternRequired(v, ParamKeySwitchPattern)
		if err != nil {
			return err
		}

		s.List[i].AutoUrl = getBool(v, ParamKeySwitchAutoUrl)

		scraperType, err := getStringRequired(v, ParamKeySwitchType)
		if err != nil {
			return err
		}

		factory, exists := switchScraperFactories[scraperType]
		if !exists {
			return fmt.Errorf("Unknown scraper type: %s", scraperType)
		}

		scrapers, err := factory.CreateScrapers(s.Name())
		if err != nil {
			return err
		}

		if len(scrapers) != 1 {
			return fmt.Errorf("ScraperSwitch (%s): Only factories that create single scrapers are supported", s.Name())
		}

		scraperParams := getMapOptional(v, ParamKeySwitchParams)

		for _, scraper := range scrapers {
			if scraperParams != nil {
				if err = scraper.Configure(scraperParams); err != nil {
					return err
				}
			}

			s.List[i].Scraper = scraper
			Log.Debugf("ScraperSwitch (%s): Created scraper of type %s", s.Name(), scraper.Type())
		}
	}

	return nil
}

func (s *ScraperSwitch) Scrape() (status Status, tags TagSet, body []byte, err error) {

	status, tags, body, err = s.ScrapeRecursive(s.Url, 0, nil)
	if err != nil {
		return
	}

	if status == StatusUnknown {
		Log.Warnf("%s: No patterns matched, returning default status", s.Name())
		status = s.DefaultStatus
	}

	return
}

func (s *ScraperSwitch) ScrapeRecursive(url string, depth int, crawled map[string]bool) (status Status, tags TagSet, body []byte, err error) {
	endpoint := new(Endpoint)
	endpoint.Method = "GET"
	endpoint.Url = url

	if depth == 0 {
		crawled = make(map[string]bool)
	} else {
		endpoint.AllowedStatusCodes = []int{404}
	}

	crawled[url] = true

	body, _, err = endpoint.FetchCached(s.Name())
	if err != nil {
		return
	}

	hasLimitedAvail := false

	for idx, item := range s.List {
		if item.Pattern.Match(body) {
			Log.Debugf("%s: Matched pattern on switch index %d (%s)", s.Name(), idx, item.Scraper.Type())

			var scrapedTags TagSet

			if item.AutoUrl {
				if urlScraper, ok := item.Scraper.(UrlScraper); ok {
					status, scrapedTags, body, err = urlScraper.ScrapeUrls(url)
				} else {
					Log.Errorf("%s: Scraper type %s does not support auto_url", s.Name(), item.Scraper.Type())
					continue
				}
			} else {
				status, scrapedTags, body, err = item.Scraper.Scrape()
			}

			tags = tags.Merge(scrapedTags)

			if err != nil {
				Log.Errorf("%s: %v", s.Name(), err)
				dumpOutput(s.Name(), "", body)
				err = nil
			}

			Log.Debugf("%s: Scraper type %s returned %s", s.Name(), item.Scraper.Type(), status)

			if status == StatusYes {
				return
			} else if status == StatusLimited {
				hasLimitedAvail = true
			}
		}
	}

	if depth < s.CrawlDepth {
		var urls map[string]bool
		urls, err = s.GetCrawlUrls(url, body)
		if err != nil {
			return
		}

		for nextUrl, crawl := range urls {
			if crawled[nextUrl] {
				continue
			}
			if crawl {
				Log.Debugf("%s: Crawl (%d): %s", s.Name(), depth, nextUrl)
				var scrapedTags TagSet
				status, scrapedTags, body, err = s.ScrapeRecursive(nextUrl, depth+1, crawled)
				tags = tags.Merge(scrapedTags)

				if status == StatusYes {
					return
				} else if status == StatusLimited {
					hasLimitedAvail = true
				}
			} else {
				crawled[nextUrl] = true
				Log.Debugf("%s: Ignore (%d): %s", s.Name(), depth, nextUrl)
			}
		}
	}

	if hasLimitedAvail {
		status = StatusLimited
	} else {
		status = StatusUnknown
	}
	return
}

func (s *ScraperSwitch) GetCrawlUrls(url string, body []byte) (map[string]bool, error) {
	hostMatch := HostPattern.FindStringSubmatch(url)
	if len(hostMatch) != 2 {
		err := fmt.Errorf("Could not parse host from url: %s", url)
		return nil, err
	}
	host := hostMatch[1]
	protocol := ProtocolPattern.FindString(url)
	if len(protocol) == 0 {
		Log.Warnf("%s: Could not parse protocol from url: %s", s.Name(), url)
		protocol = "http://"
	}

	matches := SwitchCrawlUrlPattern.FindAllStringSubmatch(string(body), -1)
	urls := make(map[string]bool)
	for _, match := range matches {
		nextUrl := match[len(match)-1]
		if len(nextUrl) == 0 || nextUrl[0] == '#' {
			continue
		} else if strings.HasPrefix(nextUrl, "javascript") || strings.HasPrefix(nextUrl, "mailto") {
			continue
		} else if nextUrl[0] == '/' {
			nextUrl = fmt.Sprintf("%s%s%s", protocol, host, nextUrl)
		}

		urls[nextUrl] = true

		if !s.CrawlExternal {
			hostMatch := HostPattern.FindStringSubmatch(nextUrl)
			if len(hostMatch) != 2 {
				Log.Errorf("%s: Could not parse host from url: %s", s.Name(), nextUrl)
				urls[nextUrl] = false
				continue
			}
			nextHost := hostMatch[1]

			if !strings.Contains(host, nextHost) && !strings.Contains(nextHost, host) {
				urls[nextUrl] = false
				continue
			}
		}

		if url == nextUrl {
			urls[nextUrl] = false
		} else if s.CrawlIgnore != nil && s.CrawlIgnore.MatchString(nextUrl) {
			urls[nextUrl] = false
		}
	}

	return urls, nil
}
