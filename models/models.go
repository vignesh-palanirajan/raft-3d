package models

import (
	"encoding/json"

	"github.com/hashicorp/raft"
)

// Command is the structure for commands applied to the FSM via Raft.
type Command struct {
	Op   string          `json:"op"`
	Data json.RawMessage `json:"data"`
}

// Type aliases for raft package types for convenience
type ServerID = raft.ServerID
type ServerAddress = raft.ServerAddress

// Printer represents a 3D printer.
type Printer struct {
	ID      string `json:"id"`
	Company string `json:"company"`
	Model   string `json:"model"`
}

// Filament represents a spool of 3D printer filament.
type Filament struct {
	ID                     string `json:"id"`
	Type                   string `json:"type"` // e.g., PLA, PETG, ABS, TPU
	Color                  string `json:"color"`
	TotalWeightInGrams     int    `json:"total_weight_in_grams"`
	RemainingWeightInGrams int    `json:"remaining_weight_in_grams"`
}

// PrintJob represents a request to print a 3D model.
type PrintJob struct {
	ID                 string `json:"id"`
	PrinterID          string `json:"printer_id"`
	FilamentID         string `json:"filament_id"`
	FilePath           string `json:"filepath"` // Path to the 3D model file
	PrintWeightInGrams int    `json:"print_weight_in_grams"`
	Status             string `json:"status"` // e.g., Queued, Running, Done, Canceled
}
