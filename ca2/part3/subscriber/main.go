package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/redis/go-redis/v9"
)

type MemoryEvent struct {
	EventType   string `json:"event_type"`
	Service     string `json:"service"`
	MemoryMB    uint64 `json:"memory_mb"`
	ThresholdMB uint64 `json:"threshold_mb"`
	Timestamp   string `json:"timestamp"`
}

func main() {
	log.Println("Subscriber service starting...")

	rdb := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	ctx := context.Background()
	pubsub := rdb.Subscribe(ctx, "memory_alerts")
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Println("Subscribed to 'memory_alerts' channel. Waiting for events...")

	for msg := range ch {
		var event MemoryEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			log.Printf("Payload parse error: %v\n", err)
			continue
		}

		printAlert(event)
	}
}

func printAlert(event MemoryEvent) {
	border := strings.Repeat("=", 50)
	fmt.Printf("\n%s\n", border)
	fmt.Printf("ALERT: %s\n", event.EventType)
	fmt.Printf("%s\n", border)
	fmt.Printf("Service:    %s\n", event.Service)
	fmt.Printf("Memory:     %d MB (Threshold: %d MB)\n", event.MemoryMB, event.ThresholdMB)
	fmt.Printf("Timestamp:  %s\n", event.Timestamp)
	fmt.Printf("%s\n\n", border)
}