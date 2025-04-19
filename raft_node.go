package main

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net"
    "net/http"
    "os"
    "sync"
    "time"

    "github.com/hashicorp/raft"
    "github.com/hashicorp/raft-boltdb"
)

type PrintJob struct {
    ID       string
    File     string
    Priority int
}

type JobFSM struct {
    jobs map[string]PrintJob
    mu   sync.Mutex
}

func NewJobFSM() *JobFSM {
    return &JobFSM{jobs: make(map[string]PrintJob)}
}

func (fsm *JobFSM) Apply(logEntry *raft.Log) interface{} {
    var job PrintJob
    if err := json.Unmarshal(logEntry.Data, &job); err != nil {
        return err
    }

    fsm.mu.Lock()
    defer fsm.mu.Unlock()
    fsm.jobs[job.ID] = job
    fmt.Println("Job added:", job)
    return nil
}

func (fsm *JobFSM) Snapshot() (raft.FSMSnapshot, error) {
    return &JobSnapshot{jobs: fsm.jobs}, nil
}

func (fsm *JobFSM) Restore(rc io.ReadCloser) error {
    return nil
}

type JobSnapshot struct {
    jobs map[string]PrintJob
}

func (s *JobSnapshot) Persist(sink raft.SnapshotSink) error {
    return nil
}

func (s *JobSnapshot) Release() {}

func NewRaftNode(nodeID, bindAddr, raftAddr, httpPort string) error {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)

	os.MkdirAll("data/"+nodeID, 0700)
	logStore, err := raftboltdb.NewBoltStore("data/" + nodeID + "/raft.db")
	if err != nil {
		return err
	}

	stableStore := logStore
	snapshotStore := raft.NewInmemSnapshotStore()

	addr, err := net.ResolveTCPAddr("tcp", raftAddr)
	if err != nil {
		return err
	}

	transport, err := raft.NewTCPTransport(raftAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return err
	}

	fsm := NewJobFSM()

	raftNode, err := raft.NewRaft(config, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return err
	}

	if nodeID == "node1" {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raft.ServerID(nodeID),
					Address: raft.ServerAddress(raftAddr),
				},
			},
		}
		raftNode.BootstrapCluster(configuration)
	}

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Raft node %s is healthy\n", nodeID)
	})

	// Submit a print job
	http.HandleFunc("/print", func(w http.ResponseWriter, r *http.Request) {
		if raftNode.State() != raft.Leader {
			http.Error(w, "Not the leader", http.StatusBadRequest)
			return
		}

		var job PrintJob
		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
			http.Error(w, "Invalid job", http.StatusBadRequest)
			return
		}

		data, err := json.Marshal(job)
		if err != nil {
			http.Error(w, "Failed to serialize job", http.StatusInternalServerError)
			return
		}

		applyFuture := raftNode.Apply(data, 5*time.Second)
		if err := applyFuture.Error(); err != nil {
			http.Error(w, "Raft apply failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Fprintln(w, "Print job submitted:", job.ID)
	})

	// Join request
	http.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
		if raftNode.State() != raft.Leader {
			http.Error(w, "Not the leader", http.StatusBadRequest)
			return
		}

		addr := r.URL.Query().Get("addr")
		id := r.URL.Query().Get("id")
		if addr == "" || id == "" {
			http.Error(w, "Missing addr or id", http.StatusBadRequest)
			return
		}

		future := raftNode.AddVoter(raft.ServerID(id), raft.ServerAddress(addr), 0, 0)
		if err := future.Error(); err != nil {
			http.Error(w, "Failed to add voter: "+err.Error(), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, "Node %s joined successfully\n", id)
	})

	// View jobs
	http.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		fsm.mu.Lock()
		defer fsm.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fsm.jobs)
	})

	// Simulate print processing
	go func() {
		for {
			time.Sleep(10 * time.Second)
			fsm.mu.Lock()
			if len(fsm.jobs) > 0 {
				for id, job := range fsm.jobs {
					fmt.Printf("Processing job: %s (File: %s, Priority: %d)\n", job.ID, job.File, job.Priority)
					delete(fsm.jobs, id)
					break
				}
			}
			fsm.mu.Unlock()
		}
	}()

	go http.ListenAndServe(":"+httpPort, nil)

	log.Printf("Raft node initialized: %s", nodeID)
	log.Printf("Server running on port %s", httpPort)
	return nil
}
