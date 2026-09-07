package domain

import "time"

type OrderItem struct {
	ID             int64     `json:"id"`
	OrderID        int64     `json:"order_id"`
	ProductID      int64     `json:"product_id"`
	Quantity       int       `json:"quantity"`
	UnitPriceCents int64     `json:"unit_price_cents"`
	SubtotalCents  int64     `json:"subtotal_cents"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (i OrderItem) Subtotal() int64 {
	return int64(i.Quantity) * i.UnitPriceCents
}
