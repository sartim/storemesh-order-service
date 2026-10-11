package service

import (
	"context"
	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
	auth "storemesh-order-service/internal/auth"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestOrderTotalsAndCancellation(t *testing.T) {
	o := NewOrders()
	created, err := o.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{Order: &orderv1.Order{CustomerId: "customer-1", Currency: "USD", Lines: []*orderv1.OrderLine{{ProductId: "p-1", Quantity: 2, UnitPriceMinor: 500}}}})
	if err != nil || created.GetOrder().GetTotalMinor() != 1000 {
		t.Fatalf("create: order=%v err=%v", created, err)
	}
	cancelled, err := o.CancelOrder(context.Background(), &orderv1.CancelOrderRequest{OrderId: created.GetOrder().GetOrderId()})
	if err != nil || cancelled.GetOrder().GetStatus() != orderv1.OrderStatus_ORDER_STATUS_CANCELLED {
		t.Fatalf("cancel: order=%v err=%v", cancelled, err)
	}
}

func TestCustomerOrderOwnershipAndAdminScope(t *testing.T) {
	o := NewOrders()
	create := func(customer string) string {
		t.Helper()
		response, err := o.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{Order: &orderv1.Order{CustomerId: customer, Currency: "USD", Lines: []*orderv1.OrderLine{{ProductId: "p-1", Quantity: 1, UnitPriceMinor: 100}}}})
		if err != nil {
			t.Fatal(err)
		}
		return response.GetOrder().GetOrderId()
	}
	ownerOrderID := create("customer-1")
	otherOrderID := create("customer-2")
	customer := auth.WithIdentity(context.Background(), auth.Identity{Subject: "customer-1"})
	created, err := o.CreateOrder(customer, &orderv1.CreateOrderRequest{Order: &orderv1.Order{CustomerId: "customer-1", Currency: "USD", Lines: []*orderv1.OrderLine{{ProductId: "p-1", Quantity: 1, UnitPriceMinor: 100}}}})
	if err != nil || created.GetOrder().GetCustomerId() != "customer-1" {
		t.Fatalf("owner order creation failed: order=%v err=%v", created, err)
	}
	_, err = o.CreateOrder(customer, &orderv1.CreateOrderRequest{Order: &orderv1.Order{CustomerId: "customer-2", Currency: "USD", Lines: []*orderv1.OrderLine{{ProductId: "p-1", Quantity: 1, UnitPriceMinor: 100}}}})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cross-customer create should be denied, got %v", err)
	}
	list, err := o.ListOrders(customer, &orderv1.ListOrdersRequest{})
	if err != nil || len(list.GetOrders()) != 2 {
		t.Fatalf("customer list should be implicitly scoped: orders=%v err=%v", list.GetOrders(), err)
	}
	_, err = o.ListOrders(customer, &orderv1.ListOrdersRequest{CustomerId: "customer-2"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("cross-customer list should be denied, got %v", err)
	}
	_, err = o.GetOrder(customer, &orderv1.GetOrderRequest{OrderId: otherOrderID})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("cross-customer get should be hidden as not found, got %v", err)
	}
	_, err = o.CancelOrder(customer, &orderv1.CancelOrderRequest{OrderId: otherOrderID})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("cross-customer cancellation should be hidden as not found, got %v", err)
	}
	if _, err := o.GetOrder(customer, &orderv1.GetOrderRequest{OrderId: ownerOrderID}); err != nil {
		t.Fatalf("owner should read own order: %v", err)
	}
	admin := auth.WithIdentity(context.Background(), auth.Identity{Subject: "admin-1", Admin: true})
	adminList, err := o.ListOrders(admin, &orderv1.ListOrdersRequest{})
	if err != nil || len(adminList.GetOrders()) != 3 {
		t.Fatalf("admin should list all orders: orders=%v err=%v", adminList.GetOrders(), err)
	}
	if _, err := o.GetOrder(admin, &orderv1.GetOrderRequest{OrderId: ownerOrderID}); err != nil {
		t.Fatalf("admin should read another customer's order: %v", err)
	}
}

func TestListOrdersFiltersAndPaginates(t *testing.T) {
	o := NewOrders()
	for _, customer := range []string{"customer-1", "customer-1", "customer-2"} {
		_, err := o.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{Order: &orderv1.Order{CustomerId: customer, Currency: "USD", Lines: []*orderv1.OrderLine{{ProductId: "p-1", Quantity: 1, UnitPriceMinor: 500}}}})
		if err != nil {
			t.Fatal(err)
		}
	}
	first, err := o.ListOrders(context.Background(), &orderv1.ListOrdersRequest{CustomerId: "customer-1", PageSize: 1})
	if err != nil || len(first.GetOrders()) != 1 || first.GetNextPageToken() == "" {
		t.Fatalf("first page: response=%v err=%v", first, err)
	}
	second, err := o.ListOrders(context.Background(), &orderv1.ListOrdersRequest{CustomerId: "customer-1", PageSize: 1, PageToken: first.GetNextPageToken()})
	if err != nil || len(second.GetOrders()) != 1 || second.GetNextPageToken() != "" {
		t.Fatalf("second page: response=%v err=%v", second, err)
	}
}
