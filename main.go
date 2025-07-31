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

	// Initialize logger
	logger, err := proxy.NewLogger(cfg.LogDir)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

	// Create proxy handler
	handler := proxy.NewProxyHandler(cfg.TargetURL, logger)

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
		fmt.Printf("Logs will be written to: %s\n", cfg.LogDir)
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	fmt.Println("\nShutting down server...")
}