# 🖨️ Raft3D - 3D Prints Management using Raft Consensus Algorithm

## 📦 API Endpoints and Sample `curl` Commands

### 1. Register a New Printer
    curl -X POST http://localhost:8083/api/v1/printers \
      -H "Content-Type: application/json" \
      -d '{"id": "1", "company": "XYZ Corp", "model": "3D Model X"}'


### 2. Register a New Filament
    curl -X POST http://localhost:8081/api/v1/filaments \
      -H "Content-Type: application/json" \
      -d '{"id": "1", "type": "PLA", "color": "Red", "total_weight_in_grams": 1000, "remaining_weight_in_grams": 800}'

### 3. Register a New Print_Job
    curl -X POST http://localhost:8081/api/v1/print_jobs \
      -H "Content-Type: application/json" \
      -d '{"ID": "1", "printer_id": "1", "filament_id": "1", "filepath": "demo_model.stl", "print_weight_in_grams": 100, "status": "Queued"}'

### 4. View All Printers
    curl -X GET http://localhost:8081/api/v1/printers

### 5. View All Filaments
    curl -X GET http://localhost:8081/api/v1/filaments

### 6. View All Print_Jobs
    curl -X GET http://localhost:8081/api/v1/print_jobs

### 7. Update Status to Running
    curl -X POST http://localhost:8081/api/v1/print_jobs/1 \
      -H "Content-Type: application/json" \
      -d '{"status":"Running"}'

### 8. Update Status to Done
    curl -X POST http://localhost:8081/api/v1/print_jobs/1 \
      -H "Content-Type: application/json" \
      -d '{"status":"Done"}'

### 9. Now check if the "remaining_weight_in_grams" of filaments is reduced.

### 10. Terminate the leader node. A new leader will be elected.
  The printer created by the old leader is still available.

## 🤝 Contributors
- Trishita Umapathi – trishitaumapathi@gmail.com
- Deepak Velmurugan – imdeepakv@gmail.com
- Vandana J – vandanaj0110@gmail.com
- Vignesh Palanirajan – vignesh.palanirajan31@gmail.com
