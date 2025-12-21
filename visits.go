package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mileusna/useragent"
)

// Visit represents a single redirection event.
type Visit struct {
	Time        time.Time `json:"time"`
	Slug        string    `json:"slug"`
	IP          string    `json:"ip"`
	UserAgent   string    `json:"user_agent"`
	Referer     string    `json:"referer"`
	QueryParams string    `json:"query_params"`
	Language    string    `json:"language"`
	DeviceType  string    `json:"device_type"`
	Browser     string    `json:"browser"`
	BrowserVer  string    `json:"browser_version"`
	OS          string    `json:"os"`
	OSVer       string    `json:"os_version"`
	IsBot       bool      `json:"is_bot"`
	Country     string    `json:"country"`
	City        string    `json:"city"`
}

// VisitTracker handles recording visit statistics.
type VisitTracker struct {
	file   *os.File
	geoip  *GeoIPService
	logger *Logger
	ch     chan Visit
	wg     sync.WaitGroup
}

// NewVisitTracker creates a new instance of VisitTracker.
func NewVisitTracker(geoip *GeoIPService) (*VisitTracker, error) {
	cfg := GetConfig()
	f, err := os.OpenFile(cfg.VisitsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	t := &VisitTracker{
		file:   f,
		geoip:  geoip,
		logger: GetLogger(),
		ch:     make(chan Visit, 100),
	}

	t.wg.Add(1)
	go t.worker()

	return t, nil
}

// Close closes the visit channel and the underlying file.
func (t *VisitTracker) Close() error {
	close(t.ch)
	t.wg.Wait()
	return t.file.Close()
}

func (t *VisitTracker) worker() {
	defer t.wg.Done()
	encoder := json.NewEncoder(t.file)
	for v := range t.ch {
		if err := encoder.Encode(v); err != nil {
			t.logger.Error("Failed to write visit", "error", err)
		}
	}
}

// Record adds a new visit to the processing queue.
func (t *VisitTracker) Record(r *http.Request, slug string) {
	uaStr := r.UserAgent()
	ua := useragent.Parse(uaStr)
	ip := getRealIP(r)

	var country, city string
	if t.geoip != nil {
		country, city = t.geoip.Lookup(ip)
	}

	v := Visit{
		Time:        time.Now(),
		Slug:        slug,
		IP:          ip,
		UserAgent:   uaStr,
		Referer:     r.Referer(),
		QueryParams: r.URL.RawQuery,
		Language:    r.Header.Get("Accept-Language"),
		DeviceType:  getDeviceType(ua),
		Browser:     ua.Name,
		BrowserVer:  ua.Version,
		OS:          ua.OS,
		OSVer:       ua.OSVersion,
		IsBot:       ua.Bot,
		Country:     country,
		City:        city,
	}

	select {
	case t.ch <- v:
	default:
		t.logger.Warn("Visit queue full, dropping visit", "slug", slug)
	}
}

func getRealIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}
	return r.RemoteAddr
}

func getDeviceType(ua useragent.UserAgent) string {
	if ua.Tablet {
		return "tablet"
	}
	if ua.Mobile {
		return "mobile"
	}
	if ua.Desktop {
		return "desktop"
	}
	if ua.Bot {
		return "bot"
	}
	return "unknown"
}
