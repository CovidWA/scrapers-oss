package csg

import (
	"fmt"
	"sync"
	"time"
)

//wrapper that caches proxy endpoints for reuse

const StickyProxyDefaultTTL = 300

type StickyProxyProvider struct {
	provider                 ProxyProvider
	cachedEndpoint           ProxyEndpoint
	cachedEndpointExpiration time.Time
	ttl                      time.Duration
	mutex                    *sync.Mutex
}

func NewStickyProxyProviderDefaults() (ProxyProvider, error) {
	prpp, err := NewProxyRackAuthHttpProxyProviderDefaults()
	if err != nil {
		return nil, err
	}

	return NewStickyProxyProvider(prpp, StickyProxyDefaultTTL)
}

func NewStickyProxyProvider(underlyingProvider ProxyProvider, ttl int) (ProxyProvider, error) {
	if underlyingProvider == nil {
		return nil, fmt.Errorf("Got nil for underlying proxy provider")
	}

	spp := new(StickyProxyProvider)
	spp.provider = underlyingProvider
	spp.ttl = time.Duration(ttl) * time.Second
	spp.mutex = new(sync.Mutex)
	spp.cachedEndpointExpiration = time.Now()

	return spp, nil
}

func (spp *StickyProxyProvider) GetProxy() (ProxyEndpoint, error) {
	spp.mutex.Lock()
	defer spp.mutex.Unlock()

	if spp.cachedEndpoint == nil || spp.cachedEndpointExpiration.Before(time.Now()) || spp.cachedEndpoint.IsBlackListed() {
		var err error
		spp.cachedEndpoint, err = spp.provider.GetProxy()
		if err != nil {
			return nil, err
		}
		spp.cachedEndpointExpiration = time.Now().Add(spp.ttl)
	}

	return spp.cachedEndpoint, nil
}
