package bootstrap

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	authgateway "github.com/karljucutan/trapigo/trapigo/internal/features/auth/gateway"
	"github.com/karljucutan/trapigo/trapigo/internal/features/core/domain"
	middleware "github.com/karljucutan/trapigo/trapigo/internal/features/middleware/transporthttp"
	"github.com/karljucutan/trapigo/trapigo/internal/platform/config"
	configuration "github.com/karljucutan/trapigo/trapigo/pkg"
)

type routeProxy struct {
	upstreamURL string
	proxy       *httputil.ReverseProxy
}

func loadGatewayConfig() (*config.Config, error) {
	setDefaultLogger()
	trapigoYamlPath := configuration.GetEnv("TRAPIGO_GATEWAY_CONFIG", "configs/trapigo-gateway.yaml")
	cfg, err := config.LoadConfig(trapigoYamlPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		return nil, err
	}

	return cfg, nil
}

func buildLoadBalancer(httpCfg config.HTTPConfig) (*domain.LoadBalancer, map[string]*routeProxy, error) {
	loadBalancer := domain.NewLoadBalancer()
	routeProxies := make(map[string]*routeProxy)

	for routerName, router := range httpCfg.Routers {
		service, ok := httpCfg.Services[router.Service]
		if !ok || len(service.LoadBalancer.Servers) == 0 {
			return nil, nil, fmt.Errorf("service %q for router %q not found", router.Service, routerName)
		}

		lbRouter := domain.NewRouter(
			router.PathPrefix,
			router.Service,
			domain.NewBackendPool(),
		)
		loadBalancer.AddRouter(lbRouter)

		for _, server := range service.LoadBalancer.Servers {
			upstreamURL, err := url.Parse(server.URL)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid upstream URL %q for router %q: %w", server.URL, routerName, err)
			}
			lbRouter.BackendPool.Add(domain.NewBackend(server.URL, upstreamURL))
			routeProxies[server.URL] = &routeProxy{
				upstreamURL: server.URL,
				proxy:       httputil.NewSingleHostReverseProxy(upstreamURL),
			}
		}
	}

	return loadBalancer, routeProxies, nil
}

func buildGatewayHttpHandler(loadBalancer *domain.LoadBalancer, routeProxies map[string]*routeProxy, auth *authComponents, rateLimitCfg *config.RateLimit) http.Handler {
	proxyHandler := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
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

		proxy, ok := routeProxies[backend.Id]
		if !ok || proxy == nil || proxy.proxy == nil {
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
		proxy.proxy.ServeHTTP(rw, req)
		slog.Info("served request",
			"path", req.URL.Path,
			"service", matchedRoute.ServiceName,
			"backend", backend.Id,
		)

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

	gatewayMux := http.NewServeMux()
	gatewayMux.Handle("/web/auth/login", http.HandlerFunc(auth.handler.HandleLogin))
	gatewayMux.Handle("/web/auth/callback", http.HandlerFunc(auth.handler.HandleCallback))
	gatewayMux.Handle("/web/auth/logout", http.HandlerFunc(auth.handler.HandleLogout))
	gatewayMux.Handle("/web/auth/me", auth.middleware(http.HandlerFunc(auth.handler.HandleMe)))
	gatewayMux.Handle("/api/", auth.middleware(authgateway.AddAuthorizationHeader(proxyHandler)))
	gatewayMux.Handle("/", proxyHandler)

	rateLimitPolicy := middleware.NewRateLimitPolicy(rateLimitCfg)
	return middleware.LoggingMiddleware(middleware.RateLimitMiddleware(rateLimitPolicy)(gatewayMux))
}

func buildAdminHttpHandler() http.Handler {
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"healthy"}`)
	})
	return adminMux
}
