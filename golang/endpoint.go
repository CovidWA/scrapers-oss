package csg

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

const EndpointUrl = "url"
const EndpointMethod = "method"
const EndpointBody = "body"
const EndpointHeaders = "headers"
const EndpointCookieWhitelist = "cookie_whitelist"
const EndpointAllowedStatusCodes = "allowed_status_codes"
const EndpointTimeout = "timeout"
const EndpointDefaultTimeout = 10

type Endpoint struct {
	Url                string
	Method             string
	Body               string
	Headers            []Header
	CookieWhitelist    []string
	Cookies            map[string]string
	AllowedStatusCodes []int
	HttpClient         *http.Client
	Timeout            int
}

type Header struct {
	Name  string
	Value string
}

func NewEndpoint(rawEndpointParams interface{}) (*Endpoint, error) {
	params := rawEndpointParams.(map[string]interface{})
	endpoint := new(Endpoint)
	if _, exists := params[EndpointUrl]; !exists {
		return nil, fmt.Errorf("Missing endpoint field: %s", EndpointUrl)
	}

	endpoint.Url = params[EndpointUrl].(string)

	if _, exists := params[EndpointMethod]; !exists {
		return nil, fmt.Errorf("Missing endpoint field: %s", EndpointMethod)
	}
	endpoint.Method = params[EndpointMethod].(string)

	if endpoint.Method == "POST" {
		if _, exists := params[EndpointBody]; !exists {
			return nil, fmt.Errorf("Missing endpoint field: %s", EndpointBody)
		}
		endpoint.Body = params[EndpointBody].(string)
	}

	endpoint.Headers = make([]Header, 0)

	if _, exists := params[EndpointHeaders]; exists {
		headers := params[EndpointHeaders].(map[interface{}]interface{})
		for headerName, headerValue := range headers {
			cookedHeader := Header{
				Name:  headerName.(string),
				Value: headerValue.(string),
			}

			endpoint.Headers = append(endpoint.Headers, cookedHeader)
		}
	}

	endpoint.CookieWhitelist = make([]string, 0)
	if _, exists := params[EndpointCookieWhitelist]; exists {
		cookieWhitelist := params[EndpointCookieWhitelist].([]interface{})
		for _, cookieName := range cookieWhitelist {
			endpoint.CookieWhitelist = append(endpoint.CookieWhitelist, cookieName.(string))
		}
	}

	endpoint.Cookies = make(map[string]string)

	if _, exists := params[EndpointAllowedStatusCodes]; exists {
		endpoint.AllowedStatusCodes = make([]int, 0)
		allowedStatusCodes := params[EndpointAllowedStatusCodes].([]interface{})
		for _, code := range allowedStatusCodes {
			endpoint.AllowedStatusCodes = append(endpoint.AllowedStatusCodes, code.(int))
		}
	}

	endpoint.Timeout, _ = getIntOptionalWithDefault(params, EndpointTimeout, EndpointDefaultTimeout)

	return endpoint, nil
}

const FetchCacheDefaultTTL = 120

func (endpoint *Endpoint) GenerateCacheKey(name string) string {
	return endpoint.GenerateCacheKeyWithTTL(name, FetchCacheDefaultTTL)
}

func (endpoint *Endpoint) GenerateCacheKeyWithTTL(name string, ttl int64) string {
	if endpoint.Method == "GET" {
		return fmt.Sprintf("%s|%d", endpoint.Url, ttl)
	} else if endpoint.Method == "POST" {
		hash := sha256.Sum256([]byte(endpoint.Body))
		hashString := hex.EncodeToString(hash[:])
		return fmt.Sprintf("%s|%s|%d", endpoint.Url, hashString, ttl)
	} else {
		return ""
	}
}

func (endpoint *Endpoint) FetchCached(name string) (body []byte, cacheMiss bool, err error) {
	return endpoint.FetchCachedWithTTL(name, FetchCacheDefaultTTL)
}

