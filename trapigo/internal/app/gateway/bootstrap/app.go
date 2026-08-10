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

	"github.com/karljucutan/trapigo/trapigo/internal/features/core/domain"
	"github.com/karljucutan/trapigo/trapigo/internal/platform/config"
	configuration "github.com/karljucutan/trapigo/trapigo/pkg"
)

type App struct {
	GatewayServer *http.Server
	AdminServer   *http.Server
}

type routeProxy struct {
	upstreamURL string
	proxy       *httputil.ReverseProxy
}

func CreateApp() (*App, error) {
	trapigoYamlPath := configuration.GetEnv("TRAPIGO_GATEWAY_CONFIG", "configs/trapigo-gateway.yaml")
	cfg, err := config.LoadConfig(trapigoYamlPath)
	if err != nil {
		log.Fatal(err)
	}

	var routeProxies []routeProxy
	loadBalancer := domain.NewLoadBalancer()

	for routerName, router := range cfg.HTTP.Routers {
		service, ok := cfg.HTTP.Services[router.Service]
		if !ok || len(service.LoadBalancer.Servers) == 0 {
			return nil, fmt.Errorf("service %q for router %q not found", router.Service, routerName)
		}

		loadBalancer.AddRouter(domain.NewRouter(
			router.PathPrefix,
			router.Service,
			domain.NewBackendPool(),
		))

		for _, server := range service.LoadBalancer.Servers {
			upstreamURL, err := url.Parse(server.URL)
			if err != nil {
				return nil, fmt.Errorf("invalid upstream URL %q for router %q: %w", server.URL, routerName, err)
			}

			for _, lbRouter := range loadBalancer.Routers {
				if lbRouter.ServiceName == router.Service {
					lbRouter.BackendPool.Add(domain.NewBackend(server.URL, upstreamURL))
					break
				}
			}

			routeProxies = append(routeProxies, routeProxy{
				upstreamURL: server.URL,
				proxy:       httputil.NewSingleHostReverseProxy(upstreamURL),
			})
		}
	}

	gatewayMux := http.NewServeMux()
	gatewayMux.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		var matchedRoute *domain.Router
		for _, route := range loadBalancer.Routers {
			if strings.HasPrefix(req.URL.Path, route.PathPrefix) {
				matchedRoute = route
				break
			}
		}

		if matchedRoute == nil {
			http.NotFound(rw, req)
			return
		}

		backend := matchedRoute.BackendPool.RoundRobinAtomic()
		if backend == nil {
			log.Printf("[Trapigo] No available backends for request %s to service %s", req.URL.Path, matchedRoute.ServiceName)
			http.Error(rw, "No available backends", http.StatusServiceUnavailable)
			return
		}

		var matchedProxy routeProxy
		for _, rp := range routeProxies {
			if rp.upstreamURL == backend.Id {
				matchedProxy = rp
				break
			}
		}

		if matchedProxy.proxy == nil {
			log.Printf("[Trapigo] No proxy configured for backend %s while serving request %s to service %s", backend.Id, req.URL.Path, matchedRoute.ServiceName)
			http.Error(rw, "No available backends", http.StatusServiceUnavailable)
			return
		}

		log.Printf("[Trapigo] Proxying request %s to service %s via %s", req.URL.Path, matchedRoute.ServiceName, backend.Id)
		matchedProxy.proxy.ServeHTTP(rw, req)
		log.Printf("[Trapigo] Served request %s to service %s via %s", req.URL.Path, matchedRoute.ServiceName, backend.Id)
	})

	gatewayServer := &http.Server{
		Addr:              ":" + configuration.GetEnv("PORT", "80"),
		Handler:           gatewayMux,
		ReadHeaderTimeout: configuration.GetEnvDuration("READ_HEADER_TIMEOUT", 5, time.Second),
		ReadTimeout:       0,
		WriteTimeout:      0,
		IdleTimeout:       configuration.GetEnvDuration("IDLE_TIMEOUT", 180, time.Second),
	}

	// Admin & Health Check Router (Private/Internal - Port 8080)
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
	// Start BOTH servers concurrently
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

	// Wait for an interrupt signal (Ctrl+C or kill command)
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
