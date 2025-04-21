package raft

import (
	"encoding/json"

	"raft3d/models" // Import the models package

	"github.com/hashicorp/raft"
)

// snapshot is a struct that holds the FSM state for snapshotting.
type snapshot struct {
	Printers  map[string]models.Printer  `json:"printers"`
	Filaments map[string]models.Filament `json:"filaments"`
	Jobs      map[string]models.PrintJob `json:"jobs"`
}

// Persist writes the snapshot to the given sink.
// This is called by the Raft library when a snapshot needs to be saved.
func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	// Encode the snapshot data to JSON and write it to the sink.
	err := json.NewEncoder(sink).Encode(s)
	if err != nil {
		// If encoding fails, cancel the snapshot operation.
		sink.Cancel()
		return err
	}

	// Close the sink to finalize the snapshot.
	return sink.Close()
}

// Release is called by the Raft library after a snapshot is
// successfully persisted or if there was an error during persistence.
// This method can be used to clean up any resources held by the snapshot.
func (s *snapshot) Release() {
	// In this case, the snapshot holds maps which will be garbage collected
	// when no longer needed, so no explicit cleanup is required.
}