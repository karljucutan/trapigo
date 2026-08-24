package domain

import (
	"testing"
	"time"
)

func TestNewOrderComputesTotalFromItems(t *testing.T) {
	items := []OrderItem{
		{ProductID: 1001, Quantity: 2, UnitPriceCents: 2500},
		{ProductID: 1002, Quantity: 1, UnitPriceCents: 1500},
	}

	order, err := NewOrder(42, items)
	if err != nil {
		t.Fatalf("NewOrder returned error: %v", err)
	}

	if order.CustomerID != 42 {
		t.Fatalf("customer id mismatch: got %d want %d", order.CustomerID, 42)
	}

	if order.TotalAmountCents != 6500 {
		t.Fatalf("total cents mismatch: got %d want %d", order.TotalAmountCents, 6500)
	}

	if len(order.Items) != 2 {
		t.Fatalf("item count mismatch: got %d want %d", len(order.Items), 2)
	}

	if order.Status != "pending" {
		t.Fatalf("status mismatch: got %q want %q", order.Status, "pending")
	}

	if order.CreatedAt.IsZero() {
		t.Fatal("created_at should be set")
	}

	if order.UpdatedAt.IsZero() {
		t.Fatal("updated_at should be set")
	}
}

func TestNewOrderRejectsEmptyItems(t *testing.T) {
	_, err := NewOrder(42, nil)
	if err == nil {
		t.Fatal("expected error for empty items")
	}
}

func TestOrderAddItemRecalculatesTotals(t *testing.T) {
	order, err := NewOrder(7, []OrderItem{{ProductID: 1, Quantity: 1, UnitPriceCents: 500}})
	if err != nil {
		t.Fatalf("NewOrder returned error: %v", err)
	}

	if err := order.AddItem(OrderItem{ProductID: 2, Quantity: 2, UnitPriceCents: 250}); err != nil {
		t.Fatalf("AddItem returned error: %v", err)
	}

	if order.TotalAmountCents != 1000 {
		t.Fatalf("total cents mismatch after add: got %d want %d", order.TotalAmountCents, 1000)
	}

	if order.UpdatedAt.Before(time.Now().Add(-time.Second)) {
		t.Fatal("updated_at should be refreshed when item added")
	}
}

func TestNewOrderRejectsInvalidCustomerID(t *testing.T) {
	_, err := NewOrder(0, []OrderItem{{ProductID: 1, Quantity: 1, UnitPriceCents: 100}})
	if err == nil {
		t.Fatal("expected error for invalid customer id")
	}
}

func TestNewOrderRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		items []OrderItem
	}{
		{name: "invalid product id", items: []OrderItem{{ProductID: 0, Quantity: 1, UnitPriceCents: 100}}},
		{name: "invalid quantity", items: []OrderItem{{ProductID: 1, Quantity: 0, UnitPriceCents: 100}}},
		{name: "invalid price", items: []OrderItem{{ProductID: 1, Quantity: 1, UnitPriceCents: 0}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewOrder(42, tc.items)
			if err == nil {
				t.Fatal("expected error for invalid order item")
			}
		})
	}
}

func TestOrderAddItemRejectsInvalidValues(t *testing.T) {
	order, err := NewOrder(7, []OrderItem{{ProductID: 1, Quantity: 1, UnitPriceCents: 500}})
	if err != nil {
		t.Fatalf("NewOrder returned error: %v", err)
	}

	tests := []struct {
		name string
		item OrderItem
	}{
		{name: "invalid product id", item: OrderItem{ProductID: 0, Quantity: 1, UnitPriceCents: 100}},
		{name: "invalid quantity", item: OrderItem{ProductID: 2, Quantity: 0, UnitPriceCents: 100}},
		{name: "invalid price", item: OrderItem{ProductID: 2, Quantity: 1, UnitPriceCents: 0}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := order.AddItem(tc.item); err == nil {
				t.Fatal("expected error for invalid order item")
			}
		})
	}
}

func TestOrderUpdateStatusSetsStatusAndRefreshesTimestamp(t *testing.T) {
	order, err := NewOrder(12, []OrderItem{{ProductID: 9, Quantity: 2, UnitPriceCents: 300}})
	if err != nil {
		t.Fatalf("NewOrder returned error: %v", err)
	}

	before := order.UpdatedAt
	order.UpdateStatus("paid")

	if order.Status != "paid" {
		t.Fatalf("status mismatch: got %q want %q", order.Status, "paid")
	}

	if !order.UpdatedAt.After(before) && !order.UpdatedAt.Equal(before) {
		t.Fatal("updated_at should be refreshed when status changes")
	}
}

func TestOrderRemoveItemUpdatesTotal(t *testing.T) {
	order, err := NewOrder(12, []OrderItem{{ID: 10, ProductID: 9, Quantity: 2, UnitPriceCents: 300}})
	if err != nil {
		t.Fatalf("NewOrder returned error: %v", err)
	}

	if err := order.RemoveItem(10); err != nil {
		t.Fatalf("RemoveItem returned error: %v", err)
	}

	if len(order.Items) != 0 {
		t.Fatalf("expected order to have no items, got %d", len(order.Items))
	}

	if order.TotalAmountCents != 0 {
		t.Fatalf("expected total to be 0, got %d", order.TotalAmountCents)
	}
}

func TestOrderUpdateItemRecalculatesSubtotalAndTotal(t *testing.T) {
	order, err := NewOrder(12, []OrderItem{{ID: 10, ProductID: 9, Quantity: 2, UnitPriceCents: 300}})
	if err != nil {
		t.Fatalf("NewOrder returned error: %v", err)
	}

	if err := order.UpdateItem(10, 5, 400); err != nil {
		t.Fatalf("UpdateItem returned error: %v", err)
	}

	if order.Items[0].Quantity != 5 {
		t.Fatalf("quantity mismatch: got %d want %d", order.Items[0].Quantity, 5)
	}

	if order.Items[0].SubtotalCents != 2000 {
		t.Fatalf("subtotal mismatch: got %d want %d", order.Items[0].SubtotalCents, 2000)
	}

	if order.TotalAmountCents != 2000 {
		t.Fatalf("total mismatch: got %d want %d", order.TotalAmountCents, 2000)
	}
}
