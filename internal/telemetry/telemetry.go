package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dnse-mt5-connector/internal/config"
	"dnse-mt5-connector/internal/logger"
)

type Client struct {
	cfg       config.TelemetryConfig
	http      *http.Client
	clientID  string
	sessionID int64
	mu        sync.Mutex
	lastSent  time.Time
	logger    *logger.FileLogger
}

func NewClient(cfg config.TelemetryConfig, appLog *logger.FileLogger) *Client {
	clientID := loadOrCreateClientID(cfg.ClientIDFile)
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 5 * time.Second,
		},
		clientID:  clientID,
		sessionID: time.Now().UTC().Unix(),
		logger:    appLog,
	}
}

func (c *Client) Track(ctx context.Context, name string, params map[string]any) {
	if c == nil {
		return
	}
	if strings.TrimSpace(c.cfg.MeasurementID) == "" || strings.TrimSpace(c.cfg.APISecret) == "" || c.clientID == "" {
		return
	}
	name = sanitizeEventName(name)
	if name == "" {
		return
	}
	params = sanitizeParams(params)
	params["session_id"] = c.sessionID
	params["engagement_time_msec"] = c.engagementMS()
	if strings.TrimSpace(c.cfg.AppVersion) != "" {
		params["app_version"] = truncate(c.cfg.AppVersion, 100)
	}
	payload := map[string]any{
		"client_id": c.clientID,
		"consent": map[string]string{
			"ad_user_data":       "DENIED",
			"ad_personalization": "DENIED",
		},
		"events": []map[string]any{{
			"name":   name,
			"params": params,
		}},
	}
	body, _ := json.Marshal(payload)
	url := "https://www.google-analytics.com/mp/collect?measurement_id=" + c.cfg.MeasurementID + "&api_secret=" + c.cfg.APISecret
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	go func() {
		resp, err := c.http.Do(req)
		if err != nil {
			if c.logger != nil {
				c.logger.Error("telemetry_send_failed", map[string]any{"event": name, "error": err.Error()})
			}
			return
		}
		_ = resp.Body.Close()
	}()
}

func (c *Client) engagementMS() int64 {
	now := time.Now().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastSent.IsZero() {
		c.lastSent = now
		return 100
	}
	elapsed := now.Sub(c.lastSent).Milliseconds()
	c.lastSent = now
	if elapsed < 100 {
		return 100
	}
	if elapsed > int64((30 * time.Minute).Milliseconds()) {
		c.sessionID = now.Unix()
		return 100
	}
	return elapsed
}

func sanitizeEventName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, name)
	return strings.Trim(name, "_")
}

func sanitizeParams(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for key, value := range in {
		key = sanitizeEventName(key)
		if key == "" || isSensitiveKey(key) {
			continue
		}
		switch v := value.(type) {
		case string:
			out[key] = truncate(v, 100)
		case bool, int, int64, float64:
			out[key] = v
		}
	}
	return out
}

func isSensitiveKey(key string) bool {
	for _, part := range []string{"password", "token", "secret", "api_key", "account", "username", "investor"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func loadOrCreateClientID(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "data/client_id"
	}
	if data, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id
		}
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	id := hex.EncodeToString(raw[:])
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(id), 0o600)
	return id
}
