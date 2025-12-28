//go:build !wails

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"work-time-tracker-go/tracker"
)

var (
	db                      *sql.DB
	activeWatcher           *tracker.TimeWatcher
	currentProject          *tracker.Project
	currentActivity         *string
	trackingRunning         bool
	currentPendingSessionID int64 // ID della sessione pendente per auto-save
)

// SetGlobalTrackingState aggiorna lo stato globale del tracking
// Usato dalla tray per sincronizzare lo stato con il web server
func SetGlobalTrackingState(watcher *tracker.TimeWatcher, project *tracker.Project, running bool, pendingSessionID int64) {
	activeWatcher = watcher
	currentProject = project
	trackingRunning = running
	currentPendingSessionID = pendingSessionID
}

// GetGlobalTrackingState ritorna lo stato corrente del tracking
func GetGlobalTrackingState() (watcher *tracker.TimeWatcher, project *tracker.Project, running bool) {
	return activeWatcher, currentProject, trackingRunning
}

// StartWebServer avvia il server web
func StartWebServer(database *sql.DB) {
	db = database

	// Serve file statici
	fs := http.FileServer(http.Dir("./web/static"))
	http.Handle("/", fs)

	// API endpoints
	http.HandleFunc("/api/projects", handleProjects)
	http.HandleFunc("/api/projects/create", handleCreateProject)
	http.HandleFunc("/api/projects/delete", handleDeleteProject)
	http.HandleFunc("/api/tracking/start", handleStartTracking)
	http.HandleFunc("/api/tracking/stop", handleStopTracking)
	http.HandleFunc("/api/tracking/status", handleTrackingStatus)
	http.HandleFunc("/api/tracking/idle-check", handleIdleCheck)
	http.HandleFunc("/api/tracking/attribute-idle", handleAttributeIdle)
	http.HandleFunc("/api/notes/create", handleCreateNote)
	http.HandleFunc("/api/notes/timeline", handleNotesTimeline)
	http.HandleFunc("/api/notes/all", handleAllNotes)
	http.HandleFunc("/api/notes/update", handleUpdateNote)
	http.HandleFunc("/api/notes/delete", handleDeleteNote)
	http.HandleFunc("/api/sessions/update-activity", handleUpdateActivityType)
	http.HandleFunc("/api/sessions/delete", handleDeleteSession)
	http.HandleFunc("/api/sessions/update-duration", handleUpdateDuration)
	http.HandleFunc("/api/sessions/split", handleSplitSession)
	http.HandleFunc("/api/sessions/create", handleCreateSession)
	http.HandleFunc("/api/stats/today", handleStatsToday)
	http.HandleFunc("/api/stats/project", handleStatsProject)
	http.HandleFunc("/api/stats/timeline", handleTimeline)
	http.HandleFunc("/api/activity-types", handleGetActivityTypes)
	http.HandleFunc("/api/activity-types/create", handleCreateActivityType)
	http.HandleFunc("/api/activity-types/update", handleUpdateActivityTypeConfig)
	http.HandleFunc("/api/activity-types/delete", handleDeleteActivityType)
	http.HandleFunc("/api/activity-types/reorder", handleReorderActivityTypes)
	http.HandleFunc("/api/projects/archive", handleArchiveProject)
	http.HandleFunc("/api/projects/reactivate", handleReactivateProject)
	http.HandleFunc("/api/projects/report", handleProjectReport)
	http.HandleFunc("/api/projects/archived", handleArchivedProjects)
	http.HandleFunc("/api/projects/delete-archived", handleDeleteArchivedProject)
	http.HandleFunc("/api/settings/get", handleGetSetting)
	http.HandleFunc("/api/settings/set", handleSetSetting)
	http.HandleFunc("/api/data/export", handleExportData)
	http.HandleFunc("/api/data/import", handleImportData)

	// Avvia server
	port := ":8080"
	fmt.Printf("\n=== PRENDITEMPO ===\n")
	fmt.Printf("Server avviato su http://localhost%s\n", port)
	fmt.Printf("Apri il browser e vai a: http://localhost%s/index_v2.html\n\n", port)

	// Apri browser automaticamente con la dashboard
	openBrowser("http://localhost" + port + "/index_v2.html")

	log.Fatal(http.ListenAndServe(port, nil))
}

