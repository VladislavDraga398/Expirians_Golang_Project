package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/vladislavdragonenkov/oms/internal/domain"
)

const (
	opTimeout = 5 * time.Second
)

type orderRepository struct {
	db *sql.DB
}

// NewOrderRepository создаёт PostgreSQL-реализацию OrderRepository.
func NewOrderRepository(store *Store) domain.OrderRepository {
	return &orderRepository{db: store.DB()}
}

func (r *orderRepository) Create(order domain.Order) error {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO orders (
			id, customer_id, status, currency, amount_minor, version, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`,
		order.ID, order.CustomerID, string(order.Status), order.Currency,
		order.AmountMinor, order.Version, order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrOrderVersionConflict
		}
		return fmt.Errorf("insert order: %w", err)
	}

	for _, item := range order.Items {
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO order_items (
				id, order_id, sku, qty, price_minor, created_at
			) VALUES ($1,$2,$3,$4,$5,$6)
		`,
			item.ID, order.ID, item.SKU, item.Qty, item.PriceMinor, item.CreatedAt,
		); err != nil {
			return fmt.Errorf("insert order item: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit create order: %w", err)
	}

	return nil
}

func (r *orderRepository) Get(id string) (domain.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	var order domain.Order
	var status string

	err := r.db.QueryRowContext(ctx, `
		SELECT id, customer_id, status, currency, amount_minor, version, created_at, updated_at
		FROM orders
		WHERE id = $1
	`, id).Scan(
		&order.ID, &order.CustomerID, &status, &order.Currency,
		&order.AmountMinor, &order.Version, &order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Order{}, domain.ErrOrderNotFound
		}
		return domain.Order{}, fmt.Errorf("select order: %w", err)
	}
	order.Status = domain.OrderStatus(status)

	items, err := r.loadItems(ctx, order.ID)
	if err != nil {
		return domain.Order{}, err
	}
	order.Items = items

	return order, nil
}

func (r *orderRepository) ListByCustomer(customerID string, limit int) ([]domain.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	query := `
		SELECT id, customer_id, status, currency, amount_minor, version, created_at, updated_at
		FROM orders
		WHERE customer_id = $1
		ORDER BY created_at DESC, id DESC
	`

	var (
		rows *sql.Rows
		err  error
	)

	if limit > 0 {
		rows, err = r.db.QueryContext(ctx, query+" LIMIT $2", customerID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, query, customerID)
	}
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	orders := make([]domain.Order, 0)
	for rows.Next() {
		var order domain.Order
		var status string
		if err := rows.Scan(
			&order.ID, &order.CustomerID, &status, &order.Currency,
			&order.AmountMinor, &order.Version, &order.CreatedAt, &order.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order row: %w", err)
		}
		order.Status = domain.OrderStatus(status)

		items, err := r.loadItems(ctx, order.ID)
		if err != nil {
			return nil, err
		}
		order.Items = items
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order rows: %w", err)
	}

	return orders, nil
}

func (r *orderRepository) Save(order domain.Order) error {
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	res, err := tx.ExecContext(ctx, `
		UPDATE orders
		SET customer_id = $1,
		    status = $2,
		    currency = $3,
		    amount_minor = $4,
		    version = version + 1,
		    updated_at = $5
		WHERE id = $6
		  AND version = $7
	`,
		order.CustomerID,
		string(order.Status),
		order.Currency,
		order.AmountMinor,
		order.UpdatedAt,
		order.ID,
		order.Version,
	)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if affected == 0 {
		exists, err := r.orderExistsTx(ctx, tx, order.ID)
		if err != nil {
			return err
		}
		if !exists {
			return domain.ErrOrderNotFound
		}
		return domain.ErrOrderVersionConflict
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit save order: %w", err)
	}

	return nil
}

func (r *orderRepository) loadItems(ctx context.Context, orderID string) ([]domain.OrderItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, sku, qty, price_minor, created_at
		FROM order_items
		WHERE order_id = $1
		ORDER BY created_at ASC, id ASC
	`, orderID)
	if err != nil {
		return nil, fmt.Errorf("load order items: %w", err)
	}
	defer rows.Close()

	items := make([]domain.OrderItem, 0)
	for rows.Next() {
		var item domain.OrderItem
		if err := rows.Scan(&item.ID, &item.SKU, &item.Qty, &item.PriceMinor, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan order item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order items: %w", err)
	}

	return items, nil
}

func (r *orderRepository) orderExistsTx(ctx context.Context, tx *sql.Tx, orderID string) (bool, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM orders WHERE id = $1`, orderID).Scan(&id)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, fmt.Errorf("check order exists: %w", err)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

var _ domain.OrderRepository = (*orderRepository)(nil)
