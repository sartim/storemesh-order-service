package service

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
	"storemesh-order-service/internal/repository"
)

// ProductCatalog is the adapter boundary for the Product Service gRPC client.
type ProductCatalog interface {
	GetProduct(context.Context, string) (ProductSnapshot, error)
}

type ProductSnapshot struct {
	ID         string
	PriceMinor int64
	Currency   string
	Active     bool
}

// Inventory is the adapter boundary for Inventory Service reservation calls.
// The order ID is used as the reservation ID, making retries deterministic.
type Inventory interface {
	Reserve(context.Context, string, string, int64) error
	Release(context.Context, string, string, int64) error
}

type CoordinatedOrders struct {
	orderv1.UnimplementedOrderServiceServer
	store     repository.Orders
	catalog   ProductCatalog
	inventory Inventory
}

func NewCoordinatedOrders(store repository.Orders, catalog ProductCatalog, inventory Inventory) *CoordinatedOrders {
	return &CoordinatedOrders{store: store, catalog: catalog, inventory: inventory}
}

func (o *CoordinatedOrders) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	if req == nil || req.GetOrder() == nil || req.GetOrder().GetCustomerId() == "" || len(req.GetOrder().GetLines()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "customer and order lines are required")
	}
	order := clone(req.GetOrder())
	order.OrderId = uuid.NewString()
	order.Status = orderv1.OrderStatus_ORDER_STATUS_PENDING
	order.TotalMinor = 0
	var currency string
	reserved := make([]*orderv1.OrderLine, 0, len(order.Lines))
	for _, line := range order.Lines {
		product, err := o.catalog.GetProduct(ctx, line.GetProductId())
		if err != nil {
			return nil, status.Errorf(codes.FailedPrecondition, "product %q unavailable: %v", line.GetProductId(), err)
		}
		if !product.Active {
			return nil, status.Errorf(codes.FailedPrecondition, "product %q is not active", line.GetProductId())
		}
		if currency == "" {
			currency = product.Currency
		}
		if currency != product.Currency {
			return nil, status.Error(codes.InvalidArgument, "order lines must use one currency")
		}
		if line.GetQuantity() <= 0 {
			return nil, status.Error(codes.InvalidArgument, "line quantity must be positive")
		}
		line.UnitPriceMinor = product.PriceMinor
		order.TotalMinor += line.Quantity * line.UnitPriceMinor
		if err := o.inventory.Reserve(ctx, line.ProductId, order.OrderId, line.Quantity); err != nil {
			o.release(ctx, order.OrderId, reserved)
			return nil, status.Errorf(codes.FailedPrecondition, "reserve product %q: %v", line.ProductId, err)
		}
		reserved = append(reserved, line)
	}
	order.Currency = currency
	order.CreatedAt, order.UpdatedAt = timestamppb.Now(), timestamppb.Now()
	if err := o.store.Insert(ctx, order); err != nil {
		o.release(ctx, order.OrderId, reserved)
		return nil, status.Errorf(codes.Internal, "persist order: %v", err)
	}
	return &orderv1.CreateOrderResponse{Order: clone(order)}, nil
}

func (o *CoordinatedOrders) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	order, err := o.store.Find(ctx, req.GetOrderId())
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load order: %v", err)
	}
	return &orderv1.GetOrderResponse{Order: order}, nil
}

func (o *CoordinatedOrders) CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	order, err := o.store.Cancel(ctx, req.GetOrderId(), timestamppb.Now().AsTime())
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "order not found or already cancelled")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cancel order: %v", err)
	}
	for _, line := range order.Lines {
		if err := o.inventory.Release(ctx, line.ProductId, order.OrderId, line.Quantity); err != nil {
			return nil, status.Errorf(codes.Internal, "release product %q: %v", line.ProductId, err)
		}
	}
	return &orderv1.CancelOrderResponse{Order: order}, nil
}

func (o *CoordinatedOrders) release(ctx context.Context, reservationID string, lines []*orderv1.OrderLine) {
	for _, line := range lines {
		_ = o.inventory.Release(ctx, line.ProductId, reservationID, line.Quantity)
	}
}
