package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	cfg := GetConfig()

	logger := GetLogger()

	store := NewURLStore()
	if err := store.Load(cfg.DataFile); err != nil {
		logger.Error("Error loading URLs", "error", err)
	}

	geoip, err := NewGeoIPService()
	if err != nil {
		logger.Warn("Could not initialize GeoIP service", "error", err)
	} else {
		defer geoip.Close()
	}

	tracker, err := NewVisitTracker(geoip)
	if err != nil {
		logger.Error("Error initializing visit tracker", "error", err)
		os.Exit(1)
	}
	defer tracker.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Security: Add headers to prevent clickjacking and MIME sniffing
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		slug := r.URL.Path[1:]

		if url, ok := store.Get(slug); ok {
			// Security: Prevent XSS by ensuring the URL scheme is http or https
			// This prevents "javascript:" or "data:" URIs from being executed via redirect.
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				logger.Warn("Blocked unsafe redirect scheme", "slug", slug, "url", url)
				http.Error(w, "Invalid redirect URL", http.StatusBadRequest)
				return
			}

			logger.Info("Redirecting", "slug", slug, "url", url)
			tracker.Record(r, slug)
			http.Redirect(w, r, url, http.StatusFound)
			return
		} else if slug == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("<!DOCTYPE html><html><head></head><body></body></html>")); err != nil {
				logger.Error("Error writing response", "error", err)
			}
			return
		}

		logger.Debug("Slug not found", "slug", slug)
		http.NotFound(w, r)
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		logger.Info("Server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Error starting server", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	logger.Info("Server exiting")
}
