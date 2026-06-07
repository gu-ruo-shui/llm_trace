package main

import (
	"fmt"
	"llm_reverse/config"
	"llm_reverse/proxy"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func loadStartupConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	return cfg
}

func proxyWithBrowserNoiseGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldRedirectRootToDashboard(r) {
			http.Redirect(w, r, "/_ui/", http.StatusFound)
			return
		}

		if isBrowserOnlyNoisePath(r.URL.Path) {
			if strings.HasPrefix(r.URL.Path, "/cdn-cgi/") {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			http.NotFound(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func shouldRedirectRootToDashboard(r *http.Request) bool {
	return r.URL.Path == "/" && (r.Method == http.MethodGet || r.Method == http.MethodHead)
}

func isBrowserOnlyNoisePath(path string) bool {
	if strings.HasPrefix(path, "/cdn-cgi/") || strings.HasPrefix(path, "/assets/") {
		return true
	}

	switch path {
	case "/logo.png", "/setup/status":
		return true
	default:
		return false
	}
}

func main() {
	// Load configuration
	cfg := loadStartupConfig()
	var handler http.Handler
	var dbLogger *proxy.DatabaseLogger

	if cfg.UseDB {
		// Initialize database logger
		fmt.Println("Using database logging...")
		var err error
		dbLogger, err = proxy.NewDatabaseLogger(cfg.DBPath)
		if err != nil {
			log.Fatalf("Failed to initialize database logger: %v", err)
		}
		defer dbLogger.Close()

		// Create proxy handler with database logging
		handler = proxy.NewProxyHandlerDB(cfg.TargetURL, dbLogger)
		fmt.Printf("Logs will be stored in database: %s\n", cfg.DBPath)
	} else {
		// Initialize file logger
		fmt.Println("Using file logging...")
		logger, err := proxy.NewLogger(cfg.LogDir)
		if err != nil {
			log.Fatalf("Failed to initialize logger: %v", err)
		}
		defer logger.Close()

		// Create proxy handler with file logging
		handler = proxy.NewProxyHandler(cfg.TargetURL, logger)
		fmt.Printf("Logs will be written to: %s\n", cfg.LogDir)
	}

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/_ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/_ui/", http.StatusFound)
	})
	if cfg.UseDB {
		mux.Handle("/_ui/", proxy.NewDashboardHandler(dbLogger, cfg.TargetURL, cfg.DBPath))
	} else {
		mux.Handle("/_ui/", proxy.NewDashboardHandler(nil, cfg.TargetURL, cfg.DBPath))
	}
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("/", proxyWithBrowserNoiseGuard(handler))

	server := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: mux,
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Printf("LLM Proxy Server starting on %s\n", cfg.ServerPort)
		fmt.Printf("Proxying requests to: %s\n", cfg.TargetURL)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	fmt.Println("\nShutting down server...")
}
