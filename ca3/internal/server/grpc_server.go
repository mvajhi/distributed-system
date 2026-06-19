package server

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/mvajhi/dist/ca3/internal/replication"
	"github.com/mvajhi/dist/ca3/internal/storage"
	pb "github.com/mvajhi/dist/ca3/proto/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// KVStoreServer implements the gRPC KVStore service.
type KVStoreServer struct {
	pb.UnimplementedKVStoreServer

	store     *storage.MemoryStore
	eventual  replication.Replicator
	strong    replication.Replicator
	replicaID string
	logger    *slog.Logger

	// Network delay injection (atomic for thread safety).
	networkDelayMs atomic.Int64

	// For graceful shutdown.
	shutdownFn func()

	// Peer connections.
	peerClients []pb.KVStoreClient
	peerAddrs   []string
	peerConns   []*grpc.ClientConn
}

// ServerConfig contains the dependencies to construct a KVStoreServer.
type ServerConfig struct {
	Store          *storage.MemoryStore
	ReplicaID      string
	PeerAddresses  []string
	NetworkDelayMs int64
	Logger         *slog.Logger
	ShutdownFn     func()
}

// NewKVStoreServer creates and wires up the server with peer connections and replicators.
func NewKVStoreServer(cfg ServerConfig) *KVStoreServer {
	s := &KVStoreServer{
		store:      cfg.Store,
		replicaID:  cfg.ReplicaID,
		peerAddrs:  cfg.PeerAddresses,
		logger:     cfg.Logger,
		shutdownFn: cfg.ShutdownFn,
	}
	s.networkDelayMs.Store(cfg.NetworkDelayMs)
	return s
}

// ConnectToPeers establishes gRPC connections to all peer replicas.
// Call this after server creation but before serving requests.
func (s *KVStoreServer) ConnectToPeers() error {
	for _, addr := range s.peerAddrs {
		conn, err := grpc.NewClient(
			addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			s.logger.Error("peer_connection_failed",
				slog.String("event_type", "Peer_Connection_Failed"),
				slog.String("peer", addr),
				slog.String("error", err.Error()),
			)
			return err
		}
		s.peerConns = append(s.peerConns, conn)
		s.peerClients = append(s.peerClients, pb.NewKVStoreClient(conn))
		s.logger.Info("peer_connected",
			slog.String("event_type", "Peer_Connected"),
			slog.String("peer", addr),
		)
	}

	// Wire up replicators now that peers are connected.
	s.eventual = replication.NewEventualReplicator(s.store, s, s.replicaID, s.logger)
	s.strong = replication.NewStrongReplicator(s.store, s, s.replicaID, s.logger)
	return nil
}

// Close shuts down peer connections.
func (s *KVStoreServer) Close() {
	for _, conn := range s.peerConns {
		conn.Close()
	}
}

// ─── PeerConnector interface ───────────────────────────────────────────────

// GetPeerClients returns gRPC clients for all peer replicas.
func (s *KVStoreServer) GetPeerClients() []pb.KVStoreClient {
	return s.peerClients
}

// GetPeerAddresses returns the addresses of all peer replicas.
func (s *KVStoreServer) GetPeerAddresses() []string {
	return s.peerAddrs
}

// ─── gRPC service implementation ──────────────────────────────────────────

