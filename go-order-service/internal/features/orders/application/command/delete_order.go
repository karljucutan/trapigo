package command

import (
	"context"
	"fmt"

	"go-order-service/internal/features/orders/domain"
	"go-order-service/internal/platform/transactionrepositories"
	"go-order-service/internal/platform/uow"
)

type DeleteOrderCommand struct {
	ID int64 `json:"id"`
}

type DeleteOrderHandler struct {
	uow uow.UnitOfWork[*transactionrepositories.TxRepositories]
}

func NewDeleteOrderHandler(u uow.UnitOfWork[*transactionrepositories.TxRepositories]) *DeleteOrderHandler {
	return &DeleteOrderHandler{uow: u}
}

func (h *DeleteOrderHandler) Handle(ctx context.Context, cmd DeleteOrderCommand) error {
	return h.uow.RunInTx(ctx, func(stores *transactionrepositories.TxRepositories) error {
		if _, err := stores.Orders.GetByID(ctx, cmd.ID); err != nil {
			return fmt.Errorf("%w: %d", domain.ErrOrderNotFound, cmd.ID)
		}
		return stores.Orders.Delete(ctx, cmd.ID)
	})
}
