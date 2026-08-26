package clients

import (
	"context"
	"net"
	"testing"
	"time"

	inventoryv1 "github.com/sartim/storemesh-inventory-service/gen/storemesh/inventory/v1"
	productv1 "github.com/sartim/storemesh-product-service/gen/storemesh/product/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
	"storemesh-order-service/internal/service"
)

type productServer struct {
	productv1.UnimplementedProductCatalogServiceServer
}

func (productServer) GetProduct(context.Context, *productv1.GetProductRequest) (*productv1.GetProductResponse, error) {
	return &productv1.GetProductResponse{Product: &productv1.Product{
		Id: "p1", PriceMinor: 1250, Currency: "USD",
		Status:    productv1.ProductStatus_PRODUCT_STATUS_ACTIVE,
		CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now(),
	}}, nil
}

type inventoryServer struct {
	inventoryv1.UnimplementedInventoryServiceServer
	reserved int64
}

func (s *inventoryServer) ReserveStock(_ context.Context, req *inventoryv1.ReserveStockRequest) (*inventoryv1.ReserveStockResponse, error) {
	s.reserved += req.GetReservation().GetQuantity()
	return &inventoryv1.ReserveStockResponse{}, nil
}

func (s *inventoryServer) ReleaseReservation(_ context.Context, req *inventoryv1.ReleaseReservationRequest) (*inventoryv1.ReleaseReservationResponse, error) {
	s.reserved -= req.GetReservation().GetQuantity()
	return &inventoryv1.ReleaseReservationResponse{}, nil
}

type integrationStore struct{ order *orderv1.Order }

func (s *integrationStore) Insert(_ context.Context, order *orderv1.Order, _ string) error {
	s.order = order
	return nil
}
func (s *integrationStore) FindByIdempotencyKey(context.Context, string) (*orderv1.Order, error) {
	return nil, context.Canceled
}
func (s *integrationStore) Find(context.Context, string) (*orderv1.Order, error) { return s.order, nil }
func (s *integrationStore) Cancel(context.Context, string, time.Time) (*orderv1.Order, error) {
	return s.order, nil
}

func TestGRPCAdaptersCoordinateOrderCreation(t *testing.T) {
	productConn, productCleanup := serveProduct(t)
	defer productCleanup()
	inventoryConn, inventoryCleanup, inventory := serveInventory(t)
	defer inventoryCleanup()

	store := &integrationStore{}
	orders := service.NewCoordinatedOrders(store, NewProductCatalog(productConn), NewInventory(inventoryConn))
	response, err := orders.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{IdempotencyKey: "integration-1", Order: &orderv1.Order{CustomerId: "customer", Lines: []*orderv1.OrderLine{{ProductId: "p1", Quantity: 2}}}})

	require.NoError(t, err)
	require.Equal(t, int64(2500), response.GetOrder().GetTotalMinor())
	require.Equal(t, int64(1250), response.GetOrder().GetLines()[0].GetUnitPriceMinor())
	require.Equal(t, int64(2), inventory.reserved)
}

func serveProduct(t *testing.T) (*grpc.ClientConn, func()) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	productv1.RegisterProductCatalogServiceServer(server, productServer{})
	go server.Serve(listener)
	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	return conn, func() { conn.Close(); server.Stop(); listener.Close() }
}

func serveInventory(t *testing.T) (*grpc.ClientConn, func(), *inventoryServer) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	implementation := &inventoryServer{}
	inventoryv1.RegisterInventoryServiceServer(server, implementation)
	go server.Serve(listener)
	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	return conn, func() { conn.Close(); server.Stop(); listener.Close() }, implementation
}
