package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	configuration "github.com/karljucutan/trapigo/trapigo/pkg"
)

func main() {
	// 1. Setup backend proxy
	// Sample but make this dynamic later on. This is just a placeholder for now.
	appOneURL, err := url.Parse("http://app-one:8080")
	if err != nil {
		log.Fatalf("Invalid app-one URL: %v", err)
	}

	// 2. Instantiate an independent native proxy engine for each service
	proxyAppOne := httputil.NewSingleHostReverseProxy(appOneURL)

	// 3. Define the main API Gateway Router (Port 80)
	gatewayMux := http.NewServeMux()
	gatewayMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		// Traefik-Style Rule 1: Route /users paths to App One
		if strings.HasPrefix(r.URL.Path, "/api/v1/users") {
			log.Printf("[Trapigo] Proxying request %s to app-one", r.URL.Path)
			proxyAppOne.ServeHTTP(w, r)
			return
		}

		// Example Traefik-Style Rule 2: Route /orders paths to App Two
		// if strings.HasPrefix(r.URL.Path, "/api/v1/orders") {
		// 	log.Printf("[Trapigo] Proxying request %s to app-two", r.URL.Path)
		// 	proxyAppTwo.ServeHTTP(w, r)
		// 	return
		// }

		// Fallback for unmatched endpoints
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, "Trapigo Gateway: Service router matching rules failed.")
	})

	gatewayServer := &http.Server{
		Addr:              ":" + configuration.GetEnv("PORT", "80"),
		Handler:           gatewayMux,
		ReadHeaderTimeout: configuration.GetEnvDuration("READ_HEADER_TIMEOUT", 5, time.Second),
		ReadTimeout:       0,
		WriteTimeout:      0,
		IdleTimeout:       configuration.GetEnvDuration("IDLE_TIMEOUT", 180, time.Second),
	}

	// 3. Admin & Health Check Router (Private/Internal - Port 8080)
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"healthy"}`)
	})

	adminServer := &http.Server{
		Addr:              ":" + configuration.GetEnv("ADMIN_PORT", "8080"),
		Handler:           adminMux,
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	// 4. Start BOTH servers concurrently
	// Run the servers in a goroutine so it doesn't block main
	// Fire off the Gateway Server
	go func() {
		log.Printf("Gateway starting on %s", gatewayServer.Addr)
		if err := gatewayServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Gateway error: %v", err)
		}
	}()
	// Fire off the Admin API/Health Server
	go func() {
		log.Printf("Admin API/Health server starting on %s", adminServer.Addr)
		if err := adminServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Admin API/Health server error: %v", err)
		}
	}()

	// 5. Wait for an interrupt signal (Ctrl+C or kill command)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down servers gracefully...")

	// Allow existing requests 5 seconds to finish processing
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := gatewayServer.Shutdown(ctx); err != nil {
		log.Printf("Gateway forced to shutdown: %v", err)
	}
	if err := adminServer.Shutdown(ctx); err != nil {
		log.Printf("Admin forced to shutdown: %v", err)
	}

	log.Println("Servers stopped cleanly.")
}
