package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// IPBlacklist handles reading and checking blocked IP addresses.
type IPBlacklist struct {
	ips map[string]struct{}
	mu  sync.RWMutex
}

// NewIPBlacklist creates a new instance of IPBlacklist.
func NewIPBlacklist() *IPBlacklist {
	return &IPBlacklist{
		ips: make(map[string]struct{}),
	}
}

// Load reads the file (one IP per line) and populates the blacklist.
// Blank lines and lines starting with '#' are ignored. A missing file yields an empty blacklist.
func (b *IPBlacklist) Load(filename string) error {
	data, err := os.ReadFile(filename)
	if errors.Is(err, fs.ErrNotExist) {
		data, err = nil, nil
	}
	if err != nil {
		return err
	}

	logger := GetLogger()
	tmp := make(map[string]struct{})
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ip := net.ParseIP(line)
		if ip == nil {
			logger.Warn("Skipping invalid IP in blacklist", "file", filename, "value", line)
			continue
		}
		tmp[ip.String()] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	b.mu.Lock()
	b.ips = tmp
	b.mu.Unlock()
	return nil
}

// Watch polls filename every interval and reloads the blacklist when the file changes.
// Runs until ctx is cancelled.
func (b *IPBlacklist) Watch(ctx context.Context, filename string, interval time.Duration) {
	logger := GetLogger()

	info, err := os.Stat(filename)
	var lastMod time.Time
	if err == nil {
		lastMod = info.ModTime()
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var modTime time.Time
			info, err := os.Stat(filename)
			if err == nil {
				modTime = info.ModTime()
			} else if !errors.Is(err, fs.ErrNotExist) {
				logger.Warn("Could not stat blacklist file", "file", filename, "error", err)
				continue
			}
			if modTime.Equal(lastMod) {
				continue
			}
			lastMod = modTime
			if err := b.Load(filename); err != nil {
				logger.Error("Failed to reload blacklist file", "file", filename, "error", err)
			} else {
				logger.Info("Reloaded blacklist file", "file", filename)
			}
		}
	}
}

// Contains reports whether ip is blacklisted.
func (b *IPBlacklist) Contains(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	_, ok := b.ips[parsed.String()]
	return ok
}

// Middleware rejects requests from blacklisted IPs with HTTP 403.
// Blocked requests are intentionally not logged nor tracked.
func (b *IPBlacklist) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if b.Contains(getRealIP(r)) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
