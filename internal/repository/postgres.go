package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/protobuf/types/known/timestamppb"
	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
)

type Orders interface {
	Insert(context.Context, *orderv1.Order, string) error
	FindByIdempotencyKey(context.Context, string) (*orderv1.Order, error)
	Find(context.Context, string) (*orderv1.Order, error)
	List(context.Context, string, orderv1.OrderStatus, int, int) ([]*orderv1.Order, int, error)
	Cancel(context.Context, string, time.Time) (*orderv1.Order, error)
}

type Postgres struct{ db *sql.DB }

func Open(ctx context.Context, databaseURL string) (*Postgres, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &Postgres{db: db}, nil
}

func (p *Postgres) Close() error { return p.db.Close() }

func (p *Postgres) Insert(ctx context.Context, order *orderv1.Order, idempotencyKey string) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO orders (order_id, customer_id, total_minor, currency, status, created_at, updated_at, idempotency_key) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, order.OrderId, order.CustomerId, order.TotalMinor, order.Currency, order.Status, order.CreatedAt.AsTime(), order.UpdatedAt.AsTime(), nullableKey(idempotencyKey))
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}
	for i, line := range order.Lines {
		if _, err = tx.ExecContext(ctx, `INSERT INTO order_lines (order_id, line_number, product_id, quantity, unit_price_minor) VALUES ($1,$2,$3,$4,$5)`, order.OrderId, i, line.ProductId, line.Quantity, line.UnitPriceMinor); err != nil {
			return fmt.Errorf("insert order line: %w", err)
		}
	}
	return tx.Commit()
}

func (p *Postgres) FindByIdempotencyKey(ctx context.Context, key string) (*orderv1.Order, error) {
	var id string
	if err := p.db.QueryRowContext(ctx, `SELECT order_id FROM orders WHERE idempotency_key=$1`, key).Scan(&id); err != nil {
		return nil, err
	}
	return p.Find(ctx, id)
}

func nullableKey(key string) any {
	if key == "" {
		return nil
	}
	return key
}

func (p *Postgres) Find(ctx context.Context, id string) (*orderv1.Order, error) {
	order := &orderv1.Order{}
	var created, updated time.Time
	err := p.db.QueryRowContext(ctx, `SELECT customer_id,total_minor,currency,status,created_at,updated_at FROM orders WHERE order_id=$1`, id).Scan(&order.CustomerId, &order.TotalMinor, &order.Currency, &order.Status, &created, &updated)
	if err != nil {
		return nil, err
	}
	order.OrderId, order.CreatedAt, order.UpdatedAt = id, timestamppb.New(created), timestamppb.New(updated)
	rows, err := p.db.QueryContext(ctx, `SELECT product_id,quantity,unit_price_minor FROM order_lines WHERE order_id=$1 ORDER BY line_number`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		line := &orderv1.OrderLine{}
		if err := rows.Scan(&line.ProductId, &line.Quantity, &line.UnitPriceMinor); err != nil {
			return nil, err
		}
		order.Lines = append(order.Lines, line)
	}
	return order, rows.Err()
}

func (p *Postgres) List(ctx context.Context, customerID string, status orderv1.OrderStatus, offset, limit int) ([]*orderv1.Order, int, error) {
	args := []any{}
	where := "WHERE 1=1"
	if customerID != "" {
		args = append(args, customerID)
		where += fmt.Sprintf(" AND customer_id=$%d", len(args))
	}
	if status != orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED {
		args = append(args, status)
		where += fmt.Sprintf(" AND status=$%d", len(args))
	}
	var total int
	if err := p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := p.db.QueryContext(ctx, "SELECT order_id,customer_id,total_minor,currency,status,created_at,updated_at FROM orders "+where+fmt.Sprintf(" ORDER BY created_at DESC, order_id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	orders := make([]*orderv1.Order, 0, limit)
	for rows.Next() {
		order := &orderv1.Order{}
		var created, updated time.Time
		if err := rows.Scan(&order.OrderId, &order.CustomerId, &order.TotalMinor, &order.Currency, &order.Status, &created, &updated); err != nil {
			return nil, 0, err
		}
		order.CreatedAt, order.UpdatedAt = timestamppb.New(created), timestamppb.New(updated)
		orders = append(orders, order)
	}
	return orders, total, rows.Err()
}

func (p *Postgres) Cancel(ctx context.Context, id string, at time.Time) (*orderv1.Order, error) {
	result, err := p.db.ExecContext(ctx, `UPDATE orders SET status=$2, updated_at=$3 WHERE order_id=$1 AND status <> $2`, id, orderv1.OrderStatus_ORDER_STATUS_CANCELLED, at)
	if err != nil {
		return nil, err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return nil, sql.ErrNoRows
	}
	return p.Find(ctx, id)
}
