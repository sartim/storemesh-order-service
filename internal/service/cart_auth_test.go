package service

import (
	"context"
	"testing"

	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
	auth "storemesh-order-service/internal/auth"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCartOperationsAreCustomerScoped(t *testing.T) {
	carts := NewCarts(NewMemoryCarts())
	owner := auth.WithIdentity(context.Background(), auth.Identity{Subject: "customer-1"})
	if _, err := carts.UpsertCart(owner, &orderv1.UpsertCartRequest{Cart: &orderv1.Cart{CustomerId: "customer-1", Lines: []*orderv1.CartLine{{ProductId: "p-1", Quantity: 2}}}}); err != nil {
		t.Fatalf("owner cart update failed: %v", err)
	}
	if _, err := carts.GetCart(owner, &orderv1.GetCartRequest{CustomerId: "customer-2"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cross-customer cart read should be denied, got %v", err)
	}
	if _, err := carts.UpsertCart(owner, &orderv1.UpsertCartRequest{Cart: &orderv1.Cart{CustomerId: "customer-2"}}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cross-customer cart update should be denied, got %v", err)
	}
	if _, err := carts.ClearCart(owner, &orderv1.ClearCartRequest{CustomerId: "customer-2"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cross-customer cart clear should be denied, got %v", err)
	}
	admin := auth.WithIdentity(context.Background(), auth.Identity{Subject: "admin-1", Admin: true})
	if _, err := carts.GetCart(admin, &orderv1.GetCartRequest{CustomerId: "customer-1"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("admin role alone should not grant cart access, got %v", err)
	}
}
