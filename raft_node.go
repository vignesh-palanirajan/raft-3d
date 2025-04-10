package main

import (
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-boltdb"
)

// NewRaftNode initializes a single Raft node
func NewRaftNode(nodeID string, raftDir string) (*raft.Raft, error) {
	// Create Raft configuration
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)

	// Create persistent storage
	store, err := raftboltdb.NewBoltStore(raftDir + "/raft.db")
	if err != nil {
		return nil, fmt.Errorf("failed to create bolt store: %v", err)
	}

	// Create stable store and log store
	logStore := store
	stableStore := store

	// Create snapshot store
	snapshots, err := raft.NewFileSnapshotStore(raftDir, 1, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %v", err)
	}

	// Create transport (local communication)
	addr := "127.0.0.1:5000" // Change for multiple nodes
	transport, err := raft.NewTCPTransport(addr, nil, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %v", err)
	}

	// Create Raft system
	raftNode, err := raft.NewRaft(config, nil, logStore, stableStore, snapshots, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create Raft node: %v", err)
	}

	fmt.Println("Raft node initialized:", nodeID)
	return raftNode, nil
}
