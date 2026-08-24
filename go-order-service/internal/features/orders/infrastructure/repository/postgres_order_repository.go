package repository

import (
	"context"
	"database/sql"
	"fmt"

	"go-order-service/internal/features/orders/domain"
	"go-order-service/internal/platform/database"
)

type PostgresOrderRepository struct {
	runner database.SQLRunner
}

func NewPostgresOrderRepository(db *sql.DB) *PostgresOrderRepository {
	return &PostgresOrderRepository{runner: db}
}

func NewPostgresOrderRepositoryWithTx(tx *sql.Tx) *PostgresOrderRepository {
	return &PostgresOrderRepository{runner: tx}
}

func (r *PostgresOrderRepository) Create(ctx context.Context, order domain.Order) (domain.Order, error) {
	row := r.runner.QueryRowContext(ctx, `
        INSERT INTO orders (customer_id, status, total_amount_cents)
        VALUES ($1, $2, $3)
        RETURNING id, customer_id, status, total_amount_cents, created_at, updated_at
    `, order.CustomerID, order.Status, order.TotalAmountCents)

	var created domain.Order
	if err := row.Scan(&created.ID, &created.CustomerID, &created.Status, &created.TotalAmountCents, &created.CreatedAt, &created.UpdatedAt); err != nil {
		return domain.Order{}, err
	}

	created.Items = make([]domain.OrderItem, 0, len(order.Items))
	for i := range order.Items {
		item := order.Items[i]
		row := r.runner.QueryRowContext(ctx, `
            INSERT INTO order_items (order_id, product_id, quantity, unit_price_cents, subtotal_cents, created_at, updated_at)
            VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
            RETURNING id, order_id, product_id, quantity, unit_price_cents, subtotal_cents, created_at, updated_at
        `, created.ID, item.ProductID, item.Quantity, item.UnitPriceCents, item.SubtotalCents)

		var saved domain.OrderItem
		if err := row.Scan(&saved.ID, &saved.OrderID, &saved.ProductID, &saved.Quantity, &saved.UnitPriceCents, &saved.SubtotalCents, &saved.CreatedAt, &saved.UpdatedAt); err != nil {
			return domain.Order{}, err
		}
		created.Items = append(created.Items, saved)
	}

	created.CustomerID = order.CustomerID
	created.Status = order.Status
	created.TotalAmountCents = order.TotalAmountCents
	created.UpdatedAt = order.UpdatedAt
	return created, nil
}

func (r *PostgresOrderRepository) GetByID(ctx context.Context, id int64) (domain.Order, error) {
	row := r.runner.QueryRowContext(ctx, `
        SELECT id, customer_id, status, total_amount_cents, created_at, updated_at
        FROM orders
        WHERE id = $1
    `, id)

	var order domain.Order
	if err := row.Scan(&order.ID, &order.CustomerID, &order.Status, &order.TotalAmountCents, &order.CreatedAt, &order.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return domain.Order{}, fmt.Errorf("order %d not found", id)
		}
		return domain.Order{}, err
	}

	items, err := r.listItemsByOrderID(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}

	order.Items = items
	return order, nil
}

func (r *PostgresOrderRepository) List(ctx context.Context) ([]domain.Order, error) {
	rows, err := r.runner.QueryContext(ctx, `
        SELECT id, customer_id, status, total_amount_cents, created_at, updated_at
        FROM orders
        ORDER BY id ASC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]domain.Order, 0)
	for rows.Next() {
		var order domain.Order
		if err := rows.Scan(&order.ID, &order.CustomerID, &order.Status, &order.TotalAmountCents, &order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, err
		}
		items, err := r.listItemsByOrderID(ctx, order.ID)
		if err != nil {
			return nil, err
		}
		order.Items = items
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func (r *PostgresOrderRepository) Update(ctx context.Context, order domain.Order) (domain.Order, error) {
	_, err := r.runner.ExecContext(ctx, `
        UPDATE orders
        SET customer_id = $1, status = $2, total_amount_cents = $3, updated_at = NOW()
        WHERE id = $4
    `, order.CustomerID, order.Status, order.TotalAmountCents, order.ID)
	if err != nil {
		return domain.Order{}, err
	}

	existingItems, err := r.listItemsByOrderID(ctx, order.ID)
	if err != nil {
		return domain.Order{}, err
	}
	seen := make(map[int64]bool, len(order.Items))
	for i := range order.Items {
		item := order.Items[i]
		if item.ID == 0 {
			row := r.runner.QueryRowContext(ctx, `
                INSERT INTO order_items (order_id, product_id, quantity, unit_price_cents, subtotal_cents, created_at, updated_at)
                VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
                RETURNING id, order_id, product_id, quantity, unit_price_cents, subtotal_cents, created_at, updated_at
            `, order.ID, item.ProductID, item.Quantity, item.UnitPriceCents, item.SubtotalCents)
			var saved domain.OrderItem
			if err := row.Scan(&saved.ID, &saved.OrderID, &saved.ProductID, &saved.Quantity, &saved.UnitPriceCents, &saved.SubtotalCents, &saved.CreatedAt, &saved.UpdatedAt); err != nil {
				return domain.Order{}, err
			}
			order.Items[i] = saved
			seen[saved.ID] = true
			continue
		}

		seen[item.ID] = true
		_, err = r.runner.ExecContext(ctx, `
            UPDATE order_items
            SET product_id = $1, quantity = $2, unit_price_cents = $3, subtotal_cents = $4, updated_at = NOW()
            WHERE id = $5 AND order_id = $6
        `, item.ProductID, item.Quantity, item.UnitPriceCents, item.SubtotalCents, item.ID, order.ID)
		if err != nil {
			return domain.Order{}, err
		}
	}

	for _, item := range existingItems {
		if !seen[item.ID] {
			_, err = r.runner.ExecContext(ctx, `DELETE FROM order_items WHERE id = $1 AND order_id = $2`, item.ID, order.ID)
			if err != nil {
				return domain.Order{}, err
			}
		}
	}

	return order, nil
}

func (r *PostgresOrderRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.runner.ExecContext(ctx, `DELETE FROM orders WHERE id = $1`, id)
	return err
}

func (r *PostgresOrderRepository) listItemsByOrderID(ctx context.Context, orderID int64) ([]domain.OrderItem, error) {
	return r.listItemsByOrderIDTx(ctx, r.runner, orderID)
}

func (r *PostgresOrderRepository) listItemsByOrderIDTx(ctx context.Context, runner database.SQLRunner, orderID int64) ([]domain.OrderItem, error) {
	rows, err := runner.QueryContext(ctx, `
        SELECT id, order_id, product_id, quantity, unit_price_cents, subtotal_cents, created_at, updated_at
        FROM order_items
        WHERE order_id = $1
        ORDER BY id ASC
    `, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.OrderItem, 0)
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.Quantity, &item.UnitPriceCents, &item.SubtotalCents, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
