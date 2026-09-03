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
	"storemesh-order-service/internal/observability"
	"storemesh-order-service/internal/repository"
	"storemesh-order-service/internal/service"
)

func main() {
	grpcAddr := env("GRPC_ADDR", ":50051")
	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatal(err)
	}
	server := grpc.NewServer()
	go serveMetrics(env("METRICS_ADDR", ":8080"))
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		store, err := repository.Open(context.Background(), databaseURL)
		if err != nil {
			log.Fatal(err)
		}
		defer store.Close()
		orderv1.RegisterCartServiceServer(server, service.NewCarts(store))
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
		orderv1.RegisterCartServiceServer(server, service.NewCarts(service.NewMemoryCarts()))
	}
	log.Println("order service listening on " + grpcAddr)
	if err := server.Serve(listener); err != nil {
		log.Fatal(err)
	}
}

func serveMetrics(addr string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", observability.Handler())
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Println("order metrics listening on " + addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("metrics server: %v", err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