// openBrowser apre il browser predefinito in una nuova finestra
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		// Usa PowerShell per aprire in una nuova finestra del browser predefinito
		psCmd := fmt.Sprintf(`
			$browserPath = (Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\Shell\Associations\UrlAssociations\http\UserChoice').ProgId
			if ($browserPath -match 'Chrome') {
				Start-Process chrome.exe -ArgumentList '--new-window','%s'
			} elseif ($browserPath -match 'Firefox') {
				Start-Process firefox.exe -ArgumentList '-new-window','%s'
			} elseif ($browserPath -match 'Edge') {
				Start-Process msedge.exe -ArgumentList '--new-window','%s'
			} elseif ($browserPath -match 'Brave') {
				Start-Process brave.exe -ArgumentList '--new-window','%s'
			} else {
				Start-Process '%s'
			}
		`, url, url, url, url, url)
		err = exec.Command("powershell", "-WindowStyle", "Hidden", "-Command", psCmd).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		log.Printf("Errore apertura browser: %v", err)
	}
}

// === API HANDLERS ===

// handleProjects restituisce la lista di tutti i progetti
func handleProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	allProjects, err := tracker.CaricaTuttiProgetti(db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filtra solo i progetti non archiviati
	var activeProjects []tracker.Project
	for _, p := range allProjects {
		if !p.Archived {
			activeProjects = append(activeProjects, p)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activeProjects)
}

// handleArchivedProjects restituisce la lista dei progetti archiviati
func handleArchivedProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	allProjects, err := tracker.CaricaTuttiProgetti(db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Filtra solo i progetti archiviati
	var archivedProjects []tracker.Project
	for _, p := range allProjects {
		if p.Archived {
			archivedProjects = append(archivedProjects, p)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(archivedProjects)
}

// handleCreateProject crea un nuovo progetto
func handleCreateProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := tracker.CreaProgetto(db, req.Name, req.Description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"id":      id,
		"message": "Progetto creato con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDeleteProject elimina un progetto
func handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := tracker.EliminaProgetto(db, req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Progetto eliminato con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleArchiveProject archivia un progetto
func handleArchiveProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := tracker.ArchivaProgetto(db, req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Genera automaticamente il report di chiusura
	report, err := tracker.GeneraReportChiusura(db, req.ID)
	if err != nil {
		http.Error(w, "Progetto archiviato ma errore generazione report: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Progetto archiviato con successo",
		"report":  report,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleReactivateProject riattiva un progetto archiviato
func handleReactivateProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := tracker.RiattivaProgetto(db, req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Progetto riattivato con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDeleteArchivedProject elimina definitivamente un progetto archiviato
func handleDeleteArchivedProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Verifica che il progetto sia archiviato prima di eliminarlo
	project, err := tracker.TrovaProgettoById(db, req.ID)
	if err != nil {
		http.Error(w, "Progetto non trovato", http.StatusNotFound)
		return
	}

	if !project.Archived {
		http.Error(w, "Solo i progetti archiviati possono essere eliminati definitivamente", http.StatusBadRequest)
		return
	}

	// Elimina il progetto
	err = tracker.EliminaProgettoById(db, req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Progetto eliminato definitivamente",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleProjectReport genera il report di chiusura di un progetto
func handleProjectReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectIDStr := r.URL.Query().Get("id")
	if projectIDStr == "" {
		http.Error(w, "ID progetto mancante", http.StatusBadRequest)
		return
	}

	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		http.Error(w, "ID progetto non valido", http.StatusBadRequest)
		return
	}

	report, err := tracker.GeneraReportChiusura(db, projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// handleStartTracking avvia il tracking
func handleStartTracking(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ProjectName   string  `json:"project_name"`
		ActivityType  *string `json:"activity_type"`
		IdleThreshold int     `json:"idle_threshold"` // in minuti
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Controlla se già in tracking
	if trackingRunning {
		http.Error(w, "Tracking già in corso", http.StatusBadRequest)
		return
	}

	// Trova progetto
	project, err := tracker.TrovaProgetto(db, req.ProjectName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Default idle threshold: 5 minuti
	if req.IdleThreshold <= 0 {
		req.IdleThreshold = 5
	}

	// Crea sessione pendente per auto-save
	startTime := fmt.Sprintf("%s", time.Now().Format("2006-01-02 15:04:05"))
	pendingSessionID, err := tracker.StartPendingTracking(db, &project.ID, req.ActivityType, startTime)
	if err != nil {
		http.Error(w, "Errore avvio pending tracking: "+err.Error(), http.StatusInternalServerError)
		return
	}
	currentPendingSessionID = pendingSessionID

	// Avvia tracking
	activeWatcher = tracker.NewTimeWatcher()
	activeWatcher.SetIdleThreshold(req.IdleThreshold * 60) // converti in secondi

	// Imposta callback per salvataggio periodico (ogni 5 minuti = 300 secondi)
	activeWatcher.SetSaveCallback(func(totalSeconds int) error {
		return tracker.UpdatePendingTracking(db, currentPendingSessionID, totalSeconds)
	}, 300)

	activeWatcher.Start(5)

	currentProject = project
	currentActivity = req.ActivityType
	trackingRunning = true

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Tracking avviato per progetto: %s (auto-save attivo)", project.Name),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStopTracking ferma il tracking
func handleStopTracking(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !trackingRunning {
		http.Error(w, "Nessun tracking attivo", http.StatusBadRequest)
		return
	}

	// Ferma watcher
	activeWatcher.Stop()

	// Finalizza la sessione pendente con i secondi finali
	finalSeconds := activeWatcher.GetTotalActiveSeconds()
	if currentPendingSessionID > 0 {
		err := tracker.FinalizePendingTracking(db, currentPendingSessionID, finalSeconds)
		if err != nil {
			http.Error(w, "Errore finalizzazione sessione: "+err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Printf("[SAVE] Sessione finalizzata: ID %d con %d secondi (%d min)\n",
			currentPendingSessionID, finalSeconds, finalSeconds/60)
		currentPendingSessionID = 0
	}

	trackingRunning = false

	// Per compatibilità, restituisci anche le stats aggregate
	stats := activeWatcher.GetStats()
	response := map[string]interface{}{
		"success": true,
		"message": "Tracking fermato e dati salvati",
		"stats":   stats,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleTrackingStatus restituisce lo stato del tracking
func handleTrackingStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"running": trackingRunning,
	}

	if trackingRunning && currentProject != nil {
		response["project"] = currentProject.Name
		response["stats"] = activeWatcher.GetStats()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStatsToday restituisce le statistiche di oggi
func handleStatsToday(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := tracker.CaricaSessioniOggi(db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleStatsProject restituisce le statistiche di un progetto
func handleStatsProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectIDStr := r.URL.Query().Get("id")
	projectID, err := strconv.Atoi(projectIDStr)
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	stats, err := tracker.CaricaSessioniProgetto(db, projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Calcola totale
	total := 0
	for _, sec := range stats {
		total += sec
	}

	response := map[string]interface{}{
		"stats":         stats,
		"total_seconds": total,
		"total_hours":   float64(total) / 3600,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleTimeline restituisce le sessioni dettagliate per la timeline
func handleTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parametri: startDate, endDate (formato: YYYY-MM-DD)
	startDate := r.URL.Query().Get("start")
	endDate := r.URL.Query().Get("end")

	// Se non specificati, usa oggi
	if startDate == "" || endDate == "" {
		startDate = "2025-01-01" // Default ampio per vedere dati storici
		endDate = "2099-12-31"
	}

	sessions, err := tracker.CaricaSessioniDettagliate(db, startDate, endDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// handleIdleCheck verifica se c'è un periodo idle pendente
func handleIdleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"has_pending": false,
	}

	if trackingRunning && activeWatcher != nil && activeWatcher.HasPendingIdlePeriod() {
		idlePeriod := activeWatcher.GetPendingIdlePeriod()
		response["has_pending"] = true
		response["idle_period"] = map[string]interface{}{
			"start_time": idlePeriod.StartTime.Format("2006-01-02 15:04:05"),
			"end_time":   idlePeriod.EndTime.Format("2006-01-02 15:04:05"),
			"duration":   idlePeriod.Duration,
			"minutes":    idlePeriod.Duration / 60,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleAttributeIdle attribuisce il tempo idle a un progetto
func handleAttributeIdle(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ProjectName string `json:"project_name"`
		IsBreak     bool   `json:"is_break"` // se true, non attribuire a nessun progetto
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !trackingRunning || activeWatcher == nil {
		http.Error(w, "Nessun tracking attivo", http.StatusBadRequest)
		return
	}

	if !activeWatcher.HasPendingIdlePeriod() {
		http.Error(w, "Nessun periodo idle pendente", http.StatusBadRequest)
		return
	}

	idlePeriod := activeWatcher.GetPendingIdlePeriod()

	// Se non è una pausa, salva la sessione
	if !req.IsBreak && req.ProjectName != "" {
		// Trova il progetto
		project, err := tracker.TrovaProgetto(db, req.ProjectName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		// Salva come sessione "off-computer"
		err = tracker.SalvaSessioneConTipo(
			db,
			"Lavoro Off-Computer",
			idlePeriod.Duration,
			&project.ID,
			"off-computer",
			nil, // activity_type non applicabile per off-computer
			idlePeriod.StartTime.Format("2006-01-02 15:04:05"),
		)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Rimuovi il periodo idle pendente
	activeWatcher.ClearPendingIdlePeriod()

	response := map[string]interface{}{
		"success": true,
		"message": "Periodo idle attribuito con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleCreateNote crea una nuova nota
func handleCreateNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ProjectName string `json:"project_name"`
		NoteText    string `json:"note_text"`
		Timestamp   string `json:"timestamp"` // opzionale
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.NoteText == "" {
		http.Error(w, "Il testo della nota è obbligatorio", http.StatusBadRequest)
		return
	}

	// Trova il progetto
	project, err := tracker.TrovaProgetto(db, req.ProjectName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Crea la nota
	noteID, err := tracker.CreaNote(db, project.ID, req.NoteText, req.Timestamp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"id":      noteID,
		"message": "Nota creata con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleNotesTimeline restituisce le note per il periodo della timeline
func handleNotesTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	startDate := r.URL.Query().Get("start")
	endDate := r.URL.Query().Get("end")

	if startDate == "" || endDate == "" {
		startDate = "2025-01-01"
		endDate = "2099-12-31"
	}

	notes, err := tracker.CaricaNotePerPeriodo(db, startDate, endDate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notes)
}

// handleAllNotes restituisce tutte le note con filtri opzionali
func handleAllNotes(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.URL.Query().Get("project_id")
	searchText := r.URL.Query().Get("search")
	limit := r.URL.Query().Get("limit")

	if limit == "" {
		limit = "50" // Default: ultime 50 note
	}

	notes, err := tracker.CaricaTutteLeNote(db, projectID, searchText, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notes)
}

// handleUpdateNote aggiorna il testo di una nota esistente
func handleUpdateNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID       int    `json:"id"`
		NoteText string `json:"note_text"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.NoteText == "" {
		http.Error(w, "Il testo della nota è obbligatorio", http.StatusBadRequest)
		return
	}

	err := tracker.AggiornaNota(db, req.ID, req.NoteText)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Nota aggiornata con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDeleteNote elimina una nota
func handleDeleteNote(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := tracker.EliminaNota(db, req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Nota eliminata con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleUpdateActivityType aggiorna il tipo di attività di una sessione
func handleUpdateActivityType(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID    int     `json:"session_id"`
		ActivityType *string `json:"activity_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Aggiorna activity_type
	err := tracker.AggiornaActivityType(db, req.SessionID, req.ActivityType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Tipo attività aggiornato con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDeleteSession elimina una sessione
func handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID int `json:"session_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := tracker.EliminaSessione(db, req.SessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Sessione eliminata con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleUpdateDuration aggiorna la durata di una sessione
func handleUpdateDuration(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID int `json:"session_id"`
		Seconds   int `json:"seconds"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Seconds <= 0 {
		http.Error(w, "La durata deve essere maggiore di 0", http.StatusBadRequest)
		return
	}

	err := tracker.AggiornaDurataSessione(db, req.SessionID, req.Seconds)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Durata aggiornata con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSplitSession divide una sessione in due parti
func handleSplitSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID              int     `json:"session_id"`
		FirstPartSeconds       int     `json:"first_part_seconds"`
		FirstPartActivityType  *string `json:"first_part_activity_type"`
		SecondPartActivityType *string `json:"second_part_activity_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.FirstPartSeconds <= 0 {
		http.Error(w, "La durata della prima parte deve essere maggiore di 0", http.StatusBadRequest)
		return
	}

	err := tracker.DividiSessione(db, req.SessionID, req.FirstPartSeconds, req.FirstPartActivityType, req.SecondPartActivityType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Sessione divisa con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleCreateSession crea una nuova sessione manuale
func handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		AppName      string  `json:"app_name"`
		Seconds      int     `json:"seconds"`
		ProjectID    *int    `json:"project_id"`
		SessionType  string  `json:"session_type"`
		ActivityType *string `json:"activity_type"`
		Timestamp    string  `json:"timestamp"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validazioni
	if req.AppName == "" {
		http.Error(w, "Il nome dell'app è obbligatorio", http.StatusBadRequest)
		return
	}

	if req.Seconds <= 0 {
		http.Error(w, "La durata deve essere maggiore di 0", http.StatusBadRequest)
		return
	}

	if req.SessionType == "" {
		req.SessionType = "computer" // Default
	}

	if req.Timestamp == "" {
		http.Error(w, "Il timestamp è obbligatorio", http.StatusBadRequest)
		return
	}

	err := tracker.CreaSessione(db, req.AppName, req.Seconds, req.ProjectID, req.SessionType, req.ActivityType, req.Timestamp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Sessione creata con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetActivityTypes restituisce tutti i tipi di attività
func handleGetActivityTypes(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	types, err := tracker.CaricaTipiAttivita(db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(types)
}

// handleCreateActivityType crea un nuovo tipo di attività
func handleCreateActivityType(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name         string  `json:"name"`
		ColorVariant float64 `json:"color_variant"`
		Pattern      string  `json:"pattern"`
		DisplayOrder int     `json:"display_order"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Il nome è obbligatorio", http.StatusBadRequest)
		return
	}

	if req.Pattern == "" {
		req.Pattern = "solid" // Default
	}

	id, err := tracker.CreaTipoAttivita(db, req.Name, req.ColorVariant, req.Pattern, req.DisplayOrder)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Tipo attività creato con successo",
		"id":      id,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleUpdateActivityTypeConfig aggiorna un tipo di attività
func handleUpdateActivityTypeConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID           int     `json:"id"`
		Name         string  `json:"name"`
		ColorVariant float64 `json:"color_variant"`
		Pattern      string  `json:"pattern"`
		DisplayOrder int     `json:"display_order"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Il nome è obbligatorio", http.StatusBadRequest)
		return
	}

	if req.Pattern == "" {
		req.Pattern = "solid" // Default
	}

	err := tracker.AggiornaTipoAttivita(db, req.ID, req.Name, req.ColorVariant, req.Pattern, req.DisplayOrder)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Tipo attività aggiornato con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDeleteActivityType elimina un tipo di attività
func handleDeleteActivityType(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := tracker.EliminaTipoAttivita(db, req.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Tipo attività eliminato con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleReorderActivityTypes riordina i tipi di attività
func handleReorderActivityTypes(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Types []struct {
			ID           int `json:"id"`
			DisplayOrder int `json:"display_order"`
		} `json:"types"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Aggiorna l'ordine di ogni tipo
	for _, t := range req.Types {
		err := tracker.AggiornaOrdineTipoAttivita(db, t.ID, t.DisplayOrder)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Ordine aggiornato con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// === SETTINGS HANDLERS ===

// handleGetSetting gestisce la richiesta per ottenere un'impostazione
func handleGetSetting(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Chiave mancante", http.StatusBadRequest)
		return
	}

	value, err := tracker.GetSetting(db, key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"key":   key,
		"value": value,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSetSetting gestisce la richiesta per salvare un'impostazione
func handleSetSetting(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Dati non validi", http.StatusBadRequest)
		return
	}

	if request.Key == "" {
		http.Error(w, "Chiave mancante", http.StatusBadRequest)
		return
	}

	if err := tracker.SetSetting(db, request.Key, request.Value); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Se l'impostazione è auto_start, gestisci l'avvio automatico di Windows
	if request.Key == "auto_start" {
		if request.Value == "true" {
			if err := EnableAutoStart(); err != nil {
				log.Printf("[AUTOSTART] Errore abilitazione: %v", err)
				http.Error(w, "Errore abilitazione avvio automatico: "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			if err := DisableAutoStart(); err != nil {
				log.Printf("[AUTOSTART] Errore disabilitazione: %v", err)
				http.Error(w, "Errore disabilitazione avvio automatico: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	response := map[string]interface{}{
		"success": true,
		"message": "Impostazione salvata con successo",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// === EXPORT/IMPORT DATA ===

// ExportData struttura per l'export dei dati
type ExportData struct {
	ExportDate     string                   `json:"export_date"`
	Version        string                   `json:"version"`
	Projects       []map[string]interface{} `json:"projects"`
	Sessions       []map[string]interface{} `json:"sessions"`
	Notes          []map[string]interface{} `json:"notes"`
	ActivityTypes  []map[string]interface{} `json:"activity_types"`
	Settings       []map[string]interface{} `json:"settings"`
}

// handleExportData esporta tutti i dati in formato JSON
func handleExportData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	export := ExportData{
		ExportDate: time.Now().Format("2006-01-02 15:04:05"),
		Version:    "1.0",
	}

	// Esporta progetti
	rows, err := db.Query("SELECT id, name, description, created_at, archived, COALESCE(closed_at, '') FROM projects")
	if err != nil {
		http.Error(w, "Errore export progetti: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name, description, createdAt, closedAt string
		var archived int
		rows.Scan(&id, &name, &description, &createdAt, &archived, &closedAt)
		project := map[string]interface{}{
			"id":          id,
			"name":        name,
			"description": description,
			"created_at":  createdAt,
			"archived":    archived == 1,
		}
		if closedAt != "" {
			project["closed_at"] = closedAt
		}
		export.Projects = append(export.Projects, project)
	}

	// Esporta sessioni
	rows, err = db.Query("SELECT id, app_name, seconds, project_id, session_type, activity_type, timestamp FROM sessions")
	if err != nil {
		http.Error(w, "Errore export sessioni: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, seconds int
		var appName, sessionType, timestamp string
		var projectID sql.NullInt64
		var activityType sql.NullString
		rows.Scan(&id, &appName, &seconds, &projectID, &sessionType, &activityType, &timestamp)
		session := map[string]interface{}{
			"id":           id,
			"app_name":     appName,
			"seconds":      seconds,
			"session_type": sessionType,
			"timestamp":    timestamp,
		}
		if projectID.Valid {
			session["project_id"] = projectID.Int64
		}
		if activityType.Valid {
			session["activity_type"] = activityType.String
		}
		export.Sessions = append(export.Sessions, session)
	}

	// Esporta note
	rows, err = db.Query("SELECT id, project_id, note_text, timestamp FROM notes")
	if err != nil {
		http.Error(w, "Errore export note: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, projectID int
		var noteText, timestamp string
		rows.Scan(&id, &projectID, &noteText, &timestamp)
		export.Notes = append(export.Notes, map[string]interface{}{
			"id":         id,
			"project_id": projectID,
			"note_text":  noteText,
			"timestamp":  timestamp,
		})
	}

	// Esporta tipi attività
	rows, err = db.Query("SELECT id, name, color_variant, pattern, display_order FROM activity_types")
	if err != nil {
		http.Error(w, "Errore export tipi attività: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, displayOrder int
		var name, pattern string
		var colorVariant float64
		rows.Scan(&id, &name, &colorVariant, &pattern, &displayOrder)
		export.ActivityTypes = append(export.ActivityTypes, map[string]interface{}{
			"id":            id,
			"name":          name,
			"color_variant": colorVariant,
			"pattern":       pattern,
			"display_order": displayOrder,
		})
	}

	// Esporta impostazioni
	rows, err = db.Query("SELECT key, value FROM settings")
	if err != nil {
		http.Error(w, "Errore export impostazioni: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		rows.Scan(&key, &value)
		export.Settings = append(export.Settings, map[string]interface{}{
			"key":   key,
			"value": value,
		})
	}

	// Invia come file JSON scaricabile
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=prenditempo_backup_%s.json", time.Now().Format("2006-01-02")))
	json.NewEncoder(w).Encode(export)
}

// handleImportData importa i dati da un file JSON
func handleImportData(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var importData ExportData
	if err := json.NewDecoder(r.Body).Decode(&importData); err != nil {
		http.Error(w, "Errore parsing JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Inizia una transazione
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Errore avvio transazione: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Elimina i dati esistenti (in ordine per rispettare le foreign keys)
	tx.Exec("DELETE FROM pending_tracking")
	tx.Exec("DELETE FROM notes")
	tx.Exec("DELETE FROM sessions")
	tx.Exec("DELETE FROM projects")
	tx.Exec("DELETE FROM activity_types")

	// Mappa vecchi ID -> nuovi ID per i progetti
	projectIDMap := make(map[int]int64)

	// Importa progetti
	for _, p := range importData.Projects {
		name := p["name"].(string)
		description := ""
		if d, ok := p["description"].(string); ok {
			description = d
		}
		archived := 0
		if a, ok := p["archived"].(bool); ok && a {
			archived = 1
		}
		var closedAt interface{}
		if c, ok := p["closed_at"].(string); ok && c != "" {
			closedAt = c
		}

		result, err := tx.Exec(
			"INSERT INTO projects (name, description, archived, closed_at) VALUES (?, ?, ?, ?)",
			name, description, archived, closedAt,
		)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Errore import progetto: "+err.Error(), http.StatusInternalServerError)
			return
		}
		newID, _ := result.LastInsertId()
		oldID := int(p["id"].(float64))
		projectIDMap[oldID] = newID
	}

	// Importa sessioni con i nuovi ID progetto
	for _, s := range importData.Sessions {
		appName := s["app_name"].(string)
		seconds := int(s["seconds"].(float64))
		sessionType := s["session_type"].(string)
		timestamp := s["timestamp"].(string)

		var projectID *int64
		if pid, ok := s["project_id"].(float64); ok {
			if newPID, exists := projectIDMap[int(pid)]; exists {
				projectID = &newPID
			}
		}

		var activityType *string
		if at, ok := s["activity_type"].(string); ok {
			activityType = &at
		}

		_, err := tx.Exec(
			"INSERT INTO sessions (app_name, seconds, project_id, session_type, activity_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)",
			appName, seconds, projectID, sessionType, activityType, timestamp,
		)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Errore import sessione: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Importa note con i nuovi ID progetto
	for _, n := range importData.Notes {
		oldProjectID := int(n["project_id"].(float64))
		noteText := n["note_text"].(string)
		timestamp := n["timestamp"].(string)

		newProjectID, exists := projectIDMap[oldProjectID]
		if !exists {
			continue // Salta note senza progetto valido
		}

		_, err := tx.Exec(
			"INSERT INTO notes (project_id, note_text, timestamp) VALUES (?, ?, ?)",
			newProjectID, noteText, timestamp,
		)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Errore import nota: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Importa tipi attività
	for _, at := range importData.ActivityTypes {
		name := at["name"].(string)
		colorVariant := at["color_variant"].(float64)
		pattern := "solid"
		if p, ok := at["pattern"].(string); ok {
			pattern = p
		}
		displayOrder := 0
		if d, ok := at["display_order"].(float64); ok {
			displayOrder = int(d)
		}

		_, err := tx.Exec(
			"INSERT INTO activity_types (name, color_variant, pattern, display_order) VALUES (?, ?, ?, ?)",
			name, colorVariant, pattern, displayOrder,
		)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Errore import tipo attività: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Commit transazione
	if err := tx.Commit(); err != nil {
		http.Error(w, "Errore commit transazione: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success":           true,
		"message":           "Dati importati con successo",
		"projects_imported": len(importData.Projects),
		"sessions_imported": len(importData.Sessions),
		"notes_imported":    len(importData.Notes),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
