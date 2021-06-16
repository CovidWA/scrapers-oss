package csg

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const DefaultSubject = "COVID WA - Notification"
const SinglePassRetries = 3

var config *Config

func Run(args []string) {
	var err error

	config, err = NewConfigDefaultPath()
	if err != nil {
		Log.Errorf("Can't read config: %v", err)
		panic(err)
	}

	if args[0] == "covidwa-scrapers-go-lambda" {
		//hack to always disable file output on lambda
		config.DumpOutput = false
	}

	if _, err = os.Stat(config.DumpDir); config.DumpOutput && err != nil {
		err := os.Mkdir(config.DumpDir, 0755)
		if err != nil {
			Log.Errorf("Can't create dump dir: %s", config.DumpDir)
			panic(err)
		}
	}

	if config.PollInterval < 10 || config.PollInterval > 86400 {
		panic(fmt.Errorf("Poll interval must be between 10 and 86400 seconds, configured: %d", config.PollInterval))
	}

	scraperFactories := GetScraperFactories()

	scrapeContexts := make([]*ScrapeAndSendContext, 0)
	scraperNames := make([]string, 0)

	for configName, scraperConfig := range config.ScraperConfigs {
		factory, exists := scraperFactories[scraperConfig.Type]
		if !exists {
			panic(fmt.Sprintf("Unknown scraper type: %s", scraperConfig.Type))
		}

		scrapers, err := factory.CreateScrapers(configName)
		if err != nil {
			panic(fmt.Sprintf("%s: %v", configName, err))
		}

		for _, scraper := range scrapers {
			if err = scraper.Configure(scraperConfig.Params); err != nil {
				panic(fmt.Sprintf("%s: %v", scraper.Name(), err))
			}

			//make copy of parsed config so we don't clobber each other
			config := scraperConfig
			config.ApiKey = strings.ReplaceAll(config.ApiKey, "##NAME##", scraper.Name())

			Log.Infof("Registering scraper: %s - type: %s, key: %s", scraper.Name(), config.Type, config.ApiKey)

			newContext := NewScrapeAndSendContext(scraper, &config)
			scrapeContexts = append(scrapeContexts, newContext)
			scraperNames = append(scraperNames, scraper.Name())
		}
	}

	changeTracker := NewChangeTracker(scraperNames)

	if len(args) > 1 {
		switch args[1] {
		case "once":
			for retryCount := 0; len(scrapeContexts) > 0 && retryCount <= SinglePassRetries; retryCount++ {
				scraperCount := len(scrapeContexts)
				if retryCount == 0 {
					Log.Infof("Running %d scraper(s) once...", scraperCount)
				} else {
					Cache.Destroy() //clear out any cached data

					// don't retry too fast
					time.Sleep(2 * time.Second)
					Log.Infof("Retrying %d failed scraper(s) (%d/%d)...", scraperCount, retryCount, SinglePassRetries)
				}

				resultChan := make(chan *ScrapeAndSendContext)

				for _, ctx := range scrapeContexts {
					//run all scrapers in parallel
					go doScrapeAndSend(changeTracker, ctx, true, resultChan)
				}

				newScrapeContexts := make([]*ScrapeAndSendContext, 0)

				for doneCount := 0; doneCount < scraperCount; doneCount++ {
					ctx := <-resultChan

					// build new list of failed scrapers
					if ctx.Status == StatusUnknown {
						newScrapeContexts = append(newScrapeContexts, ctx)
					}

					scrapersLeft := scraperCount - doneCount - 1
					Log.Infof("Scraper '%s' finished with status %s, waiting on %d more...", ctx.Name, ctx.Status, scrapersLeft)
					if scrapersLeft == 3 {
						//identify any long-running scrapers

						for _, straggler := range scrapeContexts {
							if len(straggler.Status) == 0 {
								Log.Debugf("Possible long-running scraper: '%s'", straggler.Name)
							}
						}
					}
				}

				// retry
				scrapeContexts = newScrapeContexts
			}

			Cache.Destroy() //clear out any crud left in the cache
		case "test":
			if len(args) > 2 {
				patternStr := fmt.Sprintf("^%s$", args[2])
				if strings.Contains(patternStr, "*") {
					patternStr = strings.ReplaceAll(patternStr, "*", ".*")
				}

				pattern := regexp.MustCompile(patternStr)
				Log.Debugf("Testing all scrapers with names matching %v", pattern)
				scraperCount := 0
				errorCount := 0
				resultChan := make(chan *ScrapeAndSendContext)

				for _, ctx := range scrapeContexts {
					if pattern.MatchString(ctx.Name) {
						go doScrapeAndSend(changeTracker, ctx, true, resultChan)
						scraperCount++
					}
				}

				for doneCount := 0; doneCount < scraperCount; doneCount++ {
					ctx := <-resultChan
					Log.Infof("Scraper %s returned a status of %s", ctx.Name, ctx.Status)

					if ctx.Status == StatusUnknown || ctx.Status == StatusApifail {
						errorCount++
					}
				}

				if scraperCount == 0 {
					Log.Warnf("Scraper not found: %s", args[2])
					os.Exit(2)
				} else if errorCount > 0 {
					os.Exit(2)
				} else {
					os.Exit(0)
				}
			}
			fallthrough
		default:
			printUsageAndExit(args)
		}
	} else {
		Log.Infof("Running %d scrapers continuously...", len(scrapeContexts))

		for {
			for _, ctx := range scrapeContexts {
				go doScrapeAndSend(changeTracker, ctx, false, nil)
			}
			time.Sleep(time.Duration(1) * time.Second)
		}
	}
}

