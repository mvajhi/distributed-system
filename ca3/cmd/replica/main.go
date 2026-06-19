package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/mvajhi/dist/ca3/internal/config"
	"github.com/mvajhi/dist/ca3/internal/logger"
	"github.com/mvajhi/dist/ca3/internal/server"
	"github.com/mvajhi/dist/ca3/internal/storage"
	pb "github.com/mvajhi/dist/ca3/proto/pb"
	"google.golang.org/grpc"
)

func main() {
	configPath := flag.String("config", "", "Path to replica config JSON file")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: replica --config <path/to/config.json>")
		os.Exit(1)
	}

	// Load configuration.
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger.
	log := logger.New(cfg.ReplicaID, cfg.LogLevel)
	log.Info("starting_replica",
		"event_type", "Startup",
		"port", cfg.Port,
		"peers", cfg.Peers,
		"default_consistency", cfg.DefaultConsistency,
		"network_delay_ms", cfg.NetworkDelayMs,
	)

	// Initialize storage.
	store := storage.NewMemoryStore(cfg.ReplicaID)

	// Create gRPC server.
	grpcServer := grpc.NewServer()

	// Create and register KVStore server.
	kvServer := server.NewKVStoreServer(server.ServerConfig{
		Store:          store,
		ReplicaID:      cfg.ReplicaID,
		PeerAddresses:  cfg.Peers,
		NetworkDelayMs: cfg.NetworkDelayMs,
		Logger:         log,
		ShutdownFn: func() {
			grpcServer.GracefulStop()
		},
	})

	pb.RegisterKVStoreServer(grpcServer, kvServer)

	// Connect to peers.
	if err := kvServer.ConnectToPeers(); err != nil {
		log.Error("failed_to_connect_peers",
			"event_type", "Startup_Error",
			"error", err.Error(),
		)
		// Non-fatal: peers may not be up yet. Connections will be retried on first use.
		log.Warn("continuing_without_peers",
			"event_type", "Startup_Warning",
		)
	}

	// Start listening.
	lis, err := net.Listen("tcp", cfg.ListenAddr())
	if err != nil {
		log.Error("failed_to_listen",
			"event_type", "Startup_Error",
			"error", err.Error(),
		)
		os.Exit(1)
	}

	log.Info("replica_ready",
		"event_type", "Ready",
		"address", cfg.ListenAddr(),
	)

	// Handle graceful shutdown via OS signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Info("shutdown_signal_received",
			"event_type", "Shutdown",
			"signal", sig.String(),
		)
		kvServer.Close()
		grpcServer.GracefulStop()
	}()

	// Serve.
	if err := grpcServer.Serve(lis); err != nil {
		log.Error("server_error",
			"event_type", "Server_Error",
			"error", err.Error(),
		)
		os.Exit(1)
	}

	log.Info("replica_stopped", "event_type", "Stopped")
}
