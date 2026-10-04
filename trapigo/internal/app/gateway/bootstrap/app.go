package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	configuration "github.com/karljucutan/trapigo/trapigo/pkg"
)

type App struct {
	GatewayServer *http.Server
	AdminServer   *http.Server
}

func CreateApp() (*App, error) {
	cfg, err := loadGatewayConfig()
	if err != nil {
		return nil, err
	}

	loadBalancer, routeProxies, err := buildLoadBalancer(cfg.HTTP)
	if err != nil {
		return nil, err
	}

	auth, err := buildAuthComponents(cfg)
	if err != nil {
		return nil, err
	}

	originMiddleware := buildOriginMiddlewareComponents(&cfg.AllowedOrigins, cfg.Auth.FrontendRedirectURL)

	gatewayHttpHandler := buildGatewayHttpHandler(loadBalancer, routeProxies, auth, originMiddleware, cfg.HTTP.RateLimit)
	adminHttpHandler := buildAdminHttpHandler()

	return &App{
		GatewayServer: &http.Server{
			Addr:              ":" + configuration.GetEnv("PORT", "80"),
			Handler:           gatewayHttpHandler,
			ReadHeaderTimeout: configuration.GetEnvDuration("READ_HEADER_TIMEOUT", 5, time.Second),
			ReadTimeout:       0,
			WriteTimeout:      0,
			IdleTimeout:       configuration.GetEnvDuration("IDLE_TIMEOUT", 180, time.Second),
		},
		AdminServer: &http.Server{
			Addr:              ":" + configuration.GetEnv("ADMIN_PORT", "8080"),
			Handler:           adminHttpHandler,
			ReadHeaderTimeout: 3 * time.Second,
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      5 * time.Second,
		},
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
