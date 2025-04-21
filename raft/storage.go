package raft

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

func setupStores(config Config) (raft.LogStore, raft.StableStore, raft.SnapshotStore, error) {
	// Clean up any existing BoltDB file
	dbPath := filepath.Join(config.DataDir, "raft.db")
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return nil, nil, nil, fmt.Errorf("failed to clean old db: %w", err)
	}

	// Create data directory
	if err := os.MkdirAll(config.DataDir, 0700); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	// Initialize BoltDB store (using the simple NewBoltStore interface)
	boltDB, err := raftboltdb.NewBoltStore(dbPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create bolt store: %w", err)
	}

	// Initialize snapshot store
	snapshots, err := raft.NewFileSnapshotStore(config.DataDir, 3, os.Stderr)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create snapshot store: %w", err)
	}

	log.Printf("Storage initialized successfully at %s", config.DataDir)
	return boltDB, boltDB, snapshots, nil
}