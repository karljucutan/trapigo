package bootstrap

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

	"github.com/karljucutan/trapigo/trapigo/internal/platform/config"
	configuration "github.com/karljucutan/trapigo/trapigo/pkg"
)

type App struct {
	GatewayServer *http.Server
	AdminServer   *http.Server
}

type routeProxy struct {
	prefix string
	proxy  *httputil.ReverseProxy
}

func CreateApp() (*App, error) {
	// 1. Setup backend proxy
	trapigoYamlPath := configuration.GetEnv("trapigo-gateway", "configs/trapigo-gateway.yaml")
	cfg, err := config.LoadConfig(trapigoYamlPath)
	if err != nil {
		log.Fatal(err)
	}

	var routeProxies []routeProxy

	// Instantiate a reverse proxy for each router and its associated service's servers
	for routerName, router := range cfg.HTTP.Routers {
		service, ok := cfg.HTTP.Services[router.Service]
		if !ok || len(service.LoadBalancer.Servers) == 0 {
			return nil, fmt.Errorf("service %q for router %q not found", router.Service, routerName)
		}

		// Support for multiple servers in the load balancer for a service
		for _, server := range service.LoadBalancer.Servers {
			upstreamURL, err := url.Parse(server.URL)
			if err != nil {
				return nil, fmt.Errorf("invalid upstream URL %q for router %q: %w", server.URL, routerName, err)
			}

			routeProxies = append(routeProxies, routeProxy{
				prefix: router.PathPrefix,
				proxy:  httputil.NewSingleHostReverseProxy(upstreamURL),
			})
		}
	}

	// 2. Define the main API Gateway Router (Port 80)
	gatewayMux := http.NewServeMux()
	gatewayMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		for _, rp := range routeProxies {
			if strings.HasPrefix(r.URL.Path, rp.prefix) {
				log.Printf("[Trapigo] Proxying request %s to %s", r.URL.Path, rp.prefix)
				rp.proxy.ServeHTTP(w, r)
				return
			}
		}

		http.NotFound(w, r)
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

	return &App{
			GatewayServer: gatewayServer,
			AdminServer:   adminServer,
		},
		nil
}

func (a *App) Run() {
	// 4. Start BOTH servers concurrently
	// Run the servers in a goroutine so it doesn't block main
	// Fire off the Gateway Server
	go func() {
		log.Printf("Gateway starting on %s", a.GatewayServer.Addr)
		if err := a.GatewayServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Gateway error: %v", err)
		}
	}()
	// Fire off the Admin API/Health Server
	go func() {
		log.Printf("Admin API/Health server starting on %s", a.AdminServer.Addr)
		if err := a.AdminServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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

	if err := a.GatewayServer.Shutdown(ctx); err != nil {
		log.Printf("Gateway forced to shutdown: %v", err)
	}
	if err := a.AdminServer.Shutdown(ctx); err != nil {
		log.Printf("Admin forced to shutdown: %v", err)
	}

	log.Println("Servers stopped cleanly.")
}
