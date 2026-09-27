package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func validateBackupData(data ExportDataResult) error {
	if data.Version != "" && data.Version != "1.0" && data.Version != "2.0" && data.Version != "2.1" && data.Version != "2.2" && data.Version != "2.3" {
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

	assetIDs := make(map[string]struct{}, len(data.BoardAssets))
	const maxTotalBoardAssetsBytes = 100 * 1024 * 1024
	maxEncodedImageBytes := base64.StdEncoding.EncodedLen(maxBoardImageBytes)
	totalAssetBytes := 0
	sha256Pattern := regexp.MustCompile(`^[a-f0-9]{64}$`)
	for index, asset := range data.BoardAssets {
		if !boardAssetIDPattern.MatchString(asset.ID) {
			return fmt.Errorf("immagine %d: identificativo non valido", index+1)
		}
		if _, exists := assetIDs[asset.ID]; exists {
			return fmt.Errorf("identificativo immagine duplicato: %s", asset.ID)
		}
		assetIDs[asset.ID] = struct{}{}
		if _, exists := projectIDs[asset.ProjectID]; !exists {
			return fmt.Errorf("immagine %d: progetto %d inesistente", index+1, asset.ProjectID)
		}
		if asset.FileName != filepath.Base(asset.FileName) || strings.TrimSpace(asset.FileName) == "" || len(asset.FileName) > 255 {
			return fmt.Errorf("immagine %d: nome file non valido", index+1)
		}
		if asset.Size <= 0 || asset.Size > maxBoardImageBytes || len(asset.DataBase64) > maxEncodedImageBytes {
			return fmt.Errorf("immagine %d: dimensione non valida", index+1)
		}
		binaryData, err := base64.StdEncoding.DecodeString(asset.DataBase64)
		if err != nil || int64(len(binaryData)) != asset.Size {
			return fmt.Errorf("immagine %d: dati non validi", index+1)
		}
		mimeType, err := validateBoardImage(binaryData, asset.FileName)
		if err != nil || mimeType != asset.MimeType {
			return fmt.Errorf("immagine %d: formato non valido", index+1)
		}
		if !sha256Pattern.MatchString(asset.SHA256) {
			return fmt.Errorf("immagine %d: checksum non valido", index+1)
		}
		hash := sha256.Sum256(binaryData)
		if hex.EncodeToString(hash[:]) != asset.SHA256 {
			return fmt.Errorf("immagine %d: checksum non corrispondente", index+1)
		}
		totalAssetBytes += len(binaryData)
		if totalAssetBytes > maxTotalBoardAssetsBytes {
			return fmt.Errorf("le immagini della lavagna superano il limite complessivo di 100 MiB")
		}
		if asset.CreatedAt != "" {
			if _, err := parseLocalTimestamp(asset.CreatedAt); err != nil {
				return fmt.Errorf("immagine %d: data creazione non valida", index+1)
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

	boardProjects := make(map[int]struct{}, len(data.ProjectBoards))
	for index, board := range data.ProjectBoards {
		if _, exists := projectIDs[board.ProjectID]; !exists {
			return fmt.Errorf("lavagna %d: progetto %d inesistente", index+1, board.ProjectID)
		}
		if _, exists := boardProjects[board.ProjectID]; exists {
			return fmt.Errorf("lavagna duplicata per il progetto %d", board.ProjectID)
		}
		boardProjects[board.ProjectID] = struct{}{}
		if board.SchemaVersion != currentBoardSchemaVersion {
			return fmt.Errorf("lavagna %d: versione schema non supportata", index+1)
		}
		if len(board.DocumentJSON) > maxBoardDocumentBytes || !json.Valid([]byte(board.DocumentJSON)) {
			return fmt.Errorf("lavagna %d: documento non valido", index+1)
		}
		if len(board.ViewportJSON) > maxBoardViewportBytes || !json.Valid([]byte(board.ViewportJSON)) {
			return fmt.Errorf("lavagna %d: visuale non valida", index+1)
		}
		if board.UpdatedAt != "" {
			if _, err := parseLocalTimestamp(board.UpdatedAt); err != nil {
				return fmt.Errorf("lavagna %d: data aggiornamento non valida", index+1)
			}
		}
	}

	versionIDs := make(map[int64]struct{}, len(data.BoardVersions))
	versionsPerProject := make(map[int]int)
	totalVersionBytes := 0
	for index, version := range data.BoardVersions {
		if version.ID <= 0 {
			return fmt.Errorf("versione lavagna %d: ID non valido", index+1)
		}
		if _, exists := versionIDs[version.ID]; exists {
			return fmt.Errorf("versione lavagna duplicata: %d", version.ID)
		}
		versionIDs[version.ID] = struct{}{}
		if _, exists := projectIDs[version.ProjectID]; !exists {
			return fmt.Errorf("versione lavagna %d: progetto inesistente", index+1)
		}
		if strings.TrimSpace(version.Name) == "" || len([]rune(version.Name)) > 80 {
			return fmt.Errorf("versione lavagna %d: nome non valido", index+1)
		}
		if len(version.DocumentJSON) > maxBoardDocumentBytes || !json.Valid([]byte(version.DocumentJSON)) {
			return fmt.Errorf("versione lavagna %d: documento non valido", index+1)
		}
		if len(version.ViewportJSON) > maxBoardViewportBytes || !json.Valid([]byte(version.ViewportJSON)) {
			return fmt.Errorf("versione lavagna %d: visuale non valida", index+1)
		}
		if version.CreatedAt != "" {
			if _, err := parseLocalTimestamp(version.CreatedAt); err != nil {
				return fmt.Errorf("versione lavagna %d: data non valida", index+1)
			}
		}
		versionsPerProject[version.ProjectID]++
		if versionsPerProject[version.ProjectID] > 100 {
			return fmt.Errorf("troppe versioni per il progetto %d", version.ProjectID)
		}
		totalVersionBytes += len(version.DocumentJSON) + len(version.ViewportJSON)
		if totalVersionBytes > 200*1024*1024 {
			return fmt.Errorf("la cronologia delle lavagne supera 200 MiB")
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
		if event.EventType != "holiday" && event.EventType != "vacation" && event.EventType != "permesso" {
			return fmt.Errorf("evento %d: tipo non valido", index+1)
		}
		if event.Hours < 0 || event.Hours > 24 || (event.EventType == "permesso" && event.Hours == 0) {
			return fmt.Errorf("evento %d: ore non valide", index+1)
		}
		if event.EventType != "permesso" && event.Hours != 0 {
			return fmt.Errorf("evento %d: feste e ferie devono avere zero ore", index+1)
		}
		if _, exists := eventDates[event.Date]; exists {
			return fmt.Errorf("evento duplicato per %s", event.Date)
		}
		eventDates[event.Date] = struct{}{}
	}

	return nil
}
