package replication

import (
	"context"
	"log/slog"
	"time"

	"github.com/mvajhi/dist/ca3/internal/storage"
	pb "github.com/mvajhi/dist/ca3/proto/pb"
)

// EventualReplicator implements async replication (eventual consistency).
type EventualReplicator struct {
	baseReplicator
}

// NewEventualReplicator creates a new eventual consistency replicator.
func NewEventualReplicator(store *storage.MemoryStore, peers PeerConnector, replicaID string, logger *slog.Logger) *EventualReplicator {
	return &EventualReplicator{
		baseReplicator: baseReplicator{
			store:     store,
			peers:     peers,
			replicaID: replicaID,
			logger:    logger,
		},
	}
}

// HandlePut writes locally and replicates asynchronously.
func (e *EventualReplicator) HandlePut(ctx context.Context, key, value string) (*storage.ValueEntry, error) {
	// Step 1: Write to local store immediately.
	entry := e.store.Put(key, value)

	e.logger.Info("local_put_completed",
		slog.String("event_type", "Put_Local"),
		slog.String("key", key),
		slog.String("value", value),
		slog.Int64("version", entry.Version),
	)

	// Step 2: Fire-and-forget async replication to all peers.
	go e.replicateToAll(key, entry)

	return entry, nil
}

// HandleGet reads from the local store only (eventual consistency).
func (e *EventualReplicator) HandleGet(ctx context.Context, key string) (*storage.ValueEntry, bool, error) {
	entry, found := e.store.Get(key)
	return entry, found, nil
}

// replicateToAll sends the entry to all peer replicas asynchronously with retry.
func (e *EventualReplicator) replicateToAll(key string, entry *storage.ValueEntry) {
	clients := e.peers.GetPeerClients()
	addrs := e.peers.GetPeerAddresses()

	for i, client := range clients {
		go e.replicateToPeer(client, addrs[i], key, entry)
	}
}

// replicateToPeer sends a replication request to a single peer with up to 3 retries.
func (e *EventualReplicator) replicateToPeer(client pb.KVStoreClient, addr string, key string, entry *storage.ValueEntry) {
	req := &pb.ReplicateRequest{
		Key:       key,
		Value:     entry.Value,
		Version:   entry.Version,
		UpdatedBy: entry.UpdatedBy,
		Timestamp: entry.Timestamp,
	}

	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := client.Replicate(ctx, req)
		cancel()

		if err != nil {
			e.logger.Warn("replication_failed",
				slog.String("event_type", "Replication_Failed"),
				slog.String("peer", addr),
				slog.String("key", key),
				slog.Int64("version", entry.Version),
				slog.Int("attempt", attempt),
				slog.String("error", err.Error()),
			)
			// Exponential backoff: 100ms, 200ms, 400ms
			time.Sleep(time.Duration(100*(1<<(attempt-1))) * time.Millisecond)
			continue
		}

		if resp.Accepted {
			e.logger.Info("replication_succeeded",
				slog.String("event_type", "Replication_Sent"),
				slog.String("peer", addr),
				slog.String("key", key),
				slog.Int64("version", entry.Version),
			)
		} else {
			e.logger.Info("replication_rejected",
				slog.String("event_type", "Replication_Rejected"),
				slog.String("peer", addr),
				slog.String("key", key),
				slog.Int64("version", entry.Version),
				slog.String("reason", resp.Reason),
			)
		}
		return
	}

	e.logger.Error("replication_exhausted",
		slog.String("event_type", "Replication_Exhausted"),
		slog.String("peer", addr),
		slog.String("key", key),
		slog.Int64("version", entry.Version),
	)
}
