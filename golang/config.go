package csg

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"os"
	"regexp"
	"strings"
)

const DefaultConfigPath = "./covidwa-scrapers.yaml"
const APISecretEnvName = "API_SECRETWA"
const APIHostEnvName = "API_HOSTWA"
const APISecretAWSName = "secret"

var HostPattern = regexp.MustCompile(`(?i)https?://([^/]+)`)

type Config struct {
	Debug                 bool                     `yaml:"debug"`
	TestMode              bool                     `yaml:"test_mode"`
	PollInterval          int64                    `yaml:"poll_interval"`
	ApiInterval           int64                    `yaml:"api_interval"`
	ApiSecret             string                   `yaml:"api_secret"`
	ApiUrl                string                   `yaml:"api_url"`
	ApiInternalUrl        string                   `yaml:"api_internal_url"`
	FromEmailAddress      string                   `yaml:"from_email_address"`
	SmtpUsername          string                   `yaml:"smtp_user"`
	SmtpPassword          string                   `yaml:"smtp_pass"`
	SmtpHost              string                   `yaml:"smtp_host"`
	SmtpPort              int                      `yaml:"smtp_port"`
	ErrorWarningThreshold int                      `yaml:"error_warning_threshold"`
	NotifyEmailAddrs      []string                 `yaml:"notify_email_addrs"`
	DumpDir               string                   `yaml:"dump_dir"`
	LimitedThreshold      int                      `yaml:"limited_threshold"`
	ScraperConfigs        map[string]ScraperConfig `yaml:"scraper_configs"`
	NotifyOnChange        bool                     `yaml:"notify_on_change"`
	NotifyOnError         bool                     `yaml:"notify_on_error"`
	DumpOutput            bool                     `yaml:"dump_output"`
	DumpOutputS3          bool                     `yaml:"dump_output_s3"`
}

type ScraperConfig struct {
	Type               string                 `yaml:"type"`
	Params             map[string]interface{} `yaml:"params"`
	ApiKey             string                 `yaml:"api_key"`
	AllowedStatusCodes []int                  `yaml:"allowed_status_codes"`
	MinInterval        int64                  `yaml:"min_scrape_interval"`
}

func NewConfigDefaultPath() (*Config, error) {
	return NewConfig(DefaultConfigPath)
}

func NewConfig(configPath string) (*Config, error) {
	// Create config structure
	config := &Config{}

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	if config.Debug {
		Log.SetLevel("debug")
	}

	//replace host portion of url, usually for testing
	hostOverride := os.Getenv(APIHostEnvName)
	if len(hostOverride) > 0 {
		newApiUrl, err := ReplaceHost(config.ApiUrl, hostOverride)
		if err != nil {
			return nil, err
		}
		config.ApiUrl = newApiUrl

		newApiInternalUrl, err := ReplaceHost(config.ApiInternalUrl, hostOverride)
		if err != nil {
			return nil, err
		}
		config.ApiInternalUrl = newApiInternalUrl
	}

	Log.Debugf("API Update URL: %s", config.ApiUrl)
	Log.Debugf("API Get Clinics URL: %s", config.ApiInternalUrl)

	if len(config.ApiSecret) == 0 {
		config.ApiSecret = os.Getenv(APISecretEnvName)
		notFound := ""
		if len(config.ApiSecret) == 0 {
			notFound = "NOT "
		}
		Log.Debugf("API secret %sfound in environment variable %s", notFound, APISecretEnvName)
	} else {
		Log.Debugf("API secret found in %s", configPath)
	}

	if len(config.ApiSecret) == 0 {
		config.ApiSecret, err = GetAWSEncryptedParameter(APISecretAWSName)
		if err != nil {
			Log.Errorf("Could not get api secret from AWS: %v", err)
		}

		notFound := ""
		if len(config.ApiSecret) == 0 {
			notFound = "NOT "
		}
		Log.Debugf("API secret %sfound in AWS parameter '%s'", notFound, APISecretAWSName)
	}

	if len(config.ApiSecret) == 0 {
		return nil, fmt.Errorf("Could not find api secret in any of these places: %s, $%s, or AWS parameter '%s'", configPath, APISecretEnvName, APISecretAWSName)
	}

	return config, nil
}

func ReplaceHost(originalUrl string, host string) (string, error) {
	matches := HostPattern.FindStringSubmatch(originalUrl)
	if len(matches) < 2 {
		return "", fmt.Errorf("Could not parse host from url: %s", originalUrl)
	}

	originalHost := matches[1]
	newUrl := strings.Replace(originalUrl, originalHost, host, 1)

	return newUrl, nil
}
