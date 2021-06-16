package csg

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ProxyTestUrls = []string{"https://ipv4.icanhazip.com", "https://api.ipify.org"}

const ProxyBlackListDuration = 600

/**
 * interfaces
 */
type ProxyEndpoint interface {
	GetUrl() *url.URL
	GetHttpClient() *http.Client
	BlackList()
	IsBlackListed() bool
}

type ProxyProvider interface {
	GetProxy() (ProxyEndpoint, error) // returns endpoint for a proxy
}

/**
 * HttpProxyEndpoint
 */
type HttpProxyEndpoint struct {
	url          *url.URL
	sourceString string
	blackListed  bool
	lastUsed     time.Time
}

func NewHttpProxyEndpoint(ip string, port uint16, sourceString string) (*HttpProxyEndpoint, error) {
	proxyEndpoint := new(HttpProxyEndpoint)
	proxyEndpoint.sourceString = sourceString
	proxyEndpoint.blackListed = false
	proxyEndpoint.lastUsed = time.Unix(0, 0)

	urlString := fmt.Sprintf("http://%s:%d", ip, port)
	var err error
	proxyEndpoint.url, err = new(url.URL).Parse(urlString)
	if err != nil {
		return nil, err
	}

	return proxyEndpoint, nil
}

func (hpe *HttpProxyEndpoint) BlackList() {
	hpe.blackListed = true
}

func (hpe *HttpProxyEndpoint) IsBlackListed() bool {
	return hpe.blackListed
}

func (hpe *HttpProxyEndpoint) GetUrl() *url.URL {
	return hpe.url
}

func (hpe *HttpProxyEndpoint) GetHttpClient() *http.Client {
	transport := &http.Transport{}
	transport.Proxy = http.ProxyURL(hpe.url)
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	client := new(http.Client)
	client.Transport = transport
	client.Timeout = 15 * time.Second

	return client
}

/**
 * AuthHttpProxyEndpoint
 */
type AuthHttpProxyEndpoint struct {
	url              *url.URL
	username         string
	password         string
	sourceString     string
	blackListedUntil time.Time
	blackListUrl     string
	expiry           time.Time
	lastUsed         time.Time
}

func NewAuthHttpProxyEndpoint(host string, port uint16, username string, password string, sourceString string, https bool, blackListUrl string) *AuthHttpProxyEndpoint {
	proxyEndpoint := new(AuthHttpProxyEndpoint)
	proxyEndpoint.sourceString = sourceString
	proxyEndpoint.username = username
	proxyEndpoint.password = password
	proxyEndpoint.blackListedUntil = time.Unix(0, 0)
	proxyEndpoint.blackListUrl = blackListUrl
	proxyEndpoint.expiry = time.Unix(0, 0)
	proxyEndpoint.lastUsed = time.Unix(0, 0)

	scheme := "http"
	if https {
		scheme = "https"
	}

	//urlString := fmt.Sprintf("http://%s:%d", host, port)
	//var err error
	proxyEndpoint.url = &url.URL{
		Scheme: scheme,
		User:   url.UserPassword(username, password),
		Host:   fmt.Sprintf("%s:%d", host, port),
	}

	return proxyEndpoint
}

func (ahpe *AuthHttpProxyEndpoint) BlackList() {
	if len(ahpe.blackListUrl) > 0 {
		Log.Debugf("Rotating out proxy: %s", censorUrl(ahpe.GetUrl()))
		resp, err := ahpe.GetHttpClient().Get(ahpe.blackListUrl)
		if err != nil {
			Log.Warnf("%v", err)
		} else {
			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				Log.Warnf("%v", err)
			} else {
				Log.Debugf(strings.ReplaceAll(string(body), "\n", ""))
			}
		}
	}

	if !ahpe.IsBlackListed() {
		ahpe.blackListedUntil = time.Now().Add(ProxyBlackListDuration * time.Second)
	}
}

func (ahpe *AuthHttpProxyEndpoint) IsBlackListed() bool {
	return ahpe.blackListedUntil.After(time.Now())
}

func (ahpe *AuthHttpProxyEndpoint) GetUrl() *url.URL {
	return ahpe.url
}

func (ahpe *AuthHttpProxyEndpoint) GetHttpClient() *http.Client {
	transport := &http.Transport{}
	transport.Proxy = http.ProxyURL(ahpe.url)
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	client := new(http.Client)
	client.Transport = transport
	client.Timeout = 15 * time.Second

	return client
}

/**
 * PublicHttpProxyProvider
 */
const PhppSourceUrl = "https://api.proxyscrape.com/v2/?request=getproxies&protocol=http&timeout=2500&country=all&ssl=yes&anonymity=all&simplified=true"
const PhppDefaultUpdateInterval = 60
const PhppDefaultRepeatInterval = 900

type PublicHttpProxyProvider struct {
	proxyList            []*HttpProxyEndpoint
	repeatInterval       time.Duration
	updateInterval       time.Duration
	lastUpdateFromSource time.Time
	mutex                *sync.Mutex
}

func NewPublicHttpProxyProviderDefaults() (ProxyProvider, error) {
	return NewPublicHttpProxyProvider(PhppDefaultUpdateInterval, PhppDefaultRepeatInterval)
}

