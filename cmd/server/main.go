package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	orderv1 "storemesh-order-service/gen/storemesh/order/v1"
	"storemesh-order-service/internal/clients"
	"storemesh-order-service/internal/repository"
	"storemesh-order-service/internal/service"
	"storemesh-order-service/internal/observability"
)

func main() {
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}
	server := grpc.NewServer()
	go serveMetrics()
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		store, err := repository.Open(context.Background(), databaseURL)
		if err != nil {
			log.Fatal(err)
		}
		defer store.Close()
		productAddress, inventoryAddress := os.Getenv("PRODUCT_SERVICE_ADDRESS"), os.Getenv("INVENTORY_SERVICE_ADDRESS")
		if productAddress != "" && inventoryAddress != "" {
			productOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
			if secret := os.Getenv("PRODUCT_JWT_SECRET"); secret != "" {
				issuer, audience := os.Getenv("PRODUCT_JWT_ISSUER"), os.Getenv("PRODUCT_JWT_AUDIENCE")
				if issuer == "" {
					issuer = "storemesh-product-service"
				}
				if audience == "" {
					audience = "storemesh-platform"
				}
				productOptions = append(productOptions, grpc.WithUnaryInterceptor(clients.UnaryJWTInterceptor(secret, issuer, audience)))
			}
			productConn, err := grpc.Dial(productAddress, productOptions...)
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

func serveMetrics() {
	mux := http.NewServeMux()
	mux.Handle("/metrics", observability.Handler())
	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Println("order metrics listening on :8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("metrics server: %v", err)
	}
}
