package service

import (
	"context"
	"sync"

	"github.com/google/uuid"
	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Orders struct {
	orderv1.UnimplementedOrderServiceServer
	mu     sync.RWMutex
	orders map[string]*orderv1.Order
}

func NewOrders() *Orders { return &Orders{orders: make(map[string]*orderv1.Order)} }

func (o *Orders) CreateOrder(_ context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	if req == nil || req.GetOrder() == nil || req.GetOrder().GetCustomerId() == "" || len(req.GetOrder().GetLines()) == 0 { return nil, status.Error(codes.InvalidArgument, "customer and order lines are required") }
	order := clone(req.GetOrder()); order.OrderId, order.Status = uuid.NewString(), orderv1.OrderStatus_ORDER_STATUS_PENDING; order.TotalMinor = 0
	for _, line := range order.Lines { if line.GetQuantity() <= 0 || line.GetUnitPriceMinor() < 0 { return nil, status.Error(codes.InvalidArgument, "line quantity must be positive and price cannot be negative") }; order.TotalMinor += line.Quantity * line.UnitPriceMinor }
	order.CreatedAt, order.UpdatedAt = timestamppb.Now(), timestamppb.Now()
	o.mu.Lock(); defer o.mu.Unlock(); o.orders[order.OrderId] = order
	return &orderv1.CreateOrderResponse{Order: clone(order)}, nil
}

func (o *Orders) GetOrder(_ context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	o.mu.RLock(); defer o.mu.RUnlock(); order, ok := o.orders[req.GetOrderId()]; if !ok { return nil, status.Error(codes.NotFound, "order not found") }; return &orderv1.GetOrderResponse{Order: clone(order)}, nil
}

func (o *Orders) CancelOrder(_ context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	o.mu.Lock(); defer o.mu.Unlock(); order, ok := o.orders[req.GetOrderId()]; if !ok { return nil, status.Error(codes.NotFound, "order not found") }; if order.Status == orderv1.OrderStatus_ORDER_STATUS_CANCELLED { return nil, status.Error(codes.FailedPrecondition, "order is already cancelled") }; order.Status, order.UpdatedAt = orderv1.OrderStatus_ORDER_STATUS_CANCELLED, timestamppb.Now(); return &orderv1.CancelOrderResponse{Order: clone(order)}, nil
}

func clone(order *orderv1.Order) *orderv1.Order { copy := *order; copy.Lines = append([]*orderv1.OrderLine(nil), order.Lines...); return &copy }
