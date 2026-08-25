package clients

import (
	"context"

	inventoryv1 "github.com/sartim/storemesh-inventory-service/gen/storemesh/inventory/v1"
	productv1 "github.com/sartim/storemesh-product-service/gen/storemesh/product/v1"
	"google.golang.org/grpc"

	"storemesh-order-service/internal/service"
)

type ProductCatalog struct {
	client productv1.ProductCatalogServiceClient
}

func NewProductCatalog(conn grpc.ClientConnInterface) *ProductCatalog {
	return &ProductCatalog{client: productv1.NewProductCatalogServiceClient(conn)}
}

func (c *ProductCatalog) GetProduct(ctx context.Context, id string) (service.ProductSnapshot, error) {
	response, err := c.client.GetProduct(ctx, &productv1.GetProductRequest{Id: id})
	if err != nil {
		return service.ProductSnapshot{}, err
	}
	product := response.GetProduct()
	return service.ProductSnapshot{ID: product.GetId(), PriceMinor: product.GetPriceMinor(), Currency: product.GetCurrency(), Active: product.GetStatus() == productv1.ProductStatus_PRODUCT_STATUS_ACTIVE}, nil
}

type Inventory struct {
	client inventoryv1.InventoryServiceClient
}

func NewInventory(conn grpc.ClientConnInterface) *Inventory {
	return &Inventory{client: inventoryv1.NewInventoryServiceClient(conn)}
}

func (c *Inventory) Reserve(ctx context.Context, productID, reservationID string, quantity int64) error {
	_, err := c.client.ReserveStock(ctx, &inventoryv1.ReserveStockRequest{ProductId: productID, Reservation: &inventoryv1.StockReservation{ReservationId: reservationID, Quantity: quantity}})
	return err
}

func (c *Inventory) Release(ctx context.Context, productID, reservationID string, quantity int64) error {
	_, err := c.client.ReleaseReservation(ctx, &inventoryv1.ReleaseReservationRequest{ProductId: productID, Reservation: &inventoryv1.StockReservation{ReservationId: reservationID, Quantity: quantity}})
	return err
}
