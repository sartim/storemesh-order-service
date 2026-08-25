package main

import (
	"context"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
	"storemesh-order-service/internal/clients"
	"storemesh-order-service/internal/repository"
	"storemesh-order-service/internal/service"
)

func main() {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}
	server := grpc.NewServer()
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		store, err := repository.Open(context.Background(), databaseURL)
		if err != nil {
			log.Fatal(err)
		}
		defer store.Close()
		productAddress, inventoryAddress := os.Getenv("PRODUCT_SERVICE_ADDRESS"), os.Getenv("INVENTORY_SERVICE_ADDRESS")
		if productAddress != "" && inventoryAddress != "" {
			productConn, err := grpc.Dial(productAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatal(err)
			}
			defer productConn.Close()
			inventoryConn, err := grpc.Dial(inventoryAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatal(err)
			}
			defer inventoryConn.Close()
			orderv1.RegisterOrderServiceServer(server, service.NewCoordinatedOrders(store, clients.NewProductCatalog(productConn), clients.NewInventory(inventoryConn)))
		} else {
			orderv1.RegisterOrderServiceServer(server, service.NewPersistentOrders(store))
		}
	} else {
		orderv1.RegisterOrderServiceServer(server, service.NewOrders())
	}
	log.Println("order service listening on :50051")
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
