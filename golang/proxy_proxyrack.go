package csg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"
)

const ProxyRackUrlEnvName = "PROXYRACK_URL"
const ProxyRackUrlAWSParameterName = "proxyrack-url"
const ProxyRackDefaultRepeatInterval = 900
const ProxyRackRotationUrl = "http://api.proxyrack.net/release"

const ProxyRackActiveIdxTTL = 30

/**
 * ProxyRackAuthHttpProxyProvider
 */
type ProxyRackAuthHttpProxyProvider struct {
	proxyList       []*AuthHttpProxyEndpoint
	activeIdx       []int
	activeIdxExpiry time.Time
	repeatInterval  time.Duration
	portOffset      int
	mutex           *sync.Mutex
}

func NewProxyRackAuthHttpProxyProviderDefaults() (ProxyProvider, error) {
	return NewProxyRackAuthHttpProxyProvider(ProxyRackDefaultRepeatInterval, false)
}

// updateInterval: how often to update from source, in seconds
// repeatInterval: minimum number of seconds before returning the same proxy again
func NewProxyRackAuthHttpProxyProvider(repeatInterval int, seedRand bool) (ProxyProvider, error) {
	prpp := new(ProxyRackAuthHttpProxyProvider)
	prpp.proxyList = make([]*AuthHttpProxyEndpoint, 0)
	prpp.activeIdx = make([]int, 0)
	prpp.activeIdxExpiry = time.Unix(0, 0)
	prpp.repeatInterval = time.Duration(repeatInterval) * time.Second
	prpp.mutex = &sync.Mutex{}

	proxyUrlStr := os.Getenv(ProxyRackUrlEnvName)

	if len(proxyUrlStr) == 0 && HasAWSCredentials() {
		var err error
		proxyUrlStr, err = GetAWSEncryptedParameter(ProxyRackUrlAWSParameterName)
		if err != nil {
			Log.Warnf("%v", err)
		}
	}

	if len(proxyUrlStr) == 0 {
		return nil, fmt.Errorf("No proxy list: Either AWS credentials missing or '%s' is not configured in AWS parameter store, and the '%s' environment variable is missing", ProxyRackUrlAWSParameterName, ProxyRackUrlEnvName)
	}

	urlPortRangePattern := regexp.MustCompile(`http[s]?://(?P<USERNAME>[^:@/]+):(?P<PASSWORD>[^:@/]+)@(?P<HOST>[^:@/]+):(?P<START_PORT>\d{2,5})-(?P<END_PORT>\d{2,5})`)
	matches := GetRegexSubmatches(urlPortRangePattern, proxyUrlStr)
	malformedError := fmt.Errorf("Malformed proxy url: must be in format http(s)://username:password@host:start_port-end_port)")
	if len(matches) != 5 {
		return nil, malformedError
	}

	username, exists := matches["USERNAME"]
	if !exists {
		return nil, malformedError
	}
	password, exists := matches["PASSWORD"]
	if !exists {
		return nil, malformedError
	}
	host, exists := matches["HOST"]
	if !exists {
		return nil, malformedError
	}
	startPortStr, exists := matches["START_PORT"]
	if !exists {
		return nil, malformedError
	}
	endPortStr, exists := matches["END_PORT"]
	if !exists {
		return nil, malformedError
	}
	startPort, err := strconv.Atoi(startPortStr)
	if err != nil || startPort < 1 || startPort > 65535 {
		return nil, malformedError
	}
	endPort, err := strconv.Atoi(endPortStr)
	if err != nil || endPort < 1 || endPort > 65535 || endPort < startPort {
		return nil, malformedError
	}

	prpp.portOffset = startPort

	for port := startPort; port <= endPort; port++ {
		prpp.proxyList = append(prpp.proxyList, NewAuthHttpProxyEndpoint(host, uint16(port), username, password, proxyUrlStr, false, ProxyRackRotationUrl))
	}

	SeedRand()

	return prpp, nil
}

