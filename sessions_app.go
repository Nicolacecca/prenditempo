package main

import (
	"fmt"
	"strconv"
	"strings"

	"work-time-tracker-go/tracker"
)

// SessionData rappresenta una sessione
type SessionData struct {
	ID           int     `json:"id"`
	AppName      string  `json:"app_name"`
	Seconds      int     `json:"seconds"`
	ProjectID    *int    `json:"project_id,omitempty"`
	ProjectName  string  `json:"project_name"`
	SessionType  string  `json:"session_type"`
	ActivityType *string `json:"activity_type,omitempty"`
	Timestamp    string  `json:"timestamp"`
}

// GetSessions restituisce le sessioni in un periodo
func (a *App) GetSessions(startDate, endDate string) ([]SessionData, error) {
	sessions, err := tracker.CaricaSessioniDettagliate(a.db, startDate, endDate)
	if err != nil {
		return nil, err
	}

	var result []SessionData
	for _, s := range sessions {
		result = append(result, SessionData{
			ID:           s.ID,
			AppName:      s.AppName,
			Seconds:      s.Seconds,
			ProjectID:    s.ProjectID,
			ProjectName:  s.ProjectName,
			SessionType:  s.SessionType,
			ActivityType: s.ActivityType,
			Timestamp:    s.Timestamp,
		})
	}
	return result, nil
}

// CreateSession crea una nuova sessione
func (a *App) CreateSession(appName string, seconds int, projectID *int, sessionType string, activityType *string, timestamp string) error {
	appName = strings.TrimSpace(appName)
	if appName == "" || len([]rune(appName)) > 200 {
		return fmt.Errorf("nome sessione non valido")
	}
	if seconds <= 0 || seconds > 366*24*60*60 {
		return fmt.Errorf("durata sessione non valida")
	}
	if sessionType != "computer" && sessionType != "off-computer" && sessionType != "manual" {
		return fmt.Errorf("tipo sessione non valido")
	}
	if err := validateNotInFuture(timestamp, seconds); err != nil {
		return err
	}
	if err := a.validateNewActivityReference(activityType); err != nil {
		return err
	}
	if projectID != nil {
		project, err := tracker.TrovaProgettoById(a.db, *projectID)
		if err != nil {
			return err
		}
		if project.Archived {
			return fmt.Errorf("non è possibile aggiungere una sessione a un progetto archiviato")
		}
	}
	return tracker.CreaSessione(a.db, appName, seconds, projectID, sessionType, activityType, timestamp)
}

// UpdateSessionDuration aggiorna la durata di una sessione
func (a *App) UpdateSessionDuration(sessionID, seconds int) error {
	if sessionID <= 0 || seconds <= 0 || seconds > 366*24*60*60 {
		return fmt.Errorf("sessione o durata non valida")
	}
	session, err := tracker.CaricaSessioneById(a.db, sessionID)
	if err != nil {
		return err
	}
	if err := validateNotInFuture(session.Timestamp, seconds); err != nil {
		return err
	}
	return tracker.AggiornaDurataSessione(a.db, sessionID, seconds)
}

// UpdateSessionActivityType aggiorna il tipo di attività di una sessione
func (a *App) UpdateSessionActivityType(sessionID int, activityType *string) error {
	if sessionID <= 0 {
		return fmt.Errorf("ID sessione non valido")
	}
	if err := a.validateActivityReference(activityType); err != nil {
		return err
	}
	return tracker.AggiornaActivityType(a.db, sessionID, activityType)
}

// DeleteSession elimina una sessione
func (a *App) DeleteSession(sessionID int) error {
	if sessionID <= 0 {
		return fmt.Errorf("ID sessione non valido")
	}
	return tracker.EliminaSessione(a.db, sessionID)
}

// SplitSession divide una sessione in due parti
func (a *App) SplitSession(sessionID, firstPartSeconds int, firstActivityType, secondActivityType *string) error {
	if sessionID <= 0 || firstPartSeconds <= 0 {
		return fmt.Errorf("dati divisione non validi")
	}
	if err := a.validateActivityReference(firstActivityType); err != nil {
		return err
	}
	if err := a.validateActivityReference(secondActivityType); err != nil {
		return err
	}
	return tracker.DividiSessione(a.db, sessionID, firstPartSeconds, firstActivityType, secondActivityType)
}

// UpdateSessionComplete aggiorna timestamp, durata e tipo attività di una sessione
func (a *App) UpdateSessionComplete(sessionID int, newTimestamp string, newSeconds int, activityType *string) error {
	if sessionID <= 0 || newSeconds <= 0 || newSeconds > 366*24*60*60 {
		return fmt.Errorf("sessione o durata non valida")
	}
	if err := validateNotInFuture(newTimestamp, newSeconds); err != nil {
		return err
	}
	if err := a.validateActivityReference(activityType); err != nil {
		return err
	}
	return tracker.AggiornaSessioneCompleta(a.db, sessionID, newTimestamp, newSeconds, activityType)
}

// GetSessionById restituisce una sessione dato l'ID
func (a *App) GetSessionById(sessionID int) (*SessionData, error) {
	session, err := tracker.CaricaSessioneById(a.db, sessionID)
	if err != nil {
		return nil, err
	}

	return &SessionData{
		ID:           session.ID,
		AppName:      session.AppName,
		Seconds:      session.Seconds,
		ProjectID:    session.ProjectID,
		ProjectName:  session.ProjectName,
		SessionType:  session.SessionType,
		ActivityType: session.ActivityType,
		Timestamp:    session.Timestamp,
	}, nil
}

// GetMergeGapSeconds restituisce l'intervallo massimo (secondi) entro cui due sessioni
// dello stesso progetto e tipo vengono unite automaticamente
func (a *App) GetMergeGapSeconds() int {
	return tracker.MergeGapSeconds(a.db)
}

// SetMergeGapSeconds imposta l'intervallo di unione automatica (0 = solo sessioni contigue)
func (a *App) SetMergeGapSeconds(seconds int) error {
	if seconds < 0 || seconds > tracker.MaxMergeGapSeconds {
		return fmt.Errorf("l'intervallo deve essere compreso tra 0 e %d secondi", tracker.MaxMergeGapSeconds)
	}
	return tracker.SetSetting(a.db, tracker.MergeGapSettingKey, strconv.Itoa(seconds))
}
