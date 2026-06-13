package main

import (
	"encoding/json"
	"net"
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

const (
	// visitQueueSize bounds each buffered channel in the visit pipeline
	// (ingest -> GeoIP resolution -> file write).
	visitQueueSize = 100
	// geoIPWorkerCount is the number of concurrent GeoIP resolver goroutines.
	geoIPWorkerCount = 4
)

// VisitTracker handles recording visit statistics.
type VisitTracker struct {
	file     *os.File
	geoip    *GeoIPService
	logger   *Logger
	ch       chan Visit     // ingest -> GeoIP resolvers
	writeCh  chan Visit     // resolvers -> file writer
	wg       sync.WaitGroup // GeoIP resolvers
	writerWg sync.WaitGroup // file writer
}

// NewVisitTracker creates a new instance of VisitTracker.
func NewVisitTracker(geoip *GeoIPService) (*VisitTracker, error) {
	cfg := GetConfig()
	f, err := os.OpenFile(cfg.VisitsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	t := &VisitTracker{
		file:    f,
		geoip:   geoip,
		logger:  GetLogger(),
		ch:      make(chan Visit, visitQueueSize),
		writeCh: make(chan Visit, visitQueueSize),
	}

	t.writerWg.Add(1)
	go t.writer()

	// Resolve GeoIP concurrently so a slow lookup (network-bound, up to the
	// HTTP client timeout) cannot stall the file writer or back up the ingest
	// queue. Without GeoIP there is nothing to resolve, so a single passthrough
	// goroutine suffices.
	resolvers := 1
	if geoip != nil {
		resolvers = geoIPWorkerCount
	}
	t.wg.Add(resolvers)
	for i := 0; i < resolvers; i++ {
		go t.resolver()
	}

	return t, nil
}

// Close drains the visit pipeline and closes the underlying file.
func (t *VisitTracker) Close() error {
	close(t.ch)
	t.wg.Wait() // resolvers have drained ch and finished sending to writeCh
	close(t.writeCh)
	t.writerWg.Wait()
	return t.file.Close()
}

// resolver enriches visits with GeoIP data and forwards them to the writer.
// Multiple resolvers run concurrently so network latency is not serialized.
func (t *VisitTracker) resolver() {
	defer t.wg.Done()
	for v := range t.ch {
		if t.geoip != nil {
			v.Country, v.City = t.geoip.Lookup(v.IP)
		}
		t.writeCh <- v
	}
}

// writer is the sole owner of the visits file, serializing all writes.
func (t *VisitTracker) writer() {
	defer t.writerWg.Done()
	encoder := json.NewEncoder(t.file)
	for v := range t.writeCh {
		if err := encoder.Encode(v); err != nil {
			t.logger.Error("Failed to write visit", "error", err)
		}
	}
}

// Record adds a new visit to the processing queue.
func (t *VisitTracker) Record(r *http.Request, slug string) {
	uaStr := r.UserAgent()
	ua := useragent.Parse(uaStr)

	v := Visit{
		Time:        time.Now(),
		Slug:        slug,
		IP:          getRealIP(r),
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
	}

	select {
	case t.ch <- v:
	default:
		t.logger.Warn("Visit queue full, dropping visit", "slug", slug)
	}
}

// getRealIP returns the connecting client's IP address.
//
// The service runs exclusively behind a Cloudflare Tunnel, with no port
// reachable except through the tunnel. Cloudflare overwrites CF-Connecting-IP
// on every request with the real client address, so it is the only trustworthy
// source here. Client-supplied forwarding headers (X-Forwarded-For, X-Real-IP)
// are intentionally NOT trusted: Cloudflare does not strip them, so an attacker
// could forge them to spoof their identity and bypass the per-IP rate limiter.
func getRealIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
		if net.ParseIP(ip) != nil {
			return ip
		}
	}
	// Fallback: should not occur behind the tunnel, but avoids an empty key
	// (which would collapse all such traffic into a single rate-limit bucket).
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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
