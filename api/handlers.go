package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"raft3d/models" // Import models for data structures and type aliases
	"raft3d/raft"   // Import raft for accessing the Node and FSM

	hraft "github.com/hashicorp/raft" // Import hashicorp/raft with alias hraft
)

// APIServer is the HTTP server that exposes the API endpoints.
type APIServer struct {
	node *raft.Node      // The Raft node instance
	addr string          // The address the HTTP server listens on
	mux  *http.ServeMux  // HTTP request multiplexer
}

// NewAPIServer creates a new API server.
func NewAPIServer(node *raft.Node, addr string) *APIServer {
	mux := http.NewServeMux() // Use a new ServeMux

	server := &APIServer{
		node: node,
		addr: addr,
		mux:  mux,
	}

	// Register handlers with the multiplexer
	mux.HandleFunc("/join", server.handleJoin)
	mux.HandleFunc("/health", server.handleHealthCheck)
	mux.HandleFunc("/api/v1/printers", server.handlePrinters)
	mux.HandleFunc("/api/v1/filaments", server.handleFilaments)
	mux.HandleFunc("/api/v1/print_jobs", server.handlePrintJobs)
	// Note: For handling specific job IDs in the URL, a more advanced router
	// like gorilla/mux would be better, but we'll stick to standard library
	// and parse the path manually for this example.
	mux.HandleFunc("/api/v1/print_jobs/", server.handleJobStatus)


	return server
}

// Start begins listening for HTTP requests.
func (a *APIServer) Start() error {
	// Use the configured multiplexer
	return http.ListenAndServe(a.addr, a.mux)
}

// handleJoin handles requests for joining this node to an existing cluster.
func (a *APIServer) handleJoin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID      string `json:"id"`      // The joining node's Raft ID
		Address string `json:"address"` // The joining node's Raft address
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode join request body: %v", err), http.StatusBadRequest)
		return
	}

	// Add the joining node as a voter to this node's Raft configuration.
	// The Raft library will handle communication with the joining node.
	future := a.node.Raft.AddVoter(
		models.ServerID(req.ID),
		models.ServerAddress(req.Address),
		0, 0) // Use 0 for index and term
	if err := future.Error(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to add voter: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Node added successfully")
}

// handleHealthCheck provides a simple health check endpoint.
func (a *APIServer) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check the state of the Raft node
	state := a.node.Raft.State()
	// Access the public ID field directly
	fmt.Fprintf(w, "Node %s is healthy. Raft state: %s\n", a.node.ID, state.String())
}

