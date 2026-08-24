package domain

import "testing"

func TestOrderItemSubtotalUsesQuantityAndCents(t *testing.T) {
	item := OrderItem{Quantity: 3, UnitPriceCents: 199}

	if got := item.Subtotal(); got != 597 {
		t.Fatalf("subtotal mismatch: got %d want %d", got, 597)
	}
}
