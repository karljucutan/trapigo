package transactionrepositories

import (
	"go-order-service/internal/features/orders/domain"
)

// TxRepositories encapsulates all repository interfaces for a single transaction.
// In DDD/VSA, this represents the persistence layer contract for a slice.
// It allows the generic UnitOfWork[T] to work with any set of repositories.
// T will typically be a specific implementation like TxRepositories with concrete repository types.
type TxRepositories struct {
	Orders domain.OrderRepository
}