func (prpp *ProxyRackAuthHttpProxyProvider) GetProxy() (ProxyEndpoint, error) {
	prpp.mutex.Lock()
	defer prpp.mutex.Unlock()

	if prpp.activeIdxExpiry.Before(time.Now()) {
		err := prpp.refreshSessions()
		if err != nil {
			Log.Errorf("%v", err)
		} else {
			prpp.activeIdxExpiry = time.Now().Add(ProxyRackActiveIdxTTL * time.Second)
		}
	}

	rand.Shuffle(len(prpp.activeIdx), func(i, j int) { prpp.activeIdx[i], prpp.activeIdx[j] = prpp.activeIdx[j], prpp.activeIdx[i] })

	Log.Debugf("Finding a working proxy...")

	for _, idx := range prpp.activeIdx {
		proxyEndpoint := prpp.proxyList[idx]
		if proxyEndpoint.IsBlackListed() {
			proxyEndpoint.BlackList()
			continue
		}

		if proxyEndpoint.lastUsed.After(time.Now().Add(-prpp.repeatInterval)) {
			continue
		}

		if proxyEndpoint.expiry.Before(time.Now().Add(30 * time.Second)) {
			continue
		}

		if testProxy(proxyEndpoint) {
			proxyEndpoint.lastUsed = time.Now()
			Log.Debugf("Found working proxy from session data: %s", censorUrl(proxyEndpoint.GetUrl()))
			return proxyEndpoint, nil
		} else {
			proxyEndpoint.BlackList()
		}
	}

	Log.Debugf("Couldn't find a working proxy from session, requesting...")

	for i := 0; i < 10; i++ {
		idx := rand.Intn(len(prpp.proxyList))
		proxyEndpoint := prpp.proxyList[idx]
		if proxyEndpoint.IsBlackListed() {
			continue
		}

		if proxyEndpoint.lastUsed.After(time.Now().Add(-prpp.repeatInterval)) {
			continue
		}

		if testProxy(proxyEndpoint) {
			proxyEndpoint.lastUsed = time.Now()
			Log.Debugf("Got working proxy after %d tries: %s", i+1, censorUrl(proxyEndpoint.GetUrl()))
			return proxyEndpoint, nil
		}
	}

	prpp.activeIdxExpiry = time.Unix(0, 0)

	return nil, fmt.Errorf("Could not find a valid proxy!")
}

type ProxyRackSession struct {
	Expiration ProxyRackExpiration `json:"expiration"`
	Port       int                 `json:"port"`
	ProxyInfo  ProxyRackProxyInfo  `json:"proxy"`
}

type ProxyRackExpiration struct {
	Seconds int `json:"seconds"`
}

type ProxyRackProxyInfo struct {
	Country string `json:"country"`
	Online  bool   `json:"online"`
}

func (prpp *ProxyRackAuthHttpProxyProvider) refreshSessions() error {
	httpClient := prpp.proxyList[0].GetHttpClient()
	resp, err := httpClient.Get("http://api.proxyrack.net/sessions")
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	sessions := make([]ProxyRackSession, 0)
	err = json.Unmarshal(body, &sessions)
	if err != nil {
		return err
	}

	prpp.activeIdx = make([]int, 0, len(sessions))
	for _, session := range sessions {
		idx := session.Port - prpp.portOffset
		if idx >= len(prpp.proxyList) {
			Log.Errorf("Proxy index out of range: %d >= %d", idx, len(prpp.proxyList))
			continue
		}

		if session.Expiration.Seconds > 30 &&
			session.ProxyInfo.Country == "US" &&
			session.ProxyInfo.Online &&
			!prpp.proxyList[idx].IsBlackListed() {
			prpp.activeIdx = append(prpp.activeIdx, idx)
			prpp.proxyList[idx].expiry = time.Now().Add(time.Duration(session.Expiration.Seconds) * time.Second)
		}
	}

	Log.Debugf("Found %d active and valid proxies in session data", len(prpp.activeIdx))

	return nil
}
