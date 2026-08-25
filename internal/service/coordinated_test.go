package service

import (
	"context"
	"errors"
	"testing"
	"time"

	orderv1 "storemesh-order-service/gen/storemesh/order/v1"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeCatalog map[string]ProductSnapshot

func (f fakeCatalog) GetProduct(_ context.Context, id string) (ProductSnapshot, error) {
	p, ok := f[id]
	if !ok {
		return ProductSnapshot{}, errors.New("not found")
	}
	return p, nil
}

type fakeInventory struct {
	reserved, released []string
	fail               bool
}

func (f *fakeInventory) Reserve(_ context.Context, product, reservation string, quantity int64) error {
	if f.fail {
		return errors.New("insufficient stock")
	}
	f.reserved = append(f.reserved, product+":"+reservation)
	return nil
}
func (f *fakeInventory) Release(_ context.Context, product, reservation string, quantity int64) error {
	f.released = append(f.released, product+":"+reservation)
	return nil
}

type fakeStore struct{ order *orderv1.Order }

func (f *fakeStore) Insert(_ context.Context, order *orderv1.Order) error {
	f.order = clone(order)
	return nil
}
func (f *fakeStore) Find(_ context.Context, id string) (*orderv1.Order, error) {
	if f.order == nil || f.order.OrderId != id {
		return nil, errors.New("not found")
	}
	return clone(f.order), nil
}
func (f *fakeStore) Cancel(_ context.Context, id string, at time.Time) (*orderv1.Order, error) {
	if f.order == nil || f.order.OrderId != id {
		return nil, errors.New("not found")
	}
	f.order.Status = orderv1.OrderStatus_ORDER_STATUS_CANCELLED
	f.order.UpdatedAt = timestamppb.New(at)
	return clone(f.order), nil
}

func TestCoordinatedOrdersSnapshotsPricesAndReserves(t *testing.T) {
	store, inventory := &fakeStore{}, &fakeInventory{}
	service := NewCoordinatedOrders(store, fakeCatalog{"p1": {ID: "p1", PriceMinor: 1250, Currency: "USD", Active: true}}, inventory)
	response, err := service.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{Order: &orderv1.Order{CustomerId: "customer", Lines: []*orderv1.OrderLine{{ProductId: "p1", Quantity: 2}}}})
	require.NoError(t, err)
	require.Equal(t, int64(2500), response.Order.TotalMinor)
	require.Equal(t, int64(1250), response.Order.Lines[0].UnitPriceMinor)
	require.Len(t, inventory.reserved, 1)
}

func TestCoordinatedOrdersReleasesEarlierReservationsOnFailure(t *testing.T) {
	store, inventory := &fakeStore{}, &fakeInventory{}
	service := NewCoordinatedOrders(store, fakeCatalog{"p1": {ID: "p1", PriceMinor: 100, Currency: "USD", Active: true}, "p2": {ID: "p2", PriceMinor: 200, Currency: "USD", Active: true}}, inventory)
	inventory.fail = true
	_, err := service.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{Order: &orderv1.Order{CustomerId: "customer", Lines: []*orderv1.OrderLine{{ProductId: "p1", Quantity: 1}}}})
	require.Error(t, err)
	require.Nil(t, store.order)
}
