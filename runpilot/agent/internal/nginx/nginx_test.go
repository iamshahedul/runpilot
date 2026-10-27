package nginx

import (
	"strings"
	"testing"

	"runpilot/shared/contracts"
)

func TestDomainValidation(t *testing.T) {
	valid := []string{"example.com", "api.sub.domain.org", "runpilot.io", "my-app-1.net"}
	for _, d := range valid {
		if err := ValidateDomain(d); err != nil {
			t.Errorf("Valid domain %s rejected: %v", d, err)
		}
	}

	invalid := []string{
		"http://example.com",
		"example.com/path",
		"bad;domain.com",
		"domain..com",
		"*.wildcard.com",
		"",
	}
	for _, d := range invalid {
		if err := ValidateDomain(d); err == nil {
			t.Errorf("Invalid domain %s was accepted!", d)
		}
	}
}

func TestGenerateNginxConfig(t *testing.T) {
	mgr := NewNginxManager("/tmp/nginx-test")

	// HTTP Config
	httpReq := contracts.AgentNginxConfigRequest{
		AppID:            "app-test-1",
		Domains:          []string{"test.example.com"},
		UpstreamPort:     3000,
		SSLEnabled:       false,
		WebSocketEnabled: true,
	}

	conf, err := mgr.GenerateConfig(httpReq)
	if err != nil {
		t.Fatalf("GenerateConfig HTTP failed: %v", err)
	}

	if !strings.Contains(conf, "server 127.0.0.1:3000;") {
		t.Errorf("Missing upstream port in config: %s", conf)
	}
	if !strings.Contains(conf, "proxy_set_header Upgrade $http_upgrade;") {
		t.Errorf("Missing WebSocket upgrade header: %s", conf)
	}

	// HTTPS Config
	sslReq := contracts.AgentNginxConfigRequest{
		AppID:            "app-test-2",
		Domains:          []string{"secure.example.com"},
		UpstreamPort:     4000,
		SSLEnabled:       true,
		SSLCertPath:      "/etc/letsencrypt/live/secure.example.com/fullchain.pem",
		SSLKeyPath:       "/etc/letsencrypt/live/secure.example.com/privkey.pem",
		WebSocketEnabled: false,
	}

	sslConf, err := mgr.GenerateConfig(sslReq)
	if err != nil {
		t.Fatalf("GenerateConfig SSL failed: %v", err)
	}

	if !strings.Contains(sslConf, "return 301 https://$host$request_uri;") {
		t.Errorf("Missing HTTP to HTTPS redirect in SSL config: %s", sslConf)
	}
	if !strings.Contains(sslConf, "ssl_certificate /etc/letsencrypt/live/secure.example.com/fullchain.pem;") {
		t.Errorf("Missing SSL certificate directive: %s", sslConf)
	}
}
