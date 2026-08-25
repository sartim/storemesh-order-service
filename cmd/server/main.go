package main

import (
	"log"
	"net"

	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
	"storemesh-order-service/internal/service"
	"google.golang.org/grpc"
)

func main() {
	listener, err := net.Listen("tcp", ":50051"); if err != nil { log.Fatal(err) }
	server := grpc.NewServer(); orderv1.RegisterOrderServiceServer(server, service.NewOrders()); log.Println("order service listening on :50051")
	if err := server.Serve(listener); err != nil { log.Fatal(err) }
}
