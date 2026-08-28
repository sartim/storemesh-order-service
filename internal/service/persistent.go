package service

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
	"storemesh-order-service/internal/repository"
)

type PersistentOrders struct {
	orderv1.UnimplementedOrderServiceServer
	store repository.Orders
}

func NewPersistentOrders(store repository.Orders) *PersistentOrders {
	return &PersistentOrders{store: store}
}

func (o *PersistentOrders) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	if req != nil && req.GetIdempotencyKey() != "" {
		key := req.GetIdempotencyKey()
		if existing, lookupErr := o.store.FindByIdempotencyKey(ctx, key); lookupErr == nil {
			return &orderv1.CreateOrderResponse{Order: existing}, nil
		}
	}
	order, err := prepare(req)
	if err != nil {
		return nil, err
	}
	if err := o.store.Insert(ctx, order, req.GetIdempotencyKey()); err != nil {
		return nil, status.Errorf(codes.Internal, "persist order: %v", err)
	}
	return &orderv1.CreateOrderResponse{Order: clone(order)}, nil
}

func (o *PersistentOrders) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	order, err := o.store.Find(ctx, req.GetOrderId())
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load order: %v", err)
	}
	return &orderv1.GetOrderResponse{Order: order}, nil
}

func (o *PersistentOrders) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	pageSize, offset, err := pageParameters(req)
	if err != nil {
		return nil, err
	}
	orders, total, err := o.store.List(ctx, req.GetCustomerId(), req.GetStatus(), offset, pageSize)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list orders: %v", err)
	}
	next := ""
	if offset+len(orders) < total {
		next = strconv.Itoa(offset + len(orders))
	}
	return &orderv1.ListOrdersResponse{Orders: orders, NextPageToken: next}, nil
}

func (o *PersistentOrders) CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	order, err := o.store.Cancel(ctx, req.GetOrderId(), time.Now())
	if err == sql.ErrNoRows {
		return nil, status.Error(codes.NotFound, "order not found or already cancelled")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cancel order: %v", err)
	}
	return &orderv1.CancelOrderResponse{Order: order}, nil
}

func prepare(req *orderv1.CreateOrderRequest) (*orderv1.Order, error) {
	if req == nil || req.GetOrder() == nil || req.GetOrder().GetCustomerId() == "" || len(req.GetOrder().GetLines()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "customer and order lines are required")
	}
	order := clone(req.GetOrder())
	order.OrderId, order.Status = uuid.NewString(), orderv1.OrderStatus_ORDER_STATUS_PENDING
	order.TotalMinor = 0
	for _, line := range order.Lines {
		if line.GetQuantity() <= 0 || line.GetUnitPriceMinor() < 0 {
			return nil, status.Error(codes.InvalidArgument, "line quantity must be positive and price cannot be negative")
		}
		order.TotalMinor += line.Quantity * line.UnitPriceMinor
	}
	order.CreatedAt, order.UpdatedAt = timestamppb.Now(), timestamppb.Now()
	return order, nil
}
