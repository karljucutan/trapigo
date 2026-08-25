package main

import (
	"database/sql"
	"log"
	"net/http"

	"go-order-service/internal/features/orders/application/command"
	"go-order-service/internal/features/orders/application/query"
	"go-order-service/internal/features/orders/infrastructure/repository"
	"go-order-service/internal/features/orders/transporthttp"
	"go-order-service/internal/platform/config"
	"go-order-service/internal/platform/database"
	"go-order-service/internal/platform/stores"
	"go-order-service/internal/platform/uow"
)

func main() {
	cfg := config.Load()

	db, err := database.Open(cfg.DSN)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	uow := uow.NewSQLUnitOfWork(db, func(tx *sql.Tx) *stores.TxRepositories {
		return &stores.TxRepositories{
			Orders: repository.NewPostgresOrderRepositoryWithTx(tx),
		}
	})

	repo := repository.NewPostgresOrderRepository(db)

	createOrderHandler := command.NewCreateOrderHandler(uow)
	addOrderItemHandler := command.NewAddOrderItemHandler(uow)
	updateOrderStatusHandler := command.NewUpdateOrderStatusHandler(uow)
	updateOrderItemHandler := command.NewUpdateOrderItemHandler(uow)
	removeOrderItemHandler := command.NewRemoveOrderItemHandler(uow)
	getOrderByIDHandler := query.NewGetOrderByIDHandler(repo)
	listOrdersHandler := query.NewListOrdersHandler(repo)
	deleteOrderHandler := command.NewDeleteOrderHandler(uow)
	handler := transporthttp.NewOrderHandler(
		createOrderHandler,
		addOrderItemHandler,
		updateOrderStatusHandler,
		updateOrderItemHandler,
		removeOrderItemHandler,
		getOrderByIDHandler,
		listOrdersHandler,
		deleteOrderHandler,
	)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	log.Printf("order service listening on %s", cfg.Addr())
	if err := http.ListenAndServe(cfg.Addr(), mux); err != nil {
		log.Fatalf("http server failed: %v", err)
	}
}
