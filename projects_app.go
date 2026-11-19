package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"work-time-tracker-go/tracker"
)

// ProjectData rappresenta i dati di un progetto
type ProjectData struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	Archived    bool   `json:"archived"`
	ClosedAt    string `json:"closed_at,omitempty"`
	NoteText    string `json:"note_text"`
	BoardURL    string `json:"board_url"`
}

// GetProjects restituisce tutti i progetti attivi (ottimizzato con query filtrata)
func (a *App) GetProjects() ([]ProjectData, error) {
	projects, err := tracker.CaricaProgettiAttivi(a.db)
	if err != nil {
		return nil, err
	}

	var result []ProjectData
	for _, p := range projects {
		result = append(result, ProjectData{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			CreatedAt:   p.CreatedAt,
			Archived:    p.Archived,
			ClosedAt:    p.ClosedAt,
			NoteText:    p.NoteText,
			BoardURL:    p.BoardURL,
		})
	}
	return result, nil
}

// GetArchivedProjects restituisce i progetti archiviati (ottimizzato con query filtrata)
func (a *App) GetArchivedProjects() ([]ProjectData, error) {
	projects, err := tracker.CaricaProgettiArchiviati(a.db)
	if err != nil {
		return nil, err
	}

	var result []ProjectData
	for _, p := range projects {
		result = append(result, ProjectData{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			CreatedAt:   p.CreatedAt,
			Archived:    p.Archived,
			ClosedAt:    p.ClosedAt,
			NoteText:    p.NoteText,
			BoardURL:    p.BoardURL,
		})
	}
	return result, nil
}

// CreateProject crea un nuovo progetto
func (a *App) CreateProject(name, description string) (int64, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" || len([]rune(name)) > 100 {
		return 0, fmt.Errorf("il nome del progetto è obbligatorio e non può superare 100 caratteri")
	}
	if len([]rune(description)) > 500 {
		return 0, fmt.Errorf("la descrizione non può superare 500 caratteri")
	}
	return tracker.CreaProgetto(a.db, name, description)
}

// ArchiveProject archivia un progetto
func (a *App) ArchiveProject(projectID int) error {
	if projectID <= 0 {
		return fmt.Errorf("ID progetto non valido")
	}
	a.track.mu.Lock()
	defer a.track.mu.Unlock()
	if err := a.ensureProjectNotTrackedLocked(projectID); err != nil {
		return err
	}
	return tracker.ArchivaProgetto(a.db, projectID)
}

// ReactivateProject riattiva un progetto archiviato
func (a *App) ReactivateProject(projectID int) error {
	if projectID <= 0 {
		return fmt.Errorf("ID progetto non valido")
	}
	return tracker.RiattivaProgetto(a.db, projectID)
}

// DeleteProject elimina un progetto
func (a *App) DeleteProject(projectID int) error {
	if projectID <= 0 {
		return fmt.Errorf("ID progetto non valido")
	}
	a.track.mu.Lock()
	defer a.track.mu.Unlock()
	if err := a.ensureProjectNotTrackedLocked(projectID); err != nil {
		return err
	}
	return tracker.EliminaProgettoById(a.db, projectID)
}

func (a *App) ensureProjectNotTrackedLocked(projectID int) error {
	if a.track.isTracking && a.track.currentProject != nil && a.track.currentProject.ID == projectID {
		return fmt.Errorf("il progetto è in uso dal tracking corrente")
	}
	return nil
}

// UpdateProject aggiorna nome e descrizione di un progetto
func (a *App) UpdateProject(projectID int, name, description string) error {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if projectID <= 0 || name == "" || len([]rune(name)) > 100 {
		return fmt.Errorf("dati progetto non validi")
	}
	if len([]rune(description)) > 500 {
		return fmt.Errorf("la descrizione non può superare 500 caratteri")
	}
	return tracker.AggiornaProgetto(a.db, projectID, name, description)
}

// UpdateProjectNote aggiorna la nota markdown di un progetto
func (a *App) UpdateProjectNote(projectID int, noteText string) error {
	if projectID <= 0 || len([]byte(noteText)) > 1024*1024 {
		return fmt.Errorf("nota progetto non valida o superiore a 1 MiB")
	}
	return tracker.AggiornaNotaProgetto(a.db, projectID, noteText)
}

// MigrateLegacyNotes migra le note dalla vecchia tabella al nuovo sistema
func (a *App) MigrateLegacyNotes() (int, error) {
	return tracker.MigraNoteLegacy(a.db)
}

// SetProjectBoardURL collega un progetto alla lavagna di un provider esterno (es. Excalidraw)
func (a *App) SetProjectBoardURL(projectID int, boardURL string) error {
	if projectID <= 0 {
		return fmt.Errorf("ID progetto non valido")
	}
	boardURL = strings.TrimSpace(boardURL)
	if boardURL != "" {
		parsed, err := url.Parse(boardURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return fmt.Errorf("URL lavagna non valido: deve iniziare con http:// o https://")
		}
	}
	return tracker.AggiornaBoardURLProgetto(a.db, projectID, boardURL)
}

// OpenProjectBoardURL apre nel browser predefinito la lavagna collegata al progetto
func (a *App) OpenProjectBoardURL(boardURL string) error {
	boardURL = strings.TrimSpace(boardURL)
	parsed, err := url.Parse(boardURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("URL lavagna non valido")
	}
	runtime.BrowserOpenURL(a.ctx, boardURL)
	return nil
}

// GetProjectReport genera il report di un progetto
func (a *App) GetProjectReport(projectID int) (map[string]interface{}, error) {
	return tracker.GeneraReportChiusura(a.db, projectID)
}
