package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"work-time-tracker-go/tracker"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// === SALVATAGGIO REPORT ===

// SaveReportJSON salva il report in formato JSON
func (a *App) SaveReportJSON(projectID int) (string, error) {
	// Genera il report
	report, err := tracker.GeneraReportChiusura(a.db, projectID)
	if err != nil {
		return "", err
	}

	// Aggiungi metadati per reimportazione
	report["export_type"] = "project_report"
	report["export_date"] = time.Now().Format("2006-01-02 15:04:05")
	report["version"] = "1.0"

	// Recupera sessioni del progetto per backup completo
	sessions, err := tracker.CaricaSessioniDettagliatePerProgetto(a.db, projectID)
	if err == nil {
		var projectSessions []map[string]interface{}
		for _, s := range sessions {
			projectSessions = append(projectSessions, map[string]interface{}{
				"id":            s.ID,
				"app_name":      s.AppName,
				"seconds":       s.Seconds,
				"project_id":    s.ProjectID,
				"session_type":  s.SessionType,
				"activity_type": s.ActivityType,
				"timestamp":     s.Timestamp,
			})
		}
		report["sessions"] = projectSessions
	}

	// Converti in JSON
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}

	// Chiedi all'utente dove salvare
	projectName := report["project_name"].(string)
	defaultName := fmt.Sprintf("Report_%s_%s.json", safeFilenamePart(projectName), time.Now().Format("2006-01-02"))

	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultName,
		Title:           "Salva Report JSON",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil // Utente ha annullato
	}

	// Salva il file
	err = os.WriteFile(filePath, jsonData, 0600)
	if err != nil {
		return "", err
	}

	return filePath, nil
}

// SaveReportText salva il report in formato testo (per PDF useremo JS)
func (a *App) SaveReportText(projectID int) (string, error) {
	// Genera il report
	report, err := tracker.GeneraReportChiusura(a.db, projectID)
	if err != nil {
		return "", err
	}

	projectName := report["project_name"].(string)
	projectDesc := ""
	if desc, ok := report["project_description"].(string); ok {
		projectDesc = desc
	}
	totalHours := 0.0
	if h, ok := report["total_hours"].(float64); ok {
		totalHours = h
	}
	startDate := ""
	if s, ok := report["start_date"].(string); ok {
		startDate = s
	}
	endDate := ""
	if e, ok := report["end_date"].(string); ok {
		endDate = e
	}
	closedAt := ""
	if c, ok := report["closed_at"].(string); ok {
		closedAt = c
	}

	// Costruisci il testo del report
	text := fmt.Sprintf("REPORT PROGETTO: %s\n", projectName)
	text += fmt.Sprintf("================================\n\n")
	if projectDesc != "" {
		text += fmt.Sprintf("Descrizione: %s\n\n", projectDesc)
	}
	text += fmt.Sprintf("Periodo: %s - %s\n", startDate, endDate)
	text += fmt.Sprintf("Chiuso il: %s\n\n", closedAt)
	text += fmt.Sprintf("TOTALE ORE TRACCIATE: %.2f ore\n\n", totalHours)

	// Suddivisione per tipo attività
	if breakdown, ok := report["activity_breakdown"].(map[string]float64); ok && len(breakdown) > 0 {
		text += "SUDDIVISIONE PER TIPO ATTIVITÀ:\n"
		text += "--------------------------------\n"
		for activity, hours := range breakdown {
			text += fmt.Sprintf("  %s: %.2f ore\n", activity, hours)
		}
	}

	text += fmt.Sprintf("\n\nGenerato da PrendiTempo il %s\n", time.Now().Format("02/01/2006 15:04"))

	// Chiedi all'utente dove salvare
	defaultName := fmt.Sprintf("Report_%s_%s.txt", safeFilenamePart(projectName), time.Now().Format("2006-01-02"))

	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultName,
		Title:           "Salva Report",
		Filters: []runtime.FileFilter{
			{DisplayName: "Text Files (*.txt)", Pattern: "*.txt"},
		},
	})
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil // Utente ha annullato
	}

	// Salva il file
	err = os.WriteFile(filePath, []byte(text), 0600)
	if err != nil {
		return "", err
	}

	return filePath, nil
}

// ImportProjectJSON importa un progetto da file JSON
type projectReportImport struct {
	ExportType         string                    `json:"export_type"`
	Version            string                    `json:"version"`
	ProjectName        string                    `json:"project_name"`
	ProjectDescription string                    `json:"project_description"`
	NoteText           string                    `json:"note_text"`
	Sessions           []projectSessionImport    `json:"sessions"`
	Notes              []projectLegacyNoteImport `json:"notes"`
}

type projectSessionImport struct {
	AppName      string  `json:"app_name"`
	Seconds      int     `json:"seconds"`
	SessionType  string  `json:"session_type"`
	ActivityType *string `json:"activity_type"`
	Timestamp    string  `json:"timestamp"`
}

