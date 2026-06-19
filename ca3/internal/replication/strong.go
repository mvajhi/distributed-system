package replication

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mvajhi/dist/ca3/internal/storage"
	pb "github.com/mvajhi/dist/ca3/proto/pb"
)

const (
	defaultQuorumTimeout = 5 * time.Second
)

// StrongReplicator implements quorum-based replication (strong consistency).
type StrongReplicator struct {
	baseReplicator
	quorumTimeout time.Duration
}

// NewStrongReplicator creates a new strong consistency replicator.
func NewStrongReplicator(store *storage.MemoryStore, peers PeerConnector, replicaID string, logger *slog.Logger) *StrongReplicator {
	return &StrongReplicator{
		baseReplicator: baseReplicator{
			store:     store,
			peers:     peers,
			replicaID: replicaID,
			logger:    logger,
		},
		quorumTimeout: defaultQuorumTimeout,
	}
}

// HandlePut writes locally and waits for quorum acks before returning success.
func (s *StrongReplicator) HandlePut(ctx context.Context, key, value string) (*storage.ValueEntry, error) {
	// Step 1: Write to local store.
	entry := s.store.Put(key, value)

	s.logger.Info("local_put_completed",
		slog.String("event_type", "Put_Local"),
		slog.String("key", key),
		slog.String("value", value),
		slog.Int64("version", entry.Version),
	)

	// Step 2: Send to all peers concurrently and collect acks.
	clients := s.peers.GetPeerClients()
	addrs := s.peers.GetPeerAddresses()
	totalNodes := len(clients) + 1 // peers + self
	quorum := totalNodes/2 + 1     // majority: for 3 nodes, quorum = 2

	// Self already counts as 1 ack.
	acks := 1

	if quorum <= 1 {
		// Only self needed (edge case: single node).
		return entry, nil
	}

	req := &pb.ReplicateRequest{
		Key:       key,
		Value:     entry.Value,
		Version:   entry.Version,
		UpdatedBy: entry.UpdatedBy,
		Timestamp: entry.Timestamp,
	}

	type peerResult struct {
		addr     string
		accepted bool
		err      error
	}

	resultCh := make(chan peerResult, len(clients))

	// Fan-out to all peers.
	for i, client := range clients {
		go func(c pb.KVStoreClient, addr string) {
			rCtx, cancel := context.WithTimeout(ctx, s.quorumTimeout)
			defer cancel()

			resp, err := c.Replicate(rCtx, req)
			if err != nil {
				resultCh <- peerResult{addr: addr, err: err}
				return
			}
			resultCh <- peerResult{addr: addr, accepted: resp.Accepted}
		}(client, addrs[i])
	}

	// Collect results until quorum or all responses received.
	remaining := len(clients)
	var mu sync.Mutex
	timeoutCtx, cancel := context.WithTimeout(ctx, s.quorumTimeout)
	defer cancel()

	for remaining > 0 {
		select {
		case res := <-resultCh:
			remaining--
			mu.Lock()
			if res.err != nil {
				s.logger.Warn("quorum_peer_error",
					slog.String("event_type", "Quorum_Peer_Error"),
					slog.String("peer", res.addr),
					slog.String("key", key),
					slog.String("error", res.err.Error()),
				)
			} else if res.accepted {
				acks++
				s.logger.Info("quorum_peer_ack",
					slog.String("event_type", "Quorum_Peer_Ack"),
					slog.String("peer", res.addr),
					slog.String("key", key),
					slog.Int64("version", entry.Version),
				)
			}
			if acks >= quorum {
				mu.Unlock()
				s.logger.Info("quorum_reached",
					slog.String("event_type", "Quorum_Reached"),
					slog.String("key", key),
					slog.Int("acks", acks),
					slog.Int("quorum", quorum),
				)
				return entry, nil
			}
			mu.Unlock()

		case <-timeoutCtx.Done():
			s.logger.Error("quorum_timeout",
				slog.String("event_type", "Quorum_Timeout"),
				slog.String("key", key),
				slog.Int("acks", acks),
				slog.Int("quorum", quorum),
			)
			return nil, fmt.Errorf("quorum not reached: got %d acks, need %d", acks, quorum)
		}
	}

	// All responses received but quorum not met.
	if acks >= quorum {
		return entry, nil
	}

	s.logger.Error("quorum_not_reached",
		slog.String("event_type", "Quorum_Not_Reached"),
		slog.String("key", key),
		slog.Int("acks", acks),
		slog.Int("quorum", quorum),
	)
	return nil, fmt.Errorf("quorum not reached: got %d acks, need %d", acks, quorum)
}

// HandleGet performs a quorum read: reads from all replicas (self + peers),
// waits for a quorum of responses, and returns the entry with the highest version.
func (s *StrongReplicator) HandleGet(ctx context.Context, key string) (*storage.ValueEntry, bool, error) {
	clients := s.peers.GetPeerClients()
	addrs := s.peers.GetPeerAddresses()
	totalNodes := len(clients) + 1
	quorum := totalNodes/2 + 1

	type readResult struct {
		addr  string
		entry *storage.ValueEntry
		found bool
		err   error
	}

	resultCh := make(chan readResult, totalNodes)

	// Read from local.
	go func() {
		entry, found := s.store.Get(key)
		resultCh <- readResult{addr: "self", entry: entry, found: found}
	}()

	// Read from peers.
	for i, client := range clients {
		go func(c pb.KVStoreClient, addr string) {
			rCtx, cancel := context.WithTimeout(ctx, s.quorumTimeout)
			defer cancel()

			resp, err := c.Get(rCtx, &pb.GetRequest{
				Key:         key,
				Consistency: pb.ConsistencyModel_EVENTUAL, // peer does local read
			})
			if err != nil {
				resultCh <- readResult{addr: addr, err: err}
				return
			}
			if !resp.Found {
				resultCh <- readResult{addr: addr, found: false}
				return
			}
			resultCh <- readResult{
				addr: addr,
				entry: &storage.ValueEntry{
					Value:     resp.Value,
					Version:   resp.Version,
					UpdatedBy: resp.UpdatedBy,
				},
				found: true,
			}
		}(client, addrs[i])
	}

	// Collect quorum responses.
	var (
		best      *storage.ValueEntry
		bestFound bool
		responses int
		successes int
	)

	timeoutCtx, cancel := context.WithTimeout(ctx, s.quorumTimeout)
	defer cancel()

	for responses < totalNodes {
		select {
		case res := <-resultCh:
			responses++
			if res.err != nil {
				s.logger.Warn("quorum_read_error",
					slog.String("event_type", "Quorum_Read_Error"),
					slog.String("peer", res.addr),
					slog.String("key", key),
					slog.String("error", res.err.Error()),
				)
				continue
			}
			successes++
			if res.found {
				if best == nil || res.entry.Version > best.Version {
					best = res.entry
					bestFound = true
				}
			}
			if successes >= quorum {
				return best, bestFound, nil
			}

		case <-timeoutCtx.Done():
			if successes >= quorum {
				return best, bestFound, nil
			}
			return nil, false, fmt.Errorf("quorum read timeout: got %d responses, need %d", successes, quorum)
		}
	}

	if successes >= quorum {
		return best, bestFound, nil
	}
	return nil, false, fmt.Errorf("quorum read failed: got %d responses, need %d", successes, quorum)
}
