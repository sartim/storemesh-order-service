package service

import (
	"context"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
)

type CartStore interface {
	Get(context.Context, string) (*orderv1.Cart, error)
	Upsert(context.Context, *orderv1.Cart) (*orderv1.Cart, error)
	Clear(context.Context, string) (*orderv1.Cart, error)
}

type Carts struct {
	orderv1.UnimplementedCartServiceServer
	store CartStore
}

func NewCarts(store CartStore) *Carts { return &Carts{store: store} }
func (c *Carts) GetCart(ctx context.Context, req *orderv1.GetCartRequest) (*orderv1.GetCartResponse, error) {
	if req.GetCustomerId() == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}
	cart, err := c.store.Get(ctx, req.GetCustomerId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load cart: %v", err)
	}
	return &orderv1.GetCartResponse{Cart: cart}, nil
}
func (c *Carts) UpsertCart(ctx context.Context, req *orderv1.UpsertCartRequest) (*orderv1.UpsertCartResponse, error) {
	if req.GetCart() == nil || req.GetCart().GetCustomerId() == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}
	for _, line := range req.GetCart().GetLines() {
		if line.GetProductId() == "" || line.GetQuantity() <= 0 {
			return nil, status.Error(codes.InvalidArgument, "cart lines require a product and positive quantity")
		}
	}
	cart, err := c.store.Upsert(ctx, req.GetCart())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "save cart: %v", err)
	}
	return &orderv1.UpsertCartResponse{Cart: cart}, nil
}
func (c *Carts) ClearCart(ctx context.Context, req *orderv1.ClearCartRequest) (*orderv1.ClearCartResponse, error) {
	if req.GetCustomerId() == "" {
		return nil, status.Error(codes.InvalidArgument, "customer_id is required")
	}
	cart, err := c.store.Clear(ctx, req.GetCustomerId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "clear cart: %v", err)
	}
	return &orderv1.ClearCartResponse{Cart: cart}, nil
}

type MemoryCarts struct {
	mu    sync.RWMutex
	carts map[string]*orderv1.Cart
}

func NewMemoryCarts() *MemoryCarts { return &MemoryCarts{carts: map[string]*orderv1.Cart{}} }
func (m *MemoryCarts) Get(_ context.Context, id string) (*orderv1.Cart, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneCart(m.carts[id]), nil
}
func (m *MemoryCarts) Upsert(_ context.Context, cart *orderv1.Cart) (*orderv1.Cart, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.carts[cart.CustomerId] = cloneCart(cart)
	return cloneCart(cart), nil
}
func (m *MemoryCarts) Clear(_ context.Context, id string) (*orderv1.Cart, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cart := &orderv1.Cart{CustomerId: id}
	m.carts[id] = cart
	return cloneCart(cart), nil
}
func cloneCart(cart *orderv1.Cart) *orderv1.Cart {
	if cart == nil {
		return &orderv1.Cart{}
	}
	return proto.Clone(cart).(*orderv1.Cart)
}