type projectLegacyNoteImport struct {
	NoteText  string `json:"note_text"`
	Timestamp string `json:"timestamp"`
}

func (a *App) ImportProjectJSON() (string, error) {
	// Chiedi all'utente di selezionare il file
	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Seleziona Report JSON da Importare",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil // Utente ha annullato
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("errore lettura file: %v", err)
	}
	if info.Size() > 10*1024*1024 {
		return "", fmt.Errorf("il report supera il limite di 10 MiB")
	}

	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("errore lettura file: %v", err)
	}

	var reportData projectReportImport
	if err := json.Unmarshal(jsonData, &reportData); err != nil {
		return "", fmt.Errorf("errore parsing JSON: %v", err)
	}

	// Verifica che sia un report valido
	if reportData.ExportType != "project_report" {
		return "", fmt.Errorf("file non valido: non è un report di progetto PrendiTempo")
	}
	if reportData.Version != "" && reportData.Version != "1.0" && reportData.Version != "2.0" {
		return "", fmt.Errorf("versione report non supportata: %s", reportData.Version)
	}

	projectName := strings.TrimSpace(reportData.ProjectName)
	if projectName == "" || len([]rune(projectName)) > 100 {
		return "", fmt.Errorf("nome progetto mancante o troppo lungo nel file")
	}
	if len([]rune(reportData.ProjectDescription)) > 500 || len(reportData.NoteText) > 1024*1024 {
		return "", fmt.Errorf("descrizione o nota del progetto troppo lunga")
	}
	for index, session := range reportData.Sessions {
		if strings.TrimSpace(session.AppName) == "" || len([]rune(session.AppName)) > 200 || session.Seconds <= 0 || session.Seconds > 366*24*60*60 {
			return "", fmt.Errorf("sessione %d non valida", index+1)
		}
		if session.SessionType != "computer" && session.SessionType != "off-computer" && session.SessionType != "manual" {
			return "", fmt.Errorf("sessione %d: tipo non valido", index+1)
		}
		if _, err := parseLocalTimestamp(session.Timestamp); err != nil {
			return "", fmt.Errorf("sessione %d: %w", index+1, err)
		}
	}
	for index, note := range reportData.Notes {
		if len(note.NoteText) > 1024*1024 {
			return "", fmt.Errorf("nota legacy %d troppo lunga", index+1)
		}
		if _, err := parseLocalTimestamp(note.Timestamp); err != nil {
			return "", fmt.Errorf("nota legacy %d: %w", index+1, err)
		}
	}

	// Verifica se il progetto esiste già
	existingProject, _ := tracker.TrovaProgetto(a.db, projectName)
	if existingProject != nil {
		// Aggiungi suffisso per evitare conflitti
		suffix := fmt.Sprintf(" (Importato %s)", time.Now().Format("02-01-2006 15:04"))
		nameRunes := []rune(projectName)
		if len(nameRunes)+len([]rune(suffix)) > 100 {
			nameRunes = nameRunes[:100-len([]rune(suffix))]
		}
		projectName = string(nameRunes) + suffix
	}

	// Crea il nuovo progetto
	tx, err := a.db.Begin()
	if err != nil {
		return "", err
	}

	defer tx.Rollback()
	result, err := tx.Exec(
		"INSERT INTO projects (name, description, note_text, archived, closed_at) VALUES (?, ?, ?, 0, NULL)",
		projectName, reportData.ProjectDescription, reportData.NoteText,
	)
	if err != nil {
		return "", fmt.Errorf("errore creazione progetto: %v", err)
	}

	newProjectID, err := result.LastInsertId()
	if err != nil {
		return "", err
	}

	sessionsImported := 0
	for index, session := range reportData.Sessions {
		if _, err := tx.Exec(
			"INSERT INTO sessions (app_name, seconds, project_id, session_type, activity_type, timestamp) VALUES (?, ?, ?, ?, ?, ?)",
			session.AppName, session.Seconds, newProjectID, session.SessionType, session.ActivityType, session.Timestamp,
		); err != nil {
			return "", fmt.Errorf("errore importazione sessione %d: %v", index+1, err)
		}
		sessionsImported++
	}

	notesImported := 0
	for index, note := range reportData.Notes {
		if _, err := tx.Exec(
			"INSERT INTO notes (project_id, note_text, timestamp) VALUES (?, ?, ?)",
			newProjectID, note.NoteText, note.Timestamp,
		); err != nil {
			return "", fmt.Errorf("errore importazione nota legacy %d: %v", index+1, err)
		}
		notesImported++
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return fmt.Sprintf("Progetto '%s' importato con successo!\n%d sessioni e %d note ripristinate.", projectName, sessionsImported, notesImported), nil
}
