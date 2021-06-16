package csg

import (
	"os"
	"strings"
	"testing"
)

func TestNewProxyRackAuthHttpProxyProviderDefaults(t *testing.T) {
	Log.SetLevel("debug")

	if !HasAWSCredentials() && len(os.Getenv(ProxyRackUrlEnvName)) == 0 {
		Log.Infof("No AWS credentials or proxy url env variable found, skipping test")
		return
	}

	if strings.Contains(os.Getenv(ProxyRackUrlEnvName), "dummyurl") {
		Log.Infof("Dummy proxy config found, skipping test")
		return
	}

	provider, err := NewProxyRackAuthHttpProxyProviderDefaults()
	if err != nil {
		t.Errorf("Unexpected Error: %v", err)
		return
	}

	proxy, err := provider.GetProxy()
	if err != nil {
		t.Errorf("Unexpected Error: %v", err)
		return
	}

	client := proxy.GetHttpClient()
	if client == nil {
		t.Errorf("Get not get valid client from proxy endpoint: %v", proxy)
		return
	}
}