// Put handles client Put requests, delegating to the appropriate consistency model.
func (s *KVStoreServer) Put(ctx context.Context, req *pb.PutRequest) (*pb.PutResponse, error) {
	s.logger.Info("put_request_received",
		slog.String("event_type", "Put_Request"),
		slog.String("key", req.Key),
		slog.String("value", req.Value),
		slog.String("consistency", req.Consistency.String()),
	)

	var entry *storage.ValueEntry
	var err error

	switch req.Consistency {
	case pb.ConsistencyModel_STRONG:
		entry, err = s.strong.HandlePut(ctx, req.Key, req.Value)
	default:
		entry, err = s.eventual.HandlePut(ctx, req.Key, req.Value)
	}

	if err != nil {
		s.logger.Error("put_failed",
			slog.String("event_type", "Put_Failed"),
			slog.String("key", req.Key),
			slog.String("error", err.Error()),
		)
		return &pb.PutResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &pb.PutResponse{
		Success: true,
		Version: entry.Version,
	}, nil
}

// Get handles client Get requests.
func (s *KVStoreServer) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	s.logger.Info("get_request_received",
		slog.String("event_type", "Get_Request"),
		slog.String("key", req.Key),
		slog.String("consistency", req.Consistency.String()),
	)

	var entry *storage.ValueEntry
	var found bool
	var err error

	switch req.Consistency {
	case pb.ConsistencyModel_STRONG:
		entry, found, err = s.strong.HandleGet(ctx, req.Key)
	default:
		entry, found, err = s.eventual.HandleGet(ctx, req.Key)
	}

	if err != nil {
		return &pb.GetResponse{
			Found: false,
			Error: err.Error(),
		}, nil
	}

	if !found {
		return &pb.GetResponse{
			Found: false,
			Key:   req.Key,
		}, nil
	}

	return &pb.GetResponse{
		Found:     true,
		Key:       req.Key,
		Value:     entry.Value,
		Version:   entry.Version,
		UpdatedBy: entry.UpdatedBy,
	}, nil
}

// Replicate handles inter-replica replication requests.
func (s *KVStoreServer) Replicate(ctx context.Context, req *pb.ReplicateRequest) (*pb.ReplicateResponse, error) {
	// Inject configurable network delay.
	delayMs := s.networkDelayMs.Load()
	if delayMs > 0 {
		s.logger.Debug("injecting_delay",
			slog.String("event_type", "Delay_Injected"),
			slog.Int64("delay_ms", delayMs),
			slog.String("key", req.Key),
		)
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}

	incoming := &storage.ValueEntry{
		Value:     req.Value,
		Version:   req.Version,
		UpdatedBy: req.UpdatedBy,
		Timestamp: req.Timestamp,
	}

	result := s.store.ApplyReplication(req.Key, incoming)

	eventType := "Replication_Accepted"
	if !result.Accepted {
		eventType = "Replication_Rejected"
	}

	s.logger.Info("replication_applied",
		slog.String("event_type", eventType),
		slog.String("key", req.Key),
		slog.Int64("incoming_version", req.Version),
		slog.String("from", req.UpdatedBy),
		slog.Bool("accepted", result.Accepted),
		slog.String("reason", result.Reason),
		slog.Int64("current_version", result.CurrentVersion),
	)

	return &pb.ReplicateResponse{
		Accepted:       result.Accepted,
		CurrentVersion: result.CurrentVersion,
		Reason:         result.Reason,
	}, nil
}

// HealthCheck returns the alive status of the replica.
func (s *KVStoreServer) HealthCheck(ctx context.Context, req *pb.Empty) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{
		Alive:     true,
		ReplicaId: s.replicaID,
	}, nil
}

// Shutdown gracefully stops the replica (used in testing scenarios).
func (s *KVStoreServer) Shutdown(ctx context.Context, req *pb.ShutdownRequest) (*pb.ShutdownResponse, error) {
	s.logger.Info("shutdown_requested",
		slog.String("event_type", "Shutdown_Requested"),
		slog.String("reason", req.Reason),
	)

	// Signal the main goroutine to shut down.
	go func() {
		time.Sleep(100 * time.Millisecond) // Brief delay to allow response to be sent.
		if s.shutdownFn != nil {
			s.shutdownFn()
		} else {
			os.Exit(0)
		}
	}()

	return &pb.ShutdownResponse{Acknowledged: true}, nil
}

// SetDelay dynamically changes the network delay for this replica.
func (s *KVStoreServer) SetDelay(ctx context.Context, req *pb.SetDelayRequest) (*pb.SetDelayResponse, error) {
	s.networkDelayMs.Store(req.DelayMs)
	s.logger.Info("delay_updated",
		slog.String("event_type", "Delay_Updated"),
		slog.Int64("delay_ms", req.DelayMs),
	)
	return &pb.SetDelayResponse{Success: true}, nil
}
