package service

import (
	"context"
	"sort"
	"strconv"
	"sync"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
)

type Orders struct {
	orderv1.UnimplementedOrderServiceServer
	mu     sync.RWMutex
	orders map[string]*orderv1.Order
}

func NewOrders() *Orders { return &Orders{orders: make(map[string]*orderv1.Order)} }

func (o *Orders) CreateOrder(_ context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
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
	o.mu.Lock()
	defer o.mu.Unlock()
	o.orders[order.OrderId] = order
	return &orderv1.CreateOrderResponse{Order: clone(order)}, nil
}

func (o *Orders) GetOrder(_ context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	order, ok := o.orders[req.GetOrderId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	return &orderv1.GetOrderResponse{Order: clone(order)}, nil
}

func (o *Orders) ListOrders(_ context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	pageSize, offset, err := pageParameters(req)
	if err != nil {
		return nil, err
	}
	o.mu.RLock()
	orders := make([]*orderv1.Order, 0, len(o.orders))
	for _, order := range o.orders {
		if req.GetCustomerId() != "" && order.GetCustomerId() != req.GetCustomerId() {
			continue
		}
		if req.GetStatus() != orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED && order.GetStatus() != req.GetStatus() {
			continue
		}
		orders = append(orders, clone(order))
	}
	o.mu.RUnlock()
	sort.Slice(orders, func(i, j int) bool {
		if orders[i].GetCreatedAt().AsTime().Equal(orders[j].GetCreatedAt().AsTime()) {
			return orders[i].GetOrderId() > orders[j].GetOrderId()
		}
		return orders[i].GetCreatedAt().AsTime().After(orders[j].GetCreatedAt().AsTime())
	})
	if offset > len(orders) {
		return nil, status.Error(codes.InvalidArgument, "page_token is invalid")
	}
	end := offset + pageSize
	if end > len(orders) {
		end = len(orders)
	}
	next := ""
	if end < len(orders) {
		next = strconv.Itoa(end)
	}
	return &orderv1.ListOrdersResponse{Orders: orders[offset:end], NextPageToken: next}, nil
}

func pageParameters(req *orderv1.ListOrdersRequest) (int, int, error) {
	pageSize := int(req.GetPageSize())
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize < 1 || pageSize > 100 {
		return 0, 0, status.Error(codes.InvalidArgument, "page_size must be between 1 and 100")
	}
	offset := 0
	if req.GetPageToken() != "" {
		parsed, err := strconv.Atoi(req.GetPageToken())
		if err != nil || parsed < 0 {
			return 0, 0, status.Error(codes.InvalidArgument, "page_token is invalid")
		}
		offset = parsed
	}
	return pageSize, offset, nil
}

func (o *Orders) CancelOrder(_ context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	order, ok := o.orders[req.GetOrderId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	if order.Status == orderv1.OrderStatus_ORDER_STATUS_CANCELLED {
		return nil, status.Error(codes.FailedPrecondition, "order is already cancelled")
	}
	order.Status, order.UpdatedAt = orderv1.OrderStatus_ORDER_STATUS_CANCELLED, timestamppb.Now()
	return &orderv1.CancelOrderResponse{Order: clone(order)}, nil
}

func clone(order *orderv1.Order) *orderv1.Order {
	copy := *order
	copy.Lines = append([]*orderv1.OrderLine(nil), order.Lines...)
	return &copy
}
