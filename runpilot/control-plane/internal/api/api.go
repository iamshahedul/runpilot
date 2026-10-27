package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"runpilot/control-plane/internal/audit"
	"runpilot/control-plane/internal/auth"
	"runpilot/control-plane/internal/config"
	"runpilot/control-plane/internal/models"
	"runpilot/control-plane/internal/services"
	"runpilot/shared/contracts"
)

type API struct {
	cfg         *config.Config
	authService *auth.AuthService
	appService  *services.AppService
	pkgService  *services.PackageService
	agentClient *services.AgentClient
	auditLogger *audit.AuditLogger
	mux         *http.ServeMux
}

func NewAPI(
	cfg *config.Config,
	authService *auth.AuthService,
	appService *services.AppService,
	pkgService *services.PackageService,
	agentClient *services.AgentClient,
	auditLogger *audit.AuditLogger,
) *API {
	a := &API{
		cfg:         cfg,
		authService: authService,
		appService:  appService,
		pkgService:  pkgService,
		agentClient: agentClient,
		auditLogger: auditLogger,
		mux:         http.NewServeMux(),
	}
	a.routes()
	return a
}

func (a *API) routes() {
	// Public routes
	a.mux.HandleFunc("/api/v1/auth/login", a.handleLogin)

	// Protected routes wrapper
	protect := a.authService.Middleware

	a.mux.Handle("/api/v1/auth/me", protect(http.HandlerFunc(a.handleMe)))
	a.mux.Handle("/api/v1/auth/sso-token", protect(http.HandlerFunc(a.handleSSOToken)))
	a.mux.Handle("/api/v1/profile", protect(http.HandlerFunc(a.handleProfile)))
	a.mux.Handle("/api/v1/profile/password", protect(http.HandlerFunc(a.handleChangePassword)))
	a.mux.Handle("/api/v1/users", protect(http.HandlerFunc(a.handleUsers)))
	a.mux.Handle("/api/v1/users/reset-password", protect(http.HandlerFunc(a.handleResetPassword)))

	// Packages (cPanel-style hosting plans)
	a.mux.Handle("/api/v1/packages", protect(http.HandlerFunc(a.handlePackages)))
	a.mux.Handle("/api/v1/packages/", protect(http.HandlerFunc(a.handlePackageSubroutes)))

	a.mux.Handle("/api/v1/servers", protect(http.HandlerFunc(a.handleServers)))
	a.mux.Handle("/api/v1/servers/stats", protect(http.HandlerFunc(a.handleServerStats)))

	// Applications CRUD & operations
	a.mux.Handle("/api/v1/applications", protect(http.HandlerFunc(a.handleApplications)))
	a.mux.Handle("/api/v1/applications/", protect(http.HandlerFunc(a.handleAppSubroutes)))

	// Audit Logs
	a.mux.Handle("/api/v1/audit-events", protect(http.HandlerFunc(a.handleAuditEvents)))

	// Static Frontend Dashboard
	webRoot := a.cfg.Server.WebRoot
	if _, err := os.Stat(webRoot); err != nil {
		webRoot = "control-plane/web"
	}
	fileServer := http.FileServer(http.Dir(webRoot))
	a.mux.Handle("/", fileServer)
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email      string `json:"email"`
		Username   string `json:"username"`
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	ident := req.Identifier
	if ident == "" {
		if req.Email != "" {
			ident = req.Email
		} else {
			ident = req.Username
		}
	}

	user, token, err := a.authService.Authenticate(ident, req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Set HTTP-only session cookie as well
	http.SetCookie(w, &http.Cookie{
		Name:     "runpilot_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	a.auditLogger.Log(r, "", "user_login", fmt.Sprintf("User %s logged in", user.Email))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetUserFromContext(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(claims)
}

func (a *API) handleServers(w http.ResponseWriter, r *http.Request) {
	// Returns registered servers
	servers := []map[string]interface{}{
		{
			"id":        1,
			"name":      "Primary Node",
			"hostname":  "localhost",
			"status":    "active",
			"ip":        "127.0.0.1",
			"cpu_cores": 4,
			"memory_mb": 8192,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(servers)
}

func (a *API) handleServerStats(w http.ResponseWriter, r *http.Request) {
	metrics, err := a.agentClient.GetServerMetrics()
	if err != nil {
		// Provide local fallback metrics
		metrics = &contracts.AgentServerMetrics{
			Hostname:      "runpilot-node",
			OS:            "linux",
			CPUUsagePct:   15.4,
			MemoryTotalMB: 8192,
			MemoryUsedMB:  2140,
			MemoryFreeMB:  6052,
			DiskTotalGB:   80,
			DiskUsedGB:    20,
			DiskFreeGB:    60,
			ActiveApps:    1,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metrics)
}

func (a *API) handleApplications(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetUserFromContext(r.Context())
	isAdmin := claims.Role == "admin"

	switch r.Method {
	case http.MethodGet:
		apps, err := a.appService.ListApps(claims.UserID, isAdmin)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(apps)

	case http.MethodPost:
		var appReq models.Application
		if err := json.NewDecoder(r.Body).Decode(&appReq); err != nil {
			http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
			return
		}

		created, err := a.appService.CreateApp(claims.UserID, appReq)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
			return
		}

		a.auditLogger.Log(r, created.ID, "create_app", fmt.Sprintf("Created app %s (%s)", created.Name, created.RuntimeType))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(created)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) handleAppSubroutes(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetUserFromContext(r.Context())
	isAdmin := claims.Role == "admin"

	// Parse /api/v1/applications/{appID}/...
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/applications/")
	parts := strings.Split(path, "/")
	appID := parts[0]

	if appID == "" {
		http.Error(w, "Application ID required", http.StatusBadRequest)
		return
	}

	if !isAdmin {
		if _, err := a.appService.GetApp(appID, claims.UserID, false); err != nil {
			http.Error(w, `{"error":"access denied: application not found or unauthorized"}`, http.StatusForbidden)
			return
		}
	}

	if len(parts) == 1 {
		// Route: /api/v1/applications/{id}
		switch r.Method {
		case http.MethodGet:
			app, err := a.appService.GetApp(appID, claims.UserID, isAdmin)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(app)

		case http.MethodPut:
			var upd models.Application
			if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
				http.Error(w, "Invalid json", http.StatusBadRequest)
				return
			}
			if err := a.appService.UpdateApp(appID, claims.UserID, isAdmin, upd); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			a.auditLogger.Log(r, appID, "update_app", "Updated application config")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})

		case http.MethodDelete:
			if err := a.appService.DeleteApp(appID, claims.UserID, isAdmin); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			a.auditLogger.Log(r, appID, "delete_app", "Deleted application")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	action := parts[1]

	switch action {
	case "deploy":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		dep, err := a.appService.TriggerDeploy(appID, claims.UserID, isAdmin, claims.Email)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error(), "deployment": dep})
			return
		}
		a.auditLogger.Log(r, appID, "deploy_app", fmt.Sprintf("Triggered deployment %s", dep.ID))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(dep)

	case "deployments":
		if len(parts) >= 4 && parts[3] == "log" {
			// /api/v1/applications/{id}/deployments/{dep_id}/log
			depID := parts[2]
			logStr := a.appService.GetDeploymentLog(depID)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"log": logStr})
			return
		}

		deps, err := a.appService.GetDeployments(appID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(deps)

	case "start", "stop", "restart":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		stat, err := a.appService.ProcessAction(appID, claims.UserID, isAdmin, action)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.auditLogger.Log(r, appID, fmt.Sprintf("%s_service", action), fmt.Sprintf("Executed service %s", action))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stat)

	case "logs":
		lines, _ := strconv.Atoi(r.URL.Query().Get("lines"))
		logList, err := a.agentClient.GetLogs(appID, lines)
		if err != nil {
			logList = []string{"[RunPilot] No logs available"}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"lines": logList})

	case "rollback":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ReleaseID string `json:"release_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if err := a.appService.RollbackApp(appID, claims.UserID, isAdmin, req.ReleaseID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.auditLogger.Log(r, appID, "rollback_app", fmt.Sprintf("Rolled back to %s", req.ReleaseID))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})

	case "domains":
		if r.Method == http.MethodPost {
			var dReq struct {
				Domain string `json:"domain"`
			}
			_ = json.NewDecoder(r.Body).Decode(&dReq)
			if err := a.appService.AddDomain(appID, dReq.Domain); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			a.auditLogger.Log(r, appID, "add_domain", fmt.Sprintf("Attached domain %s", dReq.Domain))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}
		if r.Method == http.MethodDelete {
			domainName := r.URL.Query().Get("domain")
			if err := a.appService.RemoveDomain(appID, domainName); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			a.auditLogger.Log(r, appID, "remove_domain", fmt.Sprintf("Detached domain %s", domainName))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

	case "env":
		if r.Method == http.MethodGet {
			env := a.appService.GetAppEnvVars(appID)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(env)
			return
		}
		if r.Method == http.MethodPost {
			var vars map[string]string
			if err := json.NewDecoder(r.Body).Decode(&vars); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			for k, v := range vars {
				_ = a.appService.SetAppEnvVar(appID, k, v)
			}
			a.auditLogger.Log(r, appID, "set_env_vars", "Updated environment variables")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

	default:
		http.Error(w, "Unknown action", http.StatusNotFound)
	}
}

func (a *API) handleAuditEvents(w http.ResponseWriter, r *http.Request) {
	events, err := a.auditLogger.ListRecent(50)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(events)
}

func (a *API) handleSSOToken(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetUserFromContext(r.Context())
	if claims.Role != "admin" {
		http.Error(w, `{"error":"forbidden: admin access required"}`, http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID     int64  `json:"user_id"`
		Identifier string `json:"identifier"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	var targetUser *models.User
	var err error
	if req.UserID > 0 {
		targetUser, err = a.authService.GetUserByID(req.UserID)
	} else if req.Identifier != "" {
		targetUser, err = a.authService.GetUserByIdentifier(req.Identifier)
	} else {
		targetUser, err = a.authService.GetUserByID(claims.UserID)
	}

	if err != nil || targetUser == nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	token, err := a.authService.GenerateTokenForUser(targetUser.ID, targetUser.Email, targetUser.Role)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"user":  targetUser,
	})
}

