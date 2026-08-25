package command

import (
	"context"
	"errors"

	"go-order-service/internal/features/orders/domain"
	"go-order-service/internal/platform/stores"
	"go-order-service/internal/platform/uow"
)

var errOrderNotFound = errors.New("order not found")

type CreateItemInput struct {
	ProductID      int64
	Quantity       int
	UnitPriceCents int64
}

type CreateOrderCommand struct {
	CustomerID int64
	Items      []CreateItemInput
}

type CreateOrderResponse struct {
	Order domain.Order
}

// CreateOrderHandler creates a new order within a transaction.
// It uses the generic UnitOfWork[*stores.TxRepositories] pattern via RunInTx.
type CreateOrderHandler struct {
	uow uow.UnitOfWork[*stores.TxRepositories]
}

func NewCreateOrderHandler(u uow.UnitOfWork[*stores.TxRepositories]) *CreateOrderHandler {
	return &CreateOrderHandler{uow: u}
}

func (h *CreateOrderHandler) Handle(ctx context.Context, cmd CreateOrderCommand) (domain.Order, error) {
	if cmd.CustomerID <= 0 {
		return domain.Order{}, domain.ErrInvalidCustomerID
	}
	if len(cmd.Items) == 0 {
		return domain.Order{}, domain.ErrOrderEmpty
	}

	domainItems := make([]domain.OrderItem, 0, len(cmd.Items))
	for _, item := range cmd.Items {
		domainItems = append(domainItems, domain.OrderItem{
			ProductID:      item.ProductID,
			Quantity:       item.Quantity,
			UnitPriceCents: item.UnitPriceCents,
		})
	}

	order, err := domain.NewOrder(cmd.CustomerID, domainItems)
	if err != nil {
		return domain.Order{}, err
	}

	var created domain.Order
	err = h.uow.RunInTx(ctx, func(stores *stores.TxRepositories) error {
		var err error
		created, err = stores.Orders.Create(ctx, order)
		return err
	})

	return created, err
}
