package command

import (
	"context"
	"fmt"

	"go-order-service/internal/features/orders/domain"
	"go-order-service/internal/platform/transactionrepositories"
	"go-order-service/internal/platform/uow"
)

type AddOrderItemCommand struct {
	OrderID        int64
	ProductID      int64
	Quantity       int
	UnitPriceCents int64
}

// AddOrderItemHandler adds an item to an order within a transaction.
// It uses the generic UnitOfWork[*transactionrepositories.TxRepositories] pattern via RunInTx.
type AddOrderItemHandler struct {
	uow uow.UnitOfWork[*transactionrepositories.TxRepositories]
}

func NewAddOrderItemHandler(u uow.UnitOfWork[*transactionrepositories.TxRepositories]) *AddOrderItemHandler {
	return &AddOrderItemHandler{uow: u}
}

func (h *AddOrderItemHandler) Handle(ctx context.Context, cmd AddOrderItemCommand) (domain.Order, error) {
	var updated domain.Order

	err := h.uow.RunInTx(ctx, func(stores *transactionrepositories.TxRepositories) error {
		order, err := stores.Orders.GetByID(ctx, cmd.OrderID)
		if err != nil {
			return fmt.Errorf("%w: %d", domain.ErrOrderNotFound, cmd.OrderID)
		}

		if err := order.AddItem(domain.OrderItem{
			ProductID:      cmd.ProductID,
			Quantity:       cmd.Quantity,
			UnitPriceCents: cmd.UnitPriceCents,
		}); err != nil {
			return err
		}

		updated, err = stores.Orders.Update(ctx, order)
		return err
	})

	return updated, err
}