// handlePrinters handles requests for managing printers.
func (a *APIServer) handlePrinters(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Read-only operation, can be served locally from the FSM
		fsm := a.node.GetFSM()
		fsm.Mu.RLock() // Use exported Mu
		defer fsm.Mu.RUnlock() // Use exported Mu
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(fsm.Printers); err != nil { // Use exported Printers
			http.Error(w, fmt.Sprintf("Failed to encode printers: %v", err), http.StatusInternalServerError)
		}

	case http.MethodPost:
		// Write operation, must go through Raft consensus
		if a.node.Raft.State() != hraft.Leader { // Use hraft.Leader
			http.Error(w, "Not the leader", http.StatusTemporaryRedirect) // Or redirect to leader
			return
		}

		var p models.Printer
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, fmt.Sprintf("Failed to decode printer request body: %v", err), http.StatusBadRequest)
			return
		}

		// Create the command to be applied to the FSM
		cmdData, err := json.Marshal(p) // Marshal the Printer struct to JSON
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to marshal printer data: %v", err), http.StatusInternalServerError)
			return
		}

		cmd := models.Command{
			Op:   "add_printer",
			Data: json.RawMessage(cmdData), // Use the marshaled data
		}

		// Apply the command through Raft consensus
		if err := a.applyCommand(cmd); err != nil {
			http.Error(w, fmt.Sprintf("Failed to apply command: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, "Printer added successfully")

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFilaments handles requests for managing filaments.
func (a *APIServer) handleFilaments(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Read-only operation
		a.node.FSM.Mu.RLock() // Use exported Mu
		defer a.node.FSM.Mu.RUnlock() // Use exported Mu
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(a.node.FSM.Filaments); err != nil { // Use exported Filaments
			http.Error(w, fmt.Sprintf("Failed to encode filaments: %v", err), http.StatusInternalServerError)
		}

	case http.MethodPost:
		// Write operation
		if a.node.Raft.State() != hraft.Leader { // Use hraft.Leader
			http.Error(w, "Not the leader", http.StatusTemporaryRedirect) // Or redirect to leader
			return
		}

		var fl models.Filament
		if err := json.NewDecoder(r.Body).Decode(&fl); err != nil {
			http.Error(w, fmt.Sprintf("Failed to decode filament request body: %v", err), http.StatusBadRequest)
			return
		}

		cmdData, err := json.Marshal(fl) // Marshal the Filament struct to JSON
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to marshal filament data: %v", err), http.StatusInternalServerError)
			return
		}

		cmd := models.Command{
			Op:   "add_filament",
			Data: json.RawMessage(cmdData), // Use the marshaled data
		}

		if err := a.applyCommand(cmd); err != nil {
			http.Error(w, fmt.Sprintf("Failed to apply command: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, "Filament added successfully")

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePrintJobs handles requests for managing print jobs.
func (a *APIServer) handlePrintJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Read-only operation
		a.node.FSM.Mu.RLock() // Use exported Mu
		defer a.node.FSM.Mu.RUnlock() // Use exported Mu
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(a.node.FSM.Jobs); err != nil { // Use exported Jobs
			http.Error(w, fmt.Sprintf("Failed to encode print jobs: %v", err), http.StatusInternalServerError)
		}

	case http.MethodPost:
		// Write operation
		if a.node.Raft.State() != hraft.Leader { // Use hraft.Leader
			http.Error(w, "Not the leader", http.StatusTemporaryRedirect) // Or redirect to leader
			return
		}

		var job models.PrintJob
		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
			http.Error(w, fmt.Sprintf("Failed to decode print job request body: %v", err), http.StatusBadRequest)
			return
		}

		// Set initial status to "Queued" as per requirements
		job.Status = "Queued"

		cmdData, err := json.Marshal(job) // Marshal the PrintJob struct to JSON
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to marshal print job data: %v", err), http.StatusInternalServerError)
			return
		}

		cmd := models.Command{
			Op:   "add_job",
			Data: json.RawMessage(cmdData), // Use the marshaled data
		}

		if err := a.applyCommand(cmd); err != nil {
			// Check if the error is a validation error from the FSM
			if strings.Contains(err.Error(), "job validation failed") {
				http.Error(w, err.Error(), http.StatusBadRequest) // Return 400 for validation errors
			} else {
				http.Error(w, fmt.Sprintf("Failed to apply command: %v", err), http.StatusInternalServerError)
			}
			return
		}

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, "Print job added successfully")

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleJobStatus handles requests to update the status of a print job.
// Expects a POST request to /api/v1/print_jobs/{jobID} with a JSON body
// containing the new status.
func (a *APIServer) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract jobID from the URL path (e.g., /api/v1/print_jobs/job123)
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) != 5 || parts[4] == "" {
		http.Error(w, "Invalid URL format. Expected /api/v1/print_jobs/{jobID}", http.StatusBadRequest)
		return
	}
	jobID := parts[4]

	var req struct {
		Status string `json:"status"` // The new status for the job
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode status update request body: %v", err), http.StatusBadRequest)
		return
	}

	// Create the command to update the job status
	cmdData, err := json.Marshal(map[string]string{
		"id":     jobID,
		"status": req.Status,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to marshal status update data: %v", err), http.StatusInternalServerError)
		return
	}

	cmd := models.Command{
		Op:   "update_job_status",
		Data: json.RawMessage(cmdData), // Use the marshaled data
	}

	// Apply the command through Raft consensus
	if err := a.applyCommand(cmd); err != nil {
		// Check if the error is related to job not found or invalid status transition
		if strings.Contains(err.Error(), "job with ID") || strings.Contains(err.Error(), "invalid status transition") {
			http.Error(w, err.Error(), http.StatusBadRequest) // Return 400 for validation errors
		} else {
			http.Error(w, fmt.Sprintf("Failed to apply command: %v", err), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Print job %s status updated to %s\n", jobID, req.Status)
}

// applyCommand applies a command to the Raft cluster.
// It ensures the command is sent to the leader.
func (a *APIServer) applyCommand(cmd models.Command) error {
	// Check if this node is the leader. If not, the Raft library will
	// forward the command to the leader, but it's good practice to check.
	if a.node.Raft.State() != hraft.Leader { // Use hraft.Leader
		// In a real application, you would redirect the client to the leader's address.
		return fmt.Errorf("not the leader")
	}

	// Marshal the command into a byte slice for Raft
	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	// Apply the command to the Raft cluster.
	// The command will be replicated to followers and applied to the FSM.
	future := a.node.Raft.Apply(data, 5*time.Second) // Apply with a timeout

	// Check for errors in the application process.
	// This includes network errors, leadership changes, and errors returned by the FSM's Apply method.
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to apply command through Raft: %w", err)
	}

	// Check if the FSM's Apply method returned an error
	response := future.Response()
	if response != nil {
		if applyErr, ok := response.(error); ok {
			return fmt.Errorf("FSM apply returned error: %w", applyErr)
		}
	}


	return nil // Command applied successfully
}
