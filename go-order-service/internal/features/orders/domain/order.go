package domain

import "time"

type Order struct {
	ID               int64
	CustomerID       int64
	Status           string
	TotalAmountCents int64
	Items            []OrderItem
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewOrder(customerID int64, items []OrderItem) (Order, error) {
	if customerID <= 0 {
		return Order{}, ErrInvalidCustomerID
	}
	if len(items) == 0 {
		return Order{}, ErrOrderEmpty
	}

	now := time.Now().UTC()
	order := Order{
		CustomerID: customerID,
		Status:     "pending",
		Items:      make([]OrderItem, 0, len(items)),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	for idx := range items {
		item := items[idx]
		if item.ProductID <= 0 {
			return Order{}, ErrInvalidProductID
		}
		if item.Quantity <= 0 {
			return Order{}, ErrInvalidQuantity
		}
		if item.UnitPriceCents <= 0 {
			return Order{}, ErrInvalidPrice
		}

		item.SubtotalCents = item.Subtotal()
		item.CreatedAt = now
		item.UpdatedAt = now
		order.Items = append(order.Items, item)
		order.TotalAmountCents += item.SubtotalCents
	}

	return order, nil
}

func (o *Order) AddItem(item OrderItem) error {
	if item.ProductID <= 0 {
		return ErrInvalidProductID
	}
	if item.Quantity <= 0 {
		return ErrInvalidQuantity
	}
	if item.UnitPriceCents <= 0 {
		return ErrInvalidPrice
	}

	item.SubtotalCents = item.Subtotal()
	item.CreatedAt = time.Now().UTC()
	item.UpdatedAt = time.Now().UTC()
	o.Items = append(o.Items, item)
	o.TotalAmountCents += item.SubtotalCents
	o.UpdatedAt = time.Now().UTC()
	return nil
}

func (o *Order) RemoveItem(itemID int64) error {
	for i, item := range o.Items {
		if item.ID == itemID {
			removedSubtotal := item.SubtotalCents
			o.Items = append(o.Items[:i], o.Items[i+1:]...)
			o.TotalAmountCents -= removedSubtotal
			o.UpdatedAt = time.Now().UTC()
			return nil
		}
	}

	return ErrOrderItemNotFound
}

func (o *Order) UpdateItem(itemID int64, quantity *int, unitPriceCents *int64) error {
	for i := range o.Items {
		if o.Items[i].ID == itemID {
			if quantity == nil && unitPriceCents == nil {
				return nil
			}

			oldSubtotal := o.Items[i].SubtotalCents
			newQuantity := o.Items[i].Quantity
			newUnitPriceCents := o.Items[i].UnitPriceCents

			if quantity != nil {
				if *quantity <= 0 {
					return ErrInvalidQuantity
				}
				newQuantity = *quantity
			}
			if unitPriceCents != nil {
				if *unitPriceCents <= 0 {
					return ErrInvalidPrice
				}
				newUnitPriceCents = *unitPriceCents
			}

			o.Items[i].Quantity = newQuantity
			o.Items[i].UnitPriceCents = newUnitPriceCents
			o.Items[i].SubtotalCents = int64(newQuantity) * newUnitPriceCents
			o.Items[i].UpdatedAt = time.Now().UTC()
			o.TotalAmountCents += o.Items[i].SubtotalCents - oldSubtotal
			o.UpdatedAt = time.Now().UTC()
			return nil
		}
	}

	return ErrOrderItemNotFound
}

func (o *Order) UpdateStatus(status string) {
	o.Status = status
	o.UpdatedAt = time.Now().UTC()
}
