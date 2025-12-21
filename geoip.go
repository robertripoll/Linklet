package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GeoIPService handles IP location lookups.
type GeoIPService struct {
	accountID  string
	licenseKey string
	client     *http.Client
}

// NewGeoIPService creates a new instance of GeoIPService.
func NewGeoIPService() (*GeoIPService, error) {
	cfg := GetConfig()
	return &GeoIPService{
		accountID:  cfg.GeoIPAccountID,
		licenseKey: cfg.GeoIPLicenseKey,
		client:     &http.Client{Timeout: 5 * time.Second},
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

	return result.Country.IsoCode, result.City.Names["en"]
}
