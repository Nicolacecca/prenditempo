package main

import (
	"fmt"
	"strings"
	"time"
)

func validateBackupData(data ExportDataResult) error {
	if data.Version != "" && data.Version != "1.0" && data.Version != "2.0" && data.Version != "2.1" && data.Version != "2.2" && data.Version != "2.3" && data.Version != "2.4" {
		return fmt.Errorf("versione backup non supportata: %s", data.Version)
	}

	projectIDs := make(map[int]struct{}, len(data.Projects))
	projectNames := make(map[string]struct{}, len(data.Projects))
	for index, project := range data.Projects {
		if project.ID <= 0 {
			return fmt.Errorf("progetto %d: ID non valido", index+1)
		}
		if _, exists := projectIDs[project.ID]; exists {
			return fmt.Errorf("ID progetto duplicato: %d", project.ID)
		}
		projectIDs[project.ID] = struct{}{}

		name := strings.TrimSpace(project.Name)
		if name == "" || len(name) > 200 {
			return fmt.Errorf("progetto %d: nome vuoto o troppo lungo", index+1)
		}
		nameKey := strings.ToLower(name)
		if _, exists := projectNames[nameKey]; exists {
			return fmt.Errorf("nome progetto duplicato: %s", name)
		}
		projectNames[nameKey] = struct{}{}
		if len([]rune(name)) > 100 || len([]rune(project.Description)) > 500 || len(project.NoteText) > 1024*1024 {
			return fmt.Errorf("progetto %q: descrizione o nota troppo lunga", name)
		}
		if project.CreatedAt != "" {
			if _, err := parseLocalTimestamp(project.CreatedAt); err != nil {
				return fmt.Errorf("progetto %q: data creazione non valida", name)
			}
		}
	}

	for index, session := range data.Sessions {
		if strings.TrimSpace(session.AppName) == "" || len(session.AppName) > 500 {
			return fmt.Errorf("sessione %d: nome applicazione non valido", index+1)
		}
		if session.Seconds <= 0 || session.Seconds > 366*24*60*60 {
			return fmt.Errorf("sessione %d: durata non valida", index+1)
		}
		if _, err := parseLocalTimestamp(session.Timestamp); err != nil {
			return fmt.Errorf("sessione %d: %w", index+1, err)
		}
		if session.SessionType != "computer" && session.SessionType != "off-computer" && session.SessionType != "manual" {
			return fmt.Errorf("sessione %d: tipo non valido", index+1)
		}
		if session.ProjectID != nil {
			if _, exists := projectIDs[*session.ProjectID]; !exists {
				return fmt.Errorf("sessione %d: progetto %d inesistente", index+1, *session.ProjectID)
			}
		}
	}

	for index, note := range data.Notes {
		if _, exists := projectIDs[note.ProjectID]; !exists {
			return fmt.Errorf("nota %d: progetto %d inesistente", index+1, note.ProjectID)
		}
		if len(note.NoteText) > 1024*1024 {
			return fmt.Errorf("nota %d: contenuto troppo lungo o timestamp mancante", index+1)
		}
		if _, err := parseLocalTimestamp(note.Timestamp); err != nil {
			return fmt.Errorf("nota %d: %w", index+1, err)
		}
	}

	activityNames := make(map[string]struct{}, len(data.ActivityTypes))
	for index, activityType := range data.ActivityTypes {
		name := strings.TrimSpace(activityType.Name)
		if name == "" || len(name) > 200 {
			return fmt.Errorf("tipo attività %d: nome non valido", index+1)
		}
		key := strings.ToLower(name)
		if _, exists := activityNames[key]; exists {
			return fmt.Errorf("tipo attività duplicato: %s", name)
		}
		activityNames[key] = struct{}{}
		if activityType.ColorVariant < -1 || activityType.ColorVariant > 1 || activityType.DisplayOrder < 0 {
			return fmt.Errorf("tipo attività %q: stile non valido", name)
		}
		if activityType.Pattern != "" && activityType.Pattern != "solid" && activityType.Pattern != "stripes" && activityType.Pattern != "dots" {
			return fmt.Errorf("tipo attività %q: motivo non valido", name)
		}
	}

	settingKeys := make(map[string]struct{}, len(data.Settings))
	for index, setting := range data.Settings {
		if strings.TrimSpace(setting.Key) == "" || len(setting.Key) > 100 || len(setting.Value) > 10000 {
			return fmt.Errorf("impostazione %d non valida", index+1)
		}
		if _, exists := settingKeys[setting.Key]; exists {
			return fmt.Errorf("impostazione duplicata: %s", setting.Key)
		}
		settingKeys[setting.Key] = struct{}{}
	}

	eventDates := make(map[string]struct{}, len(data.DayEvents))
	for index, event := range data.DayEvents {
		if _, err := time.Parse("2006-01-02", event.Date); err != nil {
			return fmt.Errorf("evento %d: data non valida", index+1)
		}
		if event.EventType != "holiday" && event.EventType != "vacation" && event.EventType != "permesso" && event.EventType != "malattia" && event.EventType != "trasferta" {
			return fmt.Errorf("evento %d: tipo non valido", index+1)
		}
		if event.Hours < 0 || event.Hours > 24 || (event.EventType == "permesso" && event.Hours == 0) {
			return fmt.Errorf("evento %d: ore non valide", index+1)
		}
		if event.EventType != "permesso" && event.Hours != 0 {
			return fmt.Errorf("evento %d: feste, ferie, malattia e trasferte devono avere zero ore", index+1)
		}
		if _, exists := eventDates[event.Date]; exists {
			return fmt.Errorf("evento duplicato per %s", event.Date)
		}
		eventDates[event.Date] = struct{}{}
	}

	return nil
}