func (a *API) handleProfile(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetUserFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		user, err := a.authService.GetUserByID(claims.UserID)
		if err != nil {
			http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(user)

	case http.MethodPut:
		var req struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			FullName string `json:"full_name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}

		updated, err := a.authService.UpdateProfile(claims.UserID, req.Username, req.Email, req.FullName)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
			return
		}

		a.auditLogger.Log(r, "", "update_profile", fmt.Sprintf("User %d updated profile to %s", claims.UserID, req.Email))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(updated)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	claims, _ := auth.GetUserFromContext(r.Context())
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if err := a.authService.ChangePassword(claims.UserID, req.OldPassword, req.NewPassword); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	a.auditLogger.Log(r, "", "change_password", fmt.Sprintf("User %d changed their password", claims.UserID))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (a *API) handleUsers(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetUserFromContext(r.Context())
	if claims.Role != "admin" {
		http.Error(w, `{"error":"forbidden: admin access required"}`, http.StatusForbidden)
		return
	}

	if r.Method == http.MethodGet {
		users, err := a.authService.ListUsers()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(users)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Password string `json:"password"`
			FullName string `json:"full_name"`
			Role     string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}

		user, err := a.authService.CreateOrUpdateUser(req.Username, req.Email, req.Password, req.FullName, req.Role)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
			return
		}

		a.auditLogger.Log(r, "", "create_user", fmt.Sprintf("Admin created/updated user %s (%s)", user.Email, user.Username))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(user)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (a *API) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetUserFromContext(r.Context())
	if claims.Role != "admin" {
		http.Error(w, `{"error":"forbidden: admin access required"}`, http.StatusForbidden)
		return
	}

	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if err := a.authService.ResetPasswordDirectly(req.Identifier, req.Password); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	a.auditLogger.Log(r, "", "reset_password", fmt.Sprintf("Admin reset password for %s", req.Identifier))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (a *API) handlePackages(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetUserFromContext(r.Context())

	switch r.Method {
	case http.MethodGet:
		pkgs, err := a.pkgService.ListPackages()
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pkgs)

	case http.MethodPost:
		if claims.Role != "admin" {
			http.Error(w, `{"error":"forbidden: admin access required"}`, http.StatusForbidden)
			return
		}
		var pkgReq models.Package
		if err := json.NewDecoder(r.Body).Decode(&pkgReq); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		created, err := a.pkgService.CreatePackage(pkgReq)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		a.auditLogger.Log(r, "", "create_package", fmt.Sprintf("Created package %s", created.Name))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(created)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) handlePackageSubroutes(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetUserFromContext(r.Context())
	if claims.Role != "admin" && r.Method != http.MethodGet {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/packages/")
	idStr = strings.TrimSpace(idStr)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid package ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		pkg, err := a.pkgService.GetPackage(idStr)
		if err != nil {
			http.Error(w, `{"error":"package not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pkg)

	case http.MethodPut:
		var pkgReq models.Package
		if err := json.NewDecoder(r.Body).Decode(&pkgReq); err != nil {
			http.Error(w, "Invalid json", http.StatusBadRequest)
			return
		}
		if err := a.pkgService.UpdatePackage(id, pkgReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.auditLogger.Log(r, "", "update_package", fmt.Sprintf("Updated package %d", id))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})

	case http.MethodDelete:
		if err := a.pkgService.DeletePackage(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a.auditLogger.Log(r, "", "delete_package", fmt.Sprintf("Deleted package %d", id))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) Handler() http.Handler {
	return a.mux
}
