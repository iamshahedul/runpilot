package audit

import (
	"database/sql"
	"net/http"
	"strings"

	"runpilot/control-plane/internal/auth"
	"runpilot/control-plane/internal/db"
	"runpilot/control-plane/internal/models"
)

type AuditLogger struct {
	db *db.DB
}

func NewAuditLogger(database *db.DB) *AuditLogger {
	return &AuditLogger{db: database}
}

func (l *AuditLogger) Log(r *http.Request, appID string, action string, details string) {
	var userID *int64
	if claims, ok := auth.GetUserFromContext(r.Context()); ok {
		uid := claims.UserID
		userID = &uid
	}

	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		ip = strings.TrimSpace(parts[0])
	}

	_, _ = l.db.Exec(`
		INSERT INTO audit_events (user_id, app_id, action, ip_address, details)
		VALUES (?, ?, ?, ?, ?)
	`, userID, appID, action, ip, details)
}

func (l *AuditLogger) ListRecent(limit int) ([]models.AuditEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rows, err := l.db.Query(`
		SELECT a.id, a.user_id, COALESCE(u.email, 'system'), COALESCE(a.app_id, ''), a.action, COALESCE(a.ip_address, ''), COALESCE(a.details, ''), a.created_at
		FROM audit_events a
		LEFT JOIN users u ON a.user_id = u.id
		ORDER BY a.id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []models.AuditEvent
	for rows.Next() {
		var ev models.AuditEvent
		var uid sql.NullInt64
		if err := rows.Scan(&ev.ID, &uid, &ev.UserEmail, &ev.AppID, &ev.Action, &ev.IPAddress, &ev.Details, &ev.CreatedAt); err != nil {
			continue
		}
		if uid.Valid {
			ev.UserID = &uid.Int64
		}
		events = append(events, ev)
	}
	return events, nil
}
