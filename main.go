package main

import (
	"fmt"
	"llm_reverse/config"
	"llm_reverse/proxy"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Load configuration
	cfg := config.Load()

	var handler http.Handler
	
	if cfg.UseDB {
		// Initialize database logger
		fmt.Println("Using database logging...")
		dbLogger, err := proxy.NewDatabaseLogger(cfg.DBPath)
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
	mux.Handle("/", handler)

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