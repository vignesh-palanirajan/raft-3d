package main

import (
	"log"
	"net/http"
	"os"
	"fmt"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <nodeID>")
	}

	nodeID := os.Args[1]

	var bindAddr, raftAddr, httpPort string

	if nodeID == "node1" {
		bindAddr = "127.0.0.1:5000"
		raftAddr = "127.0.0.1:5000"
		httpPort = "8080"
	} else if nodeID == "node2" {
		bindAddr = "127.0.0.1:5001"
		raftAddr = "127.0.0.1:5001"
		httpPort = "8081"
	} else {
		log.Fatalf("Unknown node ID: %s", nodeID)
	}

	err := NewRaftNode(nodeID, bindAddr, raftAddr, httpPort)
	if err != nil {
		log.Fatalf("Error starting Raft: %v", err)
	}

	// Follower joins the cluster
	if nodeID != "node1" {
		joinURL := fmt.Sprintf("http://127.0.0.1:8080/join?addr=%s&id=%s", raftAddr, nodeID)
		resp, err := http.Get(joinURL)
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to join cluster: %v", err)
		}
	}

	select {} // block forever
}
