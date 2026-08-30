package command

import (
	"context"
	"fmt"

	"go-order-service/internal/features/orders/domain"
	"go-order-service/internal/platform/transactionrepositories"
	"go-order-service/internal/platform/uow"
)

type UpdateOrderItemCommand struct {
	OrderID        int64
	ItemID         int64
	Quantity       *int
	UnitPriceCents *int64
}

// UpdateOrderItemHandler updates an order item within a transaction.
// It uses the generic UnitOfWork[*transactionrepositories.TxRepositories] pattern via RunInTx.
type UpdateOrderItemHandler struct {
	uow uow.UnitOfWork[*transactionrepositories.TxRepositories]
}

func NewUpdateOrderItemHandler(u uow.UnitOfWork[*transactionrepositories.TxRepositories]) *UpdateOrderItemHandler {
	return &UpdateOrderItemHandler{uow: u}
}

func (h *UpdateOrderItemHandler) Handle(ctx context.Context, cmd UpdateOrderItemCommand) (domain.Order, error) {
	var updated domain.Order

	err := h.uow.RunInTx(ctx, func(stores *transactionrepositories.TxRepositories) error {
		order, err := stores.Orders.GetByID(ctx, cmd.OrderID)
		if err != nil {
			return fmt.Errorf("%w: %d", domain.ErrOrderNotFound, cmd.OrderID)
		}

		if err := order.UpdateItem(cmd.ItemID, cmd.Quantity, cmd.UnitPriceCents); err != nil {
			return err
		}

		updated, err = stores.Orders.Update(ctx, order)
		return err
	})

	return updated, err
}
