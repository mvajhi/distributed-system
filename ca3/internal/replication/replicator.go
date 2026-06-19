package replication

import (
	"context"
	"log/slog"

	"github.com/mvajhi/dist/ca3/internal/storage"
	pb "github.com/mvajhi/dist/ca3/proto/pb"
)

// Replicator defines the interface for handling replicated Put and Get operations.
type Replicator interface {
	// HandlePut processes a client Put request under the configured consistency model.
	HandlePut(ctx context.Context, key, value string) (*storage.ValueEntry, error)

	// HandleGet processes a client Get request under the configured consistency model.
	HandleGet(ctx context.Context, key string) (*storage.ValueEntry, bool, error)
}

// PeerConnector provides gRPC client connections to peer replicas.
type PeerConnector interface {
	// GetPeerClients returns gRPC clients for all peer replicas.
	GetPeerClients() []pb.KVStoreClient

	// GetPeerAddresses returns the addresses of all peer replicas.
	GetPeerAddresses() []string
}

// baseReplicator holds shared dependencies for both consistency models.
type baseReplicator struct {
	store     *storage.MemoryStore
	peers     PeerConnector
	replicaID string
	logger    *slog.Logger
}
