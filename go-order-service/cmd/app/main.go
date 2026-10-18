package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"go-order-service/internal/features/orders/application/command"
	"go-order-service/internal/features/orders/application/query"
	"go-order-service/internal/features/orders/infrastructure/repository"
	ordertransporthttp "go-order-service/internal/features/orders/transporthttp"
	"go-order-service/internal/platform/config"
	"go-order-service/internal/platform/database"
	"go-order-service/internal/platform/telemetry"
	"go-order-service/internal/platform/transactionrepositories"
	"go-order-service/internal/platform/uow"

	"github.com/karljucutan/buildingblocks/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	cfg := config.Load()

	db, err := database.Open(cfg.DSN)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	shutdownTelemetry, err := initializeTelemetry(cfg.Env)
	if err != nil {
		closeDatabase(db)
		log.Fatalf("failed to initialize telemetry: %v", err)
	}

	server := &http.Server{
		Addr:    cfg.Addr(),
		Handler: buildHTTPHandler(db),
	}

	serveErr := serveUntilShutdown(server)
	flushTelemetry(shutdownTelemetry)
	closeDatabase(db)

	if serveErr != nil {
		log.Fatalf("order service stopped with error: %v", serveErr)
	}
	log.Print("order service stopped cleanly")
}

func initializeTelemetry(environment string) (func(context.Context) error, error) {
	samplingRatio, err := strconv.ParseFloat(getenv("TELEMETRY_SAMPLING_RATIO", "1.0"), 64)
	if err != nil {
		return nil, fmt.Errorf("parse TELEMETRY_SAMPLING_RATIO: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return telemetry.Init(ctx, telemetry.Config{
		ServiceName:   getenv("APP_NAME", "go-order-service"),
		Environment:   environment,
		SamplingRatio: samplingRatio,
	})
}

func buildHTTPHandler(db *sql.DB) http.Handler {
	uow := uow.NewSQLUnitOfWork(db, func(tx *sql.Tx) *transactionrepositories.TxRepositories {
		return &transactionrepositories.TxRepositories{
			Orders: repository.NewPostgresOrderRepositoryWithTx(tx),
		}
	})

	repo := repository.NewPostgresOrderRepository(db)

	handler := ordertransporthttp.NewOrderHandler(
		command.NewCreateOrderHandler(uow),
		command.NewAddOrderItemHandler(uow),
		command.NewUpdateOrderStatusHandler(uow),
		command.NewUpdateOrderItemHandler(uow),
		command.NewRemoveOrderItemHandler(uow),
		query.NewGetOrderByIDHandler(repo),
		query.NewListOrdersHandler(repo),
		command.NewDeleteOrderHandler(uow),
	)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	return otelhttp.NewHandler(middleware.LoggingMiddleware(mux), "orders.request")
}

func serveUntilShutdown(server *http.Server) error {
	// more modern approach compared to trapigo app.go graceful shutdown.
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("order service listening on %s", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-signalCtx.Done():
		log.Print("shutting down order service gracefully")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful HTTP shutdown failed; forcing close: %v", err)
		if closeErr := server.Close(); closeErr != nil {
			log.Printf("force close HTTP server: %v", closeErr)
		}
	}

	if err := <-serverErrors; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}
	return nil
}

func flushTelemetry(shutdown func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := shutdown(ctx); err != nil {
		log.Printf("telemetry shutdown failed: %v", err)
	}
}

func closeDatabase(db *sql.DB) {
	if err := db.Close(); err != nil {
		log.Printf("failed to close database: %v", err)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
