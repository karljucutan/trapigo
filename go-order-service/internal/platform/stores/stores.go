package stores

import (
	"go-order-service/internal/features/orders/domain"
)

// Stores encapsulates all repository interfaces for a single transaction.
// In DDD/VSA, this represents the persistence layer contract for a slice.
// It allows the generic UnitOfWork[T] to work with any set of repositories.
// T will typically be a specific implementation like Stores with concrete repository types.
type Stores struct {
	Orders domain.OrderRepository
}
