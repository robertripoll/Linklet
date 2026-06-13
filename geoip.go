package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// GeoIPService handles IP location lookups.
type GeoIPService struct {
	accountID  string
	licenseKey string
	client     *http.Client

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	country string
	city    string
	expires time.Time
}

const (
	geoIPCacheTTL  = 24 * time.Hour
	geoIPCacheSize = 10000
)

// NewGeoIPService creates a new instance of GeoIPService.
func NewGeoIPService() (*GeoIPService, error) {
	cfg := GetConfig()
	return &GeoIPService{
		accountID:  cfg.GeoIPAccountID,
		licenseKey: cfg.GeoIPLicenseKey,
		client:     &http.Client{Timeout: 5 * time.Second},
		cache:      make(map[string]cacheEntry),
	}, nil
}

// Close closes the database connection.
func (s *GeoIPService) Close() error {
	return nil
}

type maxMindResponse struct {
	City struct {
		Names map[string]string `json:"names"`
	} `json:"city"`
	Country struct {
		IsoCode string `json:"iso_code"`
	} `json:"country"`
}

// Lookup returns the country and city for a given IP address.
func (s *GeoIPService) Lookup(ipStr string) (country, city string) {
	if ipStr == "" {
		return "", ""
	}
	if net.ParseIP(ipStr) == nil {
		return "", ""
	}

	if e, ok := s.getCached(ipStr); ok {
		return e.country, e.city
	}

	url := fmt.Sprintf("https://geolite.info/geoip/v2.1/city/%s", ipStr)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", ""
	}

	req.SetBasicAuth(s.accountID, s.licenseKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", ""
	}

	var result maxMindResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", ""
	}

	country, city = result.Country.IsoCode, result.City.Names["en"]
	s.setCached(ipStr, country, city)
	return country, city
}

// getCached returns a non-expired cached result for ip, if present.
func (s *GeoIPService) getCached(ip string) (cacheEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.cache[ip]
	if !ok || time.Now().After(e.expires) {
		return cacheEntry{}, false
	}
	return e, true
}

// setCached stores a lookup result, evicting expired (and, if still at
// capacity, an arbitrary) entry to keep the cache bounded.
func (s *GeoIPService) setCached(ip, country, city string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.cache) >= geoIPCacheSize {
		now := time.Now()
		for k, e := range s.cache {
			if now.After(e.expires) {
				delete(s.cache, k)
			}
		}
		if len(s.cache) >= geoIPCacheSize {
			for k := range s.cache {
				delete(s.cache, k)
				break
			}
		}
	}

	s.cache[ip] = cacheEntry{country: country, city: city, expires: time.Now().Add(geoIPCacheTTL)}
}
