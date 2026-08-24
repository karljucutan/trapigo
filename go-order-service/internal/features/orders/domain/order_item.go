package domain

import "time"

type OrderItem struct {
	ID             int64
	OrderID        int64
	ProductID      int64
	Quantity       int
	UnitPriceCents int64
	SubtotalCents  int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (i OrderItem) Subtotal() int64 {
	return int64(i.Quantity) * i.UnitPriceCents
}
