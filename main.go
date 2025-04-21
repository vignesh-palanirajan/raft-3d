package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"
	"raft3d/api"
	"raft3d/raft"
)

var (
	nodeID   = flag.String("id", "", "Node ID (required)")
	joinAddr = flag.String("join", "", "Join address (optional)")
)

func main() {
	flag.Parse()

	if *nodeID == "" {
		log.Fatal("Node ID must be specified with -id flag")
	}

	// Clean data directory
	dataDir := filepath.Join("data", *nodeID)
	if err := os.RemoveAll(dataDir); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: cleanup failed: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	config, err := raft.LoadConfig(*nodeID)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	node, err := raft.NewNode(*nodeID, config)
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
	}

	if *joinAddr != "" {
		log.Printf("Joining cluster at %s", *joinAddr)
		if err := node.JoinCluster(*joinAddr); err != nil {
			log.Fatalf("Failed to join cluster: %v", err)
		}
	}

	apiServer := api.NewAPIServer(node, config.HTTPAddr)
	go func() {
		log.Printf("Starting API server on %s", config.HTTPAddr)
		if err := apiServer.Start(); err != nil {
			log.Fatalf("API server failed: %v", err)
		}
	}()

	select {} // Block forever
}