func GetScraperFactories() map[string]ScraperFactory {
	var factory ScraperFactory

	scraperFactories := make(map[string]ScraperFactory)

	//generic scrapers
	factory = new(ScraperStandardHashFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperStandardHeaderFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperStandardRegexpFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperMultistageRegexpFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperSwitchFactory)
	scraperFactories[factory.Type()] = factory

	//booking software specific scrapers
	factory = new(ScraperAthenaFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperCognitoFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperJotformFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperMsOutlookFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperPrepmodFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperSigneticFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperSimplyBookFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperSolvHealthFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperWpSsaFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperZohoFactory)
	scraperFactories[factory.Type()] = factory

	//chain/api scrapers
	factory = new(ScraperCvsFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperDOHFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperKrogerFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperVaccineSpotterFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperWalgreensFactory)
	scraperFactories[factory.Type()] = factory
	factory = new(ScraperWalmartFactory)
	scraperFactories[factory.Type()] = factory

	return scraperFactories
}

type ScrapeAndSendContext struct {
	Name    string
	Scraper Scraper
	Config  *ScraperConfig
	Status  Status
	Tags    []string
}

func NewScrapeAndSendContext(scraper Scraper, scraperConfig *ScraperConfig) *ScrapeAndSendContext {
	ctx := new(ScrapeAndSendContext)
	ctx.Name = scraper.Name()
	ctx.Scraper = scraper
	ctx.Config = scraperConfig

	return ctx
}

//forceScrape: ignore any interval checks and just scrape immediately
func doScrapeAndSend(tracker *ChangeTracker, ctx *ScrapeAndSendContext, forceScrape bool, resultChan chan *ScrapeAndSendContext) {
	minInterval := config.PollInterval
	if ctx.Config.MinInterval > 0 {
		minInterval = ctx.Config.MinInterval
	}

	lastScrapeTime := tracker.LastScrape(ctx.Name)
	currentTime := time.Now().Unix()
	if !forceScrape && currentTime-lastScrapeTime < minInterval {
		//fprintlnDebug("%s: under minimum interval (%d < %d), skipping", scraper.Name(), currentTime - lastScrapeTime, minInterval)
		if resultChan != nil {
			resultChan <- ctx
		}
		return
	}

	if !tracker.Lock(ctx.Name) {
		if resultChan != nil {
			resultChan <- ctx
		}
		return
	}

	status, tags, body, err := ctx.Scraper.Scrape()
	ctx.Status = status
	ctx.Tags = tags.ToStringArray()

	if err != nil {
		Log.Errorf("%s: %v", ctx.Name, err)
		errorCount := tracker.Error(ctx.Name, err)

		if errorCount == config.ErrorWarningThreshold && config.NotifyOnError {
			if err := notifyError(ctx.Name, err); err != nil {
				Log.Errorf("%+v", err)
			}
		}
	} else if ctx.Status == StatusUnknown {
		panic("Sanity check failed: Unknown status with nil error")
	}

	var contentUrl string = ""

	if body != nil {
		hash := sha256.Sum256(body)
		hashString := hex.EncodeToString(hash[:])

		if (status == StatusPossible || status == StatusUnknown) && config.DumpOutput {
			contentUrl = dumpOutput(ctx.Name, hashString, body)
		}
	}

	apiSend, changed := tracker.UpdateAndUnlock(ctx.Name, ctx.Status)

	if changed && config.NotifyOnChange {
		if err := notifyChange(ctx.Name, ctx.Status); err != nil {
			Log.Errorf("%+v", err)
		}
	}

	if apiSend {
		sent := false
		for retries := 0; retries < config.ErrorWarningThreshold; retries++ {
			sent = doApiSend(ctx.Name, ctx.Config.ApiKey, ctx.Status, ctx.Tags, contentUrl)
			if sent {
				break
			}
			time.Sleep(time.Duration(5) * time.Second)
		}

		if !sent {
			nerr := fmt.Errorf("Error(s) while sending updates to covidwa API")
			if err := notifyError(ctx.Name, nerr); err != nil {
				Log.Errorf("%+v", err)
			}
			ctx.Status = StatusApifail
			if resultChan != nil {
				resultChan <- ctx
			}
			return
		}
	}

	if resultChan != nil {
		resultChan <- ctx
	}
}

