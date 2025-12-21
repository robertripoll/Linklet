package main

import (
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
			Port:       getEnv("PORT"),
			DataFile:   "urls.json",
			VisitsFile: "visits.jsonl",
			GeoIPAccountID:  getEnv("GEOIP_ACCOUNT_ID"),
			GeoIPLicenseKey: getEnv("GEOIP_LICENSE_KEY"),
		}
	})
	return instance
}

// getEnv retrieves an environment variable or returns a default value.
func getEnv(key string) (string) {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	panic("Environment variable not defined: " + key)
}
