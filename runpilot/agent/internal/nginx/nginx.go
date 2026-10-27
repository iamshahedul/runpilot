package nginx

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"runpilot/shared/contracts"
)

var domainPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)

type NginxManager struct {
	SitesDir string
}

func NewNginxManager(sitesDir string) *NginxManager {
	if sitesDir == "" {
		sitesDir = "/etc/nginx/conf.d"
	}
	return &NginxManager{SitesDir: sitesDir}
}

// ValidateDomain ensures domain contains only safe, standard domain characters
func ValidateDomain(domain string) error {
	domain = strings.TrimSpace(domain)
	if len(domain) == 0 || len(domain) > 253 {
		return fmt.Errorf("invalid domain length: %s", domain)
	}
	if !domainPattern.MatchString(domain) {
		return fmt.Errorf("invalid domain format: %s", domain)
	}
	return nil
}

const nginxConfTmpl = `# RunPilot Generated Reverse Proxy Configuration for App: {{ .AppID }}
# DO NOT EDIT DIRECTLY. MANAGED AUTOMATICALLY BY RUNPILOT AGENT.

upstream runpilot_upstream_{{ .AppID }} {
    server 127.0.0.1:{{ .UpstreamPort }};
    keepalive 32;
}

{{ if .SSLEnabled }}
# HTTP -> HTTPS Redirect
server {
    listen 80;
    listen [::]:80;
    server_name {{ .ServerNames }};

    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        return 301 https://$host$request_uri;
    }
}

# HTTPS Main Server Block
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name {{ .ServerNames }};

    ssl_certificate {{ .SSLCertPath }};
    ssl_certificate_key {{ .SSLKeyPath }};
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    client_max_body_size 50M;

    location / {
        proxy_pass http://runpilot_upstream_{{ .AppID }};
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;

        {{ if .WebSocketEnabled }}
        # WebSocket Upgrade Headers
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
        {{ else }}
        proxy_set_header Connection "";
        {{ end }}
    }
}
{{ else }}
# Standard HTTP Server Block
server {
    listen 80;
    listen [::]:80;
    server_name {{ .ServerNames }};

    client_max_body_size 50M;

    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        proxy_pass http://runpilot_upstream_{{ .AppID }};
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto http;

        {{ if .WebSocketEnabled }}
        # WebSocket Upgrade Headers
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
        {{ else }}
        proxy_set_header Connection "";
        {{ end }}
    }
}
{{ end }}
`

type tmplData struct {
	AppID            string
	ServerNames      string
	UpstreamPort     int
	SSLEnabled       bool
	SSLCertPath      string
	SSLKeyPath       string
	WebSocketEnabled bool
}

// GenerateConfig generates the Nginx site configuration string
func (m *NginxManager) GenerateConfig(req contracts.AgentNginxConfigRequest) (string, error) {
	for _, d := range req.Domains {
		if err := ValidateDomain(d); err != nil {
			return "", err
		}
	}

	serverNames := strings.Join(req.Domains, " ")
	if strings.TrimSpace(serverNames) == "" {
		serverNames = "_"
	}

	data := tmplData{
		AppID:            req.AppID,
		ServerNames:      serverNames,
		UpstreamPort:     req.UpstreamPort,
		SSLEnabled:       req.SSLEnabled,
		SSLCertPath:      req.SSLCertPath,
		SSLKeyPath:       req.SSLKeyPath,
		WebSocketEnabled: req.WebSocketEnabled,
	}

	t, err := template.New("nginx").Parse(nginxConfTmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// WriteAndValidate writes configuration file and tests with nginx -t
func (m *NginxManager) WriteAndValidate(req contracts.AgentNginxConfigRequest) error {
	confContent, err := m.GenerateConfig(req)
	if err != nil {
		return err
	}

	_ = os.MkdirAll(m.SitesDir, 0755)
	fileName := fmt.Sprintf("runpilot-app-%s.conf", req.AppID)
	targetPath := filepath.Join(m.SitesDir, fileName)

	// Backup existing if any
	oldContent, _ := os.ReadFile(targetPath)

	if err := os.WriteFile(targetPath, []byte(confContent), 0644); err != nil {
		return fmt.Errorf("failed to write nginx configuration file %s: %w", targetPath, err)
	}

	// Validate with nginx -t if nginx binary is installed
	if _, err := exec.LookPath("nginx"); err == nil {
		testCmd := exec.Command("nginx", "-t")
		out, err := testCmd.CombinedOutput()
		if err != nil {
			// Rollback to previous configuration or remove
			if len(oldContent) > 0 {
				_ = os.WriteFile(targetPath, oldContent, 0644)
			} else {
				_ = os.Remove(targetPath)
			}
			return fmt.Errorf("nginx -t validation failed: %s (%w)", strings.TrimSpace(string(out)), err)
		}

		// Reload nginx
		_ = exec.Command("nginx", "-s", "reload").Run()
	}

	return nil
}

// RemoveConfig deletes the app's reverse proxy configuration
func (m *NginxManager) RemoveConfig(appID string) error {
	fileName := fmt.Sprintf("runpilot-app-%s.conf", appID)
	targetPath := filepath.Join(m.SitesDir, fileName)
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	if _, err := exec.LookPath("nginx"); err == nil {
		_ = exec.Command("nginx", "-s", "reload").Run()
	}

	return nil
}
