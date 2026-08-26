package command

import (
	"context"
	"fmt"

	"go-order-service/internal/features/orders/domain"
	"go-order-service/internal/platform/stores"
	"go-order-service/internal/platform/uow"
)

type RemoveOrderItemCommand struct {
	OrderID int64
	ItemID  int64
}

// RemoveOrderItemHandler removes an item from an order within a transaction.
// It uses the generic UnitOfWork[*stores.TxRepositories] pattern via RunInTx.
type RemoveOrderItemHandler struct {
	uow uow.UnitOfWork[*stores.TxRepositories]
}

func NewRemoveOrderItemHandler(u uow.UnitOfWork[*stores.TxRepositories]) *RemoveOrderItemHandler {
	return &RemoveOrderItemHandler{uow: u}
}

func (h *RemoveOrderItemHandler) Handle(ctx context.Context, cmd RemoveOrderItemCommand) (domain.Order, error) {
	var updated domain.Order

	err := h.uow.RunInTx(ctx, func(stores *stores.TxRepositories) error {
		order, err := stores.Orders.GetByID(ctx, cmd.OrderID)
		if err != nil {
			return fmt.Errorf("%w: %d", domain.ErrOrderNotFound, cmd.OrderID)
		}

		if err := order.RemoveItem(cmd.ItemID); err != nil {
			return err
		}

		updated, err = stores.Orders.Update(ctx, order)
		return err
	})

	return updated, err
}
