package raft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"raft3d/models"
)

type Node struct {
	Raft      *raft.Raft
	FSM       *FSM
	transport *raft.NetworkTransport
	config    Config
	ID        string
}

type Config struct {
	RaftAddr  string
	HTTPAddr  string
	DataDir   string
	Bootstrap bool
}

func LoadConfig(nodeID string) (Config, error) {
	if !strings.HasPrefix(nodeID, "node") {
		return Config{}, fmt.Errorf("invalid node ID format: %s. Expected 'nodeX'", nodeID)
	}
	nodeNumStr := strings.TrimPrefix(nodeID, "node")
	nodeNum, err := strconv.Atoi(nodeNumStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid node ID number: %s", nodeNumStr)
	}

	raftPort := 5000 + nodeNum
	httpPort := 8080 + nodeNum

	return Config{
		RaftAddr:  fmt.Sprintf("127.0.0.1:%d", raftPort),
		HTTPAddr:  fmt.Sprintf("127.0.0.1:%d", httpPort),
		DataDir:   filepath.Join("data", nodeID),
		Bootstrap: nodeID == "node1",
	}, nil
}

func NewNode(nodeID string, config Config) (*Node, error) {
	// Clean up old state
	dbPath := filepath.Join(config.DataDir, "raft.db")
	if err := os.RemoveAll(dbPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to clean old db: %w", err)
	}

	if err := os.MkdirAll(config.DataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	// Initialize FSM
	fsm := NewFSM()

	// Configure Raft
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = models.ServerID(nodeID)
	raftConfig.Logger = hclog.New(&hclog.LoggerOptions{
		Name:   "raft",
		Output: os.Stderr,
		Level:  hclog.Info,
	})

	// Initialize BoltDB store (simplified - no custom options)
	boltDB, err := raftboltdb.NewBoltStore(filepath.Join(config.DataDir, "raft.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create bolt store: %w", err)
	}

	// Use single store instance
	logStore := boltDB
	stableStore := boltDB

	// Initialize snapshot store
	snapshots, err := raft.NewFileSnapshotStore(config.DataDir, 3, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %w", err)
	}

	// Create network transport
	raftAddr, err := net.ResolveTCPAddr("tcp", config.RaftAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve raft address: %w", err)
	}

	transport, err := raft.NewTCPTransport(
		config.RaftAddr,
		raftAddr,
		3,
		10*time.Second,
		os.Stderr,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	// Create Raft node
	raftNode, err := raft.NewRaft(
		raftConfig,
		fsm,
		logStore,
		stableStore,
		snapshots,
		transport,
	)
	if err != nil {
		transport.Close()
		return nil, fmt.Errorf("failed to create raft node: %w", err)
	}

	// Bootstrap if needed
	if config.Bootstrap {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raftConfig.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		if err := raftNode.BootstrapCluster(configuration).Error(); err != nil {
			log.Printf("[WARN] Bootstrap failed: %v", err)
		}
	}

	return &Node{
		Raft:      raftNode,
		FSM:       fsm,
		transport: transport,
		config:    config,
		ID:        nodeID,
	}, nil
}

func (n *Node) JoinCluster(joinAddr string) error {
	req := struct {
		ID      string `json:"id"`
		Address string `json:"address"`
	}{
		ID:      n.ID,
		Address: string(n.transport.LocalAddr()),
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal join request: %w", err)
	}

	resp, err := http.Post(
		fmt.Sprintf("http://%s/join", joinAddr),
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		return fmt.Errorf("join request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("join failed: status=%d, body=%s", resp.StatusCode, body)
	}

	return nil
}

func (n *Node) GetFSM() *FSM {
	return n.FSM
}

func (n *Node) IsLeader() bool {
	return n.Raft.State() == raft.Leader
}

func (n *Node) GetRaft() *raft.Raft {
	return n.Raft
}