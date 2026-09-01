package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"storemesh-order-service/internal/repository"
)

func main() {
	databaseURL, brokers := os.Getenv("DATABASE_URL"), os.Getenv("KAFKA_BROKERS")
	if databaseURL == "" || brokers == "" {
		log.Fatal("DATABASE_URL and KAFKA_BROKERS are required")
	}
	store, err := repository.Open(context.Background(), databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	writer := &kafka.Writer{Addr: kafka.TCP(strings.Split(brokers, ",")...), Balancer: &kafka.Hash{}, BatchTimeout: 100 * time.Millisecond}
	defer writer.Close()
	ctx := context.Background()
	for {
		events, err := store.PendingOutbox(ctx, 100)
		if err != nil {
			log.Fatal(err)
		}
		for _, event := range events {
			topic := "storemesh.order.events"
			err = writer.WriteMessages(ctx, kafka.Message{Topic: topic, Key: []byte(event.AggregateID), Value: event.Payload, Headers: []kafka.Header{{Key: "event-type", Value: []byte(event.EventType)}, {Key: "event-id", Value: []byte(event.ID)}}})
			if err != nil {
				log.Printf("publish %s: %v", event.ID, err)
				continue
			}
			if err := store.MarkOutboxPublished(ctx, event.ID, time.Now()); err != nil {
				log.Printf("mark %s: %v", event.ID, err)
			}
		}
		time.Sleep(time.Second)
	}
}
