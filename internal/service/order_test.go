package service

import (
	"context"
	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
	"testing"
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