func doApiSend(name string, key string, status Status, tags []string, contentUrl string) bool {
	statusStr := string(status)

	if len(key) == 0 || config.TestMode || status == StatusApiSkip {
		Log.Debugf("(silent) name: %s, key: %s, status: %s, tags: %v", name, key, statusStr, tags)
		return true
	}

	client := &http.Client{}
	tagStr := strings.Join(tags, `","`)
	if len(tagStr) > 0 {
		tagStr = fmt.Sprintf(`"%s"`, tagStr)
	}

	var data string
	if len(contentUrl) > 0 {
		data = fmt.Sprintf(`{"key": "%s", "status":"%s","secret":"%s","content_url":"%s","scraperTags":[%s]}`, key, statusStr, config.ApiSecret, contentUrl, tagStr)
	} else {
		data = fmt.Sprintf(`{"key": "%s", "status":"%s","secret":"%s","scraperTags":[%s]}`, key, statusStr, config.ApiSecret, tagStr)
	}

	req, _ := http.NewRequest("POST", config.ApiUrl, strings.NewReader(data))
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)

	if err != nil {
		Log.Errorf("%+v", err)
		return false
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Errorf("%+v", err)
		return false
	}

	Log.Debug(fmt.Sprintf("%s: %s", strings.ReplaceAll(data, config.ApiSecret, "<snip>"), string(bytes)))

	if resp.StatusCode != 200 {
		Log.Errorf("%s: API Status code is %d!", name, resp.StatusCode)
		return false
	}

	return true
}

func dumpOutput(name string, hash string, body []byte) (url string) {
	if len(hash) == 0 {
		hashBytes := sha256.Sum256(body)
		hash = hex.EncodeToString(hashBytes[:])
	}

	fileName := fmt.Sprintf("%s.%s.out", name, hash)
	url = ""
	var err error

	if config.DumpOutputS3 {
		if HasAWSCredentials() {
			url, err = PutS3Object(S3ScraperOutputBucket, fileName, body)
			if err != nil {
				Log.Warnf("%v", err)
			} else {
				Log.Debugf("Sent %d bytes to S3: %s", len(body), url)
			}
		} else {
			Log.Warnf("Scraper configured to send to S3 but no AWS credentials were found")
		}
	}

	if config.DumpOutput {
		filePath := filepath.Join(config.DumpDir, fileName)

		if _, err := os.Stat(filePath); err == nil {
			//fprintlnDebug("%s already exists, skipping", filePath)
			return url
		}

		err = ioutil.WriteFile(filePath, body, 0644)
		if err != nil {
			Log.Warnf("%v", err)
		}

		Log.Debugf("Wrote %d bytes to file: %s", len(body), filePath)
	}

	return url
}

func notifyError(name string, err error) error {
	subject := DefaultSubject
	body := fmt.Sprintf("Error during scrape: %s: %v", name, err)

	return sendEmail(subject, body)
}

func notifyChange(name string, status Status) error {
	subject := DefaultSubject
	body := fmt.Sprintf("Detected change: %s, new status: %v", name, status)

	return sendEmail(subject, body)
}

func sendEmail(subject string, body string) error {
	if len(config.SmtpHost) == 0 {
		return nil
	}

	Log.Infof("Subject: %s", subject)
	Log.Infof("Body: %s", body)

	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	sb.WriteString("\r\n")
	sb.WriteString(body)
	fmt.Println(sb.String())

	auth := smtp.PlainAuth("", config.SmtpUsername, config.SmtpPassword, config.SmtpHost)

	err := smtp.SendMail(fmt.Sprintf("%s:%d", config.SmtpHost, config.SmtpPort), auth, config.FromEmailAddress, config.NotifyEmailAddrs, []byte(sb.String()))

	if err != nil {
		Log.Errorf("sendEmail: %+v", err)
	}

	return err
}

func printUsageAndExit(args []string) {
	exeName := filepath.Base(args[0])
	fmt.Printf("Usage: %s once | test <scraper_name>\n", exeName)
	os.Exit(0)
}
