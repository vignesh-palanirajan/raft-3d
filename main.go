package main

import (
	"fmt"
	"log"
	"net/http"
)

var raftNode *Raft // Global Raft node

func main() {
	// Initialize Raft node
	var err error
	raftNode, err = NewRaftNode("node1", "./data")
	if err != nil {
		log.Fatalf("Error starting Raft: %v", err)
	}

	// Start HTTP server
	http.HandleFunc("/health", healthCheckHandler)

	port := "8080"
	fmt.Println("Server running on port", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Raft3D API is running!")
}
