package main

import (
	"log"
	"os"
	"sync"
)

// Config holds the application configuration.
type Config struct {
	Port       string
	DataFile   string
	VisitsFile string
	GeoIPAccountID  string
	GeoIPLicenseKey string
}

var (
	instance *Config
	once     sync.Once
)

// GetConfig returns the singleton configuration instance.
func GetConfig() *Config {
	once.Do(func() {
		instance = &Config{
			Port:            getEnv("PORT", "8080"),
			DataFile:        "urls.json",
			VisitsFile:      "visits.jsonl",
			GeoIPAccountID:  os.Getenv("GEOIP_ACCOUNT_ID"),
			GeoIPLicenseKey: os.Getenv("GEOIP_LICENSE_KEY"),
		}
	})
	return instance
}

// getEnv returns the value of key, falling back to def. Exits if key is unset and def is empty.
func getEnv(key, def string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	if def != "" {
		return def
	}
	log.Fatalf("required environment variable %q is not set", key)
	return ""
}