// updateInterval: how often to update from source, in seconds
// repeatInterval: minimum number of seconds before returning the same proxy again
func NewPublicHttpProxyProvider(updateInterval int, repeatInterval int) (ProxyProvider, error) {
	phpp := new(PublicHttpProxyProvider)
	phpp.proxyList = make([]*HttpProxyEndpoint, 0)
	phpp.updateInterval = time.Duration(updateInterval) * time.Second
	phpp.repeatInterval = time.Duration(repeatInterval) * time.Second
	phpp.mutex = &sync.Mutex{}

	if err := phpp.updateProxiesFromSource(); err != nil {
		return nil, err
	}

	SeedRand()

	return phpp, nil
}

func (phpp *PublicHttpProxyProvider) GetProxy() (ProxyEndpoint, error) {
	phpp.mutex.Lock()
	defer phpp.mutex.Unlock()

	if phpp.lastUpdateFromSource.Add(phpp.updateInterval).Before(time.Now()) {
		if err := phpp.updateProxiesFromSource(); err != nil {
			return nil, err
		}
	}

	rand.Shuffle(len(phpp.proxyList), func(i, j int) { phpp.proxyList[i], phpp.proxyList[j] = phpp.proxyList[j], phpp.proxyList[i] })

	Log.Debugf("Finding a working proxy...")

	for idx, proxyEndpoint := range phpp.proxyList {
		if proxyEndpoint.IsBlackListed() {
			continue
		}

		if proxyEndpoint.lastUsed.After(time.Now().Add(-phpp.repeatInterval)) {
			continue
		}

		if testProxy(proxyEndpoint) {
			proxyEndpoint.lastUsed = time.Now()
			Log.Debugf("Found working proxy after %d/%d attempt(s): %s", idx+1, len(phpp.proxyList), proxyEndpoint.sourceString)
			return proxyEndpoint, nil
		} else {
			proxyEndpoint.BlackList()
		}
	}

	return nil, fmt.Errorf("PublicHttpProxyProvider: could not find a valid proxy!")
}

func (phpp *PublicHttpProxyProvider) updateProxiesFromSource() error {
	resp, err := http.Get(PhppSourceUrl)
	if err != nil {
		Log.Errorf("%+v", err)
		return err
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Log.Errorf("%+v", err)
		return err
	}

	ipPattern := regexp.MustCompile(`^([\d]{1,3}\.){3}[\d]{1,3}$`)

	newProxyListStr := strings.Split(string(bytes), "\r\n")
	newProxyList := make([]*HttpProxyEndpoint, 0)

	for _, proxyStr := range newProxyListStr {
		proxyStrParts := strings.Split(proxyStr, ":")
		if len(proxyStrParts) != 2 {
			continue
		}

		ip := proxyStrParts[0]
		if !ipPattern.Match([]byte(ip)) {
			Log.Warnf("invalid ip: %s did not match %v", proxyStrParts[0], ipPattern)
			continue
		}
		port, err := strconv.Atoi(proxyStrParts[1])
		if err != nil {
			Log.Warnf("invalid port: %s: %+v", proxyStrParts[1], err)
			continue
		}

		if port < 1 || port > 65535 {
			Log.Warnf("invalid port: %d", port)
			continue
		}

		var proxyEndpoint *HttpProxyEndpoint

		//get flags from old list
		//there's a more efficient way to do this but at this size it doesn't matter
		for _, oldEndpoint := range phpp.proxyList {
			if oldEndpoint.sourceString == proxyStr {
				proxyEndpoint = oldEndpoint
				break
			}
		}

		if proxyEndpoint == nil {
			proxyEndpoint, err = NewHttpProxyEndpoint(ip, uint16(port), proxyStr)
			if err != nil {
				Log.Warnf("%+v", err)
				continue
			}
		}

		newProxyList = append(newProxyList, proxyEndpoint)
	}

	if len(newProxyList) < 1 {
		return fmt.Errorf("PublicHttpProxyProvider: Proxy source did not return any proxies: %s", PhppSourceUrl)
	}

	phpp.proxyList = newProxyList
	phpp.lastUpdateFromSource = time.Now()

	Log.Infof("Found %d proxies", len(newProxyList))

	return nil
}

func censorUrl(url *url.URL) string {
	parts := strings.Split(url.String(), "@")
	if len(parts) == 1 {
		return url.String() //no auth, just return the plain url
	}

	if len(parts) != 2 {
		return "<malformed>"
	}

	partsAuth := strings.Split(parts[0], ":")

	if len(parts) != 2 {
		return "<malformed>"
	}

	return fmt.Sprintf("%s:<snip>:%s", partsAuth[0], parts[1])
}

func testProxy(proxy ProxyEndpoint) bool {
	passed := make(chan bool)

	for _, proxyTestUrl := range ProxyTestUrls {
		go testProxyAsync(proxy, proxyTestUrl, passed)
	}

	for range ProxyTestUrls {
		if <-passed {
			return true
		}
	}

	return false
}

func testProxyAsync(proxy ProxyEndpoint, testUrl string, passed chan bool) {
	httpClient := proxy.GetHttpClient()
	httpClient.Timeout = 2 * time.Second

	resp, err := httpClient.Get(testUrl)
	if err != nil {
		Log.Warnf("Proxy failed test: %s: %v", censorUrl(proxy.GetUrl()), err)
		passed <- false
	} else {
		defer resp.Body.Close()
		passed <- true
	}
}
