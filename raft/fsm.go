package raft

import (
	"encoding/json"
	"fmt"
	"io"
	"raft3d/models"
	"sync"

	"github.com/hashicorp/raft"
)

// FSM is the Finite State Machine for the Raft node.
// It holds the application state (printers, filaments, jobs).
type FSM struct {
	Mu        sync.RWMutex // Exported mutex to protect access to the state
	Printers  map[string]models.Printer // Exported map for printers
	Filaments map[string]models.Filament // Exported map for filaments
	Jobs      map[string]models.PrintJob // Exported map for print jobs
}

// NewFSM creates a new FSM with initialized maps.
func NewFSM() *FSM {
	return &FSM{
		Printers:  make(map[string]models.Printer),
		Filaments: make(map[string]models.Filament),
		Jobs:      make(map[string]models.PrintJob),
	}
}

// Apply applies a Raft log entry to the FSM.
// This is called by the Raft library on the leader and followers.
func (f *FSM) Apply(log *raft.Log) interface{} {
	var cmd models.Command
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		// In a real application, you might want more sophisticated error handling
		// here, but returning the error allows the Raft library to potentially
		// signal it back to the Apply caller on the leader.
		return fmt.Errorf("failed to unmarshal command: %w", err)
	}

	f.Mu.Lock() // Use the exported mutex
	defer f.Mu.Unlock() // Use the exported mutex

	switch cmd.Op {
	case "add_printer":
		var p models.Printer
		if err := json.Unmarshal(cmd.Data, &p); err != nil {
			return err
		}
		f.Printers[p.ID] = p // Use the exported map

	case "add_filament":
		var fl models.Filament
		if err := json.Unmarshal(cmd.Data, &fl); err != nil {
			return err
		}
		f.Filaments[fl.ID] = fl // Use the exported map

	case "add_job":
		var job models.PrintJob
		if err := json.Unmarshal(cmd.Data, &job); err != nil {
			return err
		}
		// Validate job data before adding (e.g., printer/filament existence, filament weight)
		if err := f.validateJob(job); err != nil {
			// Return validation error
			return fmt.Errorf("job validation failed: %w", err)
		}
		f.Jobs[job.ID] = job // Use the exported map

	case "update_job_status":
		var req struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal(cmd.Data, &req); err != nil {
			return err
		}
		job, exists := f.Jobs[req.ID] // Use the exported map
		if !exists {
			return fmt.Errorf("job with ID %s not found", req.ID)
		}
		// Validate status transition
		if err := f.validateStatusTransition(job.Status, req.Status); err != nil {
			return fmt.Errorf("invalid status transition: %w", err)
		}

		// If status transitions to Done, reduce filament weight
		if req.Status == "Done" {
			filament, filamentExists := f.Filaments[job.FilamentID] // Use the exported map
			if !filamentExists {
				// This case should ideally be caught by validateJob, but adding a check here for safety
				return fmt.Errorf("filament %s for job %s not found", job.FilamentID, job.ID)
			}
			filament.RemainingWeightInGrams -= job.PrintWeightInGrams
			f.Filaments[job.FilamentID] = filament // Update the exported map
		}

		job.Status = req.Status
		f.Jobs[job.ID] = job // Update the exported map

	default:
		return fmt.Errorf("unknown operation: %s", cmd.Op)
	}
	return nil // Return nil on successful application
}

// Snapshot returns a snapshot of the FSM's state.
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.Mu.RLock() // Use the exported mutex
	defer f.Mu.RUnlock() // Use the exported mutex

	// Create a copy of the state to avoid modifying it while snapshotting
	snapshot := &snapshot{
		Printers:  make(map[string]models.Printer),
		Filaments: make(map[string]models.Filament),
		Jobs:      make(map[string]models.PrintJob),
	}

	for k, v := range f.Printers { // Use the exported map
		snapshot.Printers[k] = v
	}
	for k, v := range f.Filaments { // Use the exported map
		snapshot.Filaments[k] = v
	}
	for k, v := range f.Jobs { // Use the exported map
		snapshot.Jobs[k] = v
	}

	return snapshot, nil
}

// Restore restores the FSM's state from a snapshot.
func (f *FSM) Restore(rc io.ReadCloser) error {
	var snap snapshot
	if err := json.NewDecoder(rc).Decode(&snap); err != nil {
		return fmt.Errorf("failed to decode snapshot: %w", err)
	}

	f.Mu.Lock() // Use the exported mutex
	defer f.Mu.Unlock() // Use the exported mutex

	// Replace the current state with the restored state
	f.Printers = snap.Printers
	f.Filaments = snap.Filaments
	f.Jobs = snap.Jobs

	return nil
}

// validateJob performs validation on a PrintJob before it is applied to the FSM.
func (f *FSM) validateJob(job models.PrintJob) error {
	// Check if the printer exists
	if _, exists := f.Printers[job.PrinterID]; !exists { // Use the exported map
		return fmt.Errorf("printer with ID %s not found", job.PrinterID)
	}

	// Check if the filament exists and has enough remaining weight
	filament, exists := f.Filaments[job.FilamentID] // Use the exported map
	if !exists {
		return fmt.Errorf("filament with ID %s not found", job.FilamentID)
	}

	if job.PrintWeightInGrams <= 0 {
		return fmt.Errorf("print weight must be positive")
	}

	if job.PrintWeightInGrams > filament.RemainingWeightInGrams {
		return fmt.Errorf("insufficient filament remaining for filament ID %s. Required: %d g, Available: %d g", job.FilamentID, job.PrintWeightInGrams, filament.RemainingWeightInGrams)
	}

	// Basic validation for status
	if job.Status != "Queued" {
		return fmt.Errorf("initial job status must be 'Queued'")
	}

	return nil
}

// validateStatusTransition checks if a status transition for a print job is valid.
func (f *FSM) validateStatusTransition(old, new string) error {
	if old == new {
        return fmt.Errorf("status is already %s", old)
    }

	validTransitions := map[string][]string{
		"Queued":  {"Running", "Canceled"},
		"Running": {"Done", "Canceled"},
        "Done": {}, // No transitions from Done
        "Canceled": {}, // No transitions from Canceled
	}

	allowedTransitions, ok := validTransitions[old]
	if !ok {
		return fmt.Errorf("invalid old status: %s", old)
	}

	for _, status := range allowedTransitions {
		if status == new {
			return nil
		}
	}

	return fmt.Errorf("invalid status transition from '%s' to '%s'", old, new)
}