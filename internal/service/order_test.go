package service

import (
	"context"
	"testing"
	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
)

func TestOrderTotalsAndCancellation(t *testing.T) {
	o := NewOrders(); created, err := o.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{Order: &orderv1.Order{CustomerId: "customer-1", Currency: "USD", Lines: []*orderv1.OrderLine{{ProductId: "p-1", Quantity: 2, UnitPriceMinor: 500}}}})
	if err != nil || created.GetOrder().GetTotalMinor() != 1000 { t.Fatalf("create: order=%v err=%v", created, err) }
	cancelled, err := o.CancelOrder(context.Background(), &orderv1.CancelOrderRequest{OrderId: created.GetOrder().GetOrderId()})
	if err != nil || cancelled.GetOrder().GetStatus() != orderv1.OrderStatus_ORDER_STATUS_CANCELLED { t.Fatalf("cancel: order=%v err=%v", cancelled, err) }
}