func (endpoint *Endpoint) FetchCachedWithTTL(name string, ttl int64) (body []byte, cacheMiss bool, err error) {
	key := endpoint.GenerateCacheKeyWithTTL(name, ttl)
	if len(key) == 0 {
		body, _, err := endpoint.Fetch(name)
		return body, true, err
	}

	body, ok := Cache.GetOrLock(key).([]byte)

	if !ok || body == nil {
		defer Cache.Unlock(key)
		body, _, err := endpoint.Fetch(name)
		if err != nil {
			return body, true, err
		}
		Cache.Put(key, body, ttl, -1)

		return body, true, nil
	}

	return body, false, nil
}

func (endpoint *Endpoint) Fetch(name string) ([]byte, map[string][]string, error) {
	var resp *http.Response
	var err error

	url := replaceMagic(endpoint.Url)

	if endpoint.Method == "POST" || endpoint.Method == "GET" {
		client := endpoint.HttpClient
		if client == nil {
			client = &http.Client{
				Timeout: time.Duration(endpoint.Timeout) * time.Second,
				Transport: &http.Transport{
					DisableKeepAlives: false,
				},
			}
		}

		req, err := http.NewRequest(endpoint.Method, url, strings.NewReader(replaceMagic(endpoint.Body)))
		if err != nil {
			return nil, nil, err
		}

		for _, header := range endpoint.Headers {
			req.Header.Add(header.Name, header.Value)
		}

		cookie := req.Header.Get("Cookie")
		if len(endpoint.Cookies) > 0 {
			for cookieName, cookieVal := range endpoint.Cookies {
				if len(cookie) > 0 {
					cookie = cookie + "; "
				}
				cookie = cookie + fmt.Sprintf("%s=%s", cookieName, cookieVal)
			}
			req.Header.Set("Cookie", cookie)
		}
		//Log.Debugf("COOKIE: %s", cookie)

		resp, err = client.Do(req)

		if err != nil {
			Log.Debugf("WARNING: Error during fetch: %v", err)
			return nil, nil, err
		}

	} else {
		err = fmt.Errorf("Unknown method: %s", endpoint.Method)
		return nil, nil, err
	}

	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	respHeaders := make(map[string][]string)
	gzipContent := false
	for headerKey, headerVals := range resp.Header {
		respHeaders[headerKey] = make([]string, 0)
		respHeaders[headerKey] = append(respHeaders[headerKey], headerVals...)

		if strings.ToLower(headerKey) == "set-cookie" {

			for _, val := range headerVals {
				setCookieParts := strings.Split(val, ";")
				idx := strings.Index(setCookieParts[0], "=")
				cookieName := setCookieParts[0][0:idx]
				cookieVal := setCookieParts[0][idx+1:]
				if len(cookieVal) == 0 {
					continue
				}
				//Log.Debugf("Set-Cookie: %s - %s", cookieName, cookieVal)

				existingCookieVal, existingCookieOk := endpoint.Cookies[cookieName]
				whitelisted := false
				for _, val := range endpoint.CookieWhitelist {
					if val == cookieName || val == "*" {
						whitelisted = true
						break
					}
				}

				if whitelisted && (!existingCookieOk || existingCookieVal != cookieVal) {
					endpoint.Cookies[cookieName] = cookieVal
					//Log.Debugf("%s: Set-Cookie: %s = %s", name, cookieName, cookieVal)
				}
			}
		}

		if strings.ToLower(headerKey) == "content-encoding" {
			if strings.ToLower(headerVals[0]) == "gzip" {
				gzipContent = true
			}
		}
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	if gzipContent {
		Log.Debug("Decompressing gzipped content...")

		gzReader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, nil, err
		}

		body, err = ioutil.ReadAll(gzReader)
		if err != nil {
			return nil, nil, err
		}
	}

	Log.Debugf("%s: fetched %d bytes with status code %d from %s", name, len(body), resp.StatusCode, url)

	if resp.StatusCode != 200 {
		allowed := false
		if endpoint.AllowedStatusCodes != nil {
			for _, code := range endpoint.AllowedStatusCodes {
				if resp.StatusCode == code {
					allowed = true
				}
			}
		}

		if !allowed {
			Log.Warnf("%s: Status code: %d, %s", name, resp.StatusCode, string(body[:128]))
			err = fmt.Errorf("Status code: %d", resp.StatusCode)
			return body, respHeaders, err
		}
	}

	return body, respHeaders, nil
}
