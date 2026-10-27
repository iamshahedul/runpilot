package proxy

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"runpilot/control-plane/internal/services"
)

// Router inspects the Host header of incoming requests and dynamically routes traffic
// either to the RunPilot Control Plane Dashboard/API or to the target application process.
type Router struct {
	apiHandler http.Handler
	appService *services.AppService
	serverHost string
	proxies    sync.Map // int (port) -> *httputil.ReverseProxy
}

// NewRouter constructs a native RunPilot dynamic host router
func NewRouter(apiHandler http.Handler, appService *services.AppService, serverHost string) *Router {
	return &Router{
		apiHandler: apiHandler,
		appService: appService,
		serverHost: strings.ToLower(strings.TrimSpace(serverHost)),
	}
}

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(strings.TrimSpace(host))

	// 1. Control Plane & API Traffic:
	// Matches localhost, 127.0.0.1, server IP/hostname, or explicit /api/ endpoints
	if host == "" || host == "localhost" || host == "127.0.0.1" || host == rt.serverHost || strings.HasPrefix(r.URL.Path, "/api/") {
		rt.apiHandler.ServeHTTP(w, r)
		return
	}

	// 2. Query application assigned to this domain
	appID, port, err := rt.appService.GetAppPortByDomain(host)
	if err != nil || port <= 0 {
		// If accessing dashboard assets without custom domain, fall back to dashboard
		if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/css/") || strings.HasPrefix(r.URL.Path, "/js/") {
			rt.apiHandler.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Domain Not Attached - RunPilot</title>
  <style>
    body { background: #0B1120; color: #F9FAFB; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
    .card { background: #111827; border: 1px solid #1F2937; padding: 36px; border-radius: 12px; text-align: center; max-width: 480px; box-shadow: 0 4px 24px rgba(0,0,0,0.4); }
    h1 { color: #4ADE80; font-size: 20px; margin: 0 0 10px; }
    p { color: #9CA3AF; font-size: 14px; line-height: 1.5; margin: 0; }
    .badge { display: inline-block; background: rgba(74, 222, 128, 0.1); color: #4ADE80; padding: 4px 10px; border-radius: 6px; font-family: monospace; margin: 12px 0; }
  </style>
</head>
<body>
  <div class="card">
    <h1>⚡ RunPilot Native Router</h1>
    <div class="badge">%s</div>
    <p>This domain is pointed to this server but is not currently attached to any active RunPilot application.</p>
  </div>
</body>
</html>`, host)
		return
	}

	// 3. Dynamic Native Reverse Proxying to unprivileged application process
	var revProxy *httputil.ReverseProxy
	if val, ok := rt.proxies.Load(port); ok {
		revProxy = val.(*httputil.ReverseProxy)
	} else {
		targetURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
		revProxy = httputil.NewSingleHostReverseProxy(targetURL)

		revProxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, proxyErr error) {
			rw.Header().Set("Content-Type", "text/html; charset=utf-8")
			rw.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(rw, `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Application Starting - RunPilot</title>
  <style>
    body { background: #0B1120; color: #F9FAFB; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
    .card { background: #111827; border: 1px solid #1F2937; padding: 36px; border-radius: 12px; text-align: center; max-width: 480px; box-shadow: 0 4px 24px rgba(0,0,0,0.4); }
    h1 { color: #F59E0B; font-size: 20px; margin: 0 0 10px; }
    p { color: #9CA3AF; font-size: 14px; line-height: 1.5; margin: 0; }
    .badge { display: inline-block; background: rgba(245, 158, 11, 0.1); color: #F59E0B; padding: 4px 10px; border-radius: 6px; font-family: monospace; margin: 12px 0; }
  </style>
</head>
<body>
  <div class="card">
    <h1>⚡ Application Offline or Starting</h1>
    <div class="badge">%s (port %d)</div>
    <p>The application backend is not responding yet. Please check your deployment logs or process status in the RunPilot Control Panel.</p>
  </div>
</body>
</html>`, appID, port)
		}

		rt.proxies.Store(port, revProxy)
	}

	// Forward standard proxy headers (HTTP/2 & WebSocket upgrades handled natively by Go)
	r.Header.Set("X-Forwarded-Host", r.Host)
	r.Header.Set("X-Real-IP", r.RemoteAddr)
	revProxy.ServeHTTP(w, r)
}
