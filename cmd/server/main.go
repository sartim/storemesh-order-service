package main

import (
	"context"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
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
		orderv1.RegisterOrderServiceServer(server, service.NewPersistentOrders(store))
	} else {
		orderv1.RegisterOrderServiceServer(server, service.NewOrders())
	}
	log.Println("order service listening on :50051")
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
