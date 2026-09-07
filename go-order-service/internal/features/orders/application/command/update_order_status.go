package command

import (
	"context"
	"errors"
	"fmt"

	"go-order-service/internal/features/orders/domain"
	"go-order-service/internal/platform/transactionrepositories"
	"go-order-service/internal/platform/uow"
)

type UpdateOrderStatusCommand struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

// UpdateOrderStatusHandler updates the status of an order within a transaction.
// It uses the generic UnitOfWork[*transactionrepositories.TxRepositories] pattern via RunInTx.
type UpdateOrderStatusHandler struct {
	uow uow.UnitOfWork[*transactionrepositories.TxRepositories]
}

func NewUpdateOrderStatusHandler(u uow.UnitOfWork[*transactionrepositories.TxRepositories]) *UpdateOrderStatusHandler {
	return &UpdateOrderStatusHandler{uow: u}
}

func (h *UpdateOrderStatusHandler) Handle(ctx context.Context, cmd UpdateOrderStatusCommand) (domain.Order, error) {
	if cmd.Status == "" {
		return domain.Order{}, errors.New("status is required")
	}

	var updated domain.Order

	err := h.uow.RunInTx(ctx, func(stores *transactionrepositories.TxRepositories) error {
		order, err := stores.Orders.GetByID(ctx, cmd.ID)
		if err != nil {
			return fmt.Errorf("%w: %d", domain.ErrOrderNotFound, cmd.ID)
		}

		order.UpdateStatus(cmd.Status)

		updated, err = stores.Orders.Update(ctx, order)
		return err
	})

	return updated, err
}
