package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/karljucutan/trapigo/trapigo/internal/features/core/domain"
	middleware "github.com/karljucutan/trapigo/trapigo/internal/features/middleware/transporthttp"
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
	setDefaultLogger()
	trapigoYamlPath := configuration.GetEnv("TRAPIGO_GATEWAY_CONFIG", "configs/trapigo-gateway.yaml")
	cfg, err := config.LoadConfig(trapigoYamlPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
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
	gatewayHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
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
			slog.Warn("no available backends",
				"path", req.URL.Path,
				"service", matchedRoute.ServiceName,
			)
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
			slog.Warn("no proxy configured",
				"backend", backend.Id,
				"path", req.URL.Path,
				"service", matchedRoute.ServiceName,
			)
			http.Error(rw, "No available backends", http.StatusServiceUnavailable)
			return
		}

		slog.Info("proxying request",
			"path", req.URL.Path,
			"service", matchedRoute.ServiceName,
			"backend", backend.Id,
		)
		matchedProxy.proxy.ServeHTTP(rw, req)
		slog.Info("served request",
			"path", req.URL.Path,
			"service", matchedRoute.ServiceName,
			"backend", backend.Id,
		)

		// TODO: Add basic reverse proxy ret
		// [ Incoming Client Request ]
		//            │
		//            ▼
		// ┌─────────────────────────────────────────────────────────┐
		// │                 TRAPIGO ENGINE (Port 80)              │
		// ├─────────────────────────────────────────────────────────┤
		// │ 1. LOGGING MIDDLEWARE                                   │
		// │    - Starts a high-resolution sub-millisecond timer.    │
		// │    - Captures the incoming request path/method.         │
		// ├─────────────────────────────────────────────────────────┤
		// │ 2. RATE-LIMIT MIDDLEWARE                                │
		// │    - Checks an in-memory map of Client IPs.             │
		// │    - Deducts tokens from their bucket.                  │
		// │    - Short-circuits with HTTP 429 if exceeded.          │
		// ├─────────────────────────────────────────────────────────┤
		// │ 3. AUTH MIDDLEWARE                                      │
		// │    - Validates credentials (token / API key).           │
		// │    - Rejects with HTTP 401 if missing/invalid.          │
		// ├─────────────────────────────────────────────────────────┤
		// │ 4. REVERSE-PROXY ROUTER                                 │
		// │    - Picks a healthy CRUD API backend via Round-Robin.  │
		// │    - Forwards request; streams response back.          │
		// └─────────────────────────────────────────────────────────┘
	})
	rateLimitPolicy := middleware.NewRateLimitPolicy(cfg.HTTP.RateLimit)
	gatewayMux.Handle("/", middleware.LoggingMiddleware(middleware.RateLimitMiddleware(rateLimitPolicy)(gatewayHandler)))

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
	}, nil
}

func (a *App) Run() {
	// Start BOTH servers concurrently
	// Run the servers in a goroutine so it doesn't block main
	// Fire off the Gateway Server
	go func() {
		slog.Info("gateway starting", "addr", a.GatewayServer.Addr)
		if err := a.GatewayServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("gateway error", "error", err)
		}
	}()
	// Fire off the Admin API/Health Server
	go func() {
		slog.Info("admin api server starting", "addr", a.AdminServer.Addr)
		if err := a.AdminServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("admin api server error", "error", err)
		}
	}()

	// Wait for an interrupt signal (Ctrl+C or kill command)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down servers gracefully")

	// Allow existing requests 5 seconds to finish processing
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Concurrently shutdown both servers
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := a.GatewayServer.Shutdown(ctx); err != nil {
			slog.Warn("gateway forced to shutdown", "error", err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := a.AdminServer.Shutdown(ctx); err != nil {
			slog.Warn("admin forced to shutdown", "error", err)
		}
	}()

	wg.Wait()
	slog.Info("servers stopped cleanly")
}

func setDefaultLogger() {
	slog.SetDefault(slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
	).With("app", "trapigo"))
}
