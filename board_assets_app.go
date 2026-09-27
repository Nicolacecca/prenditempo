package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxBoardImageBytes = 10 * 1024 * 1024

var boardAssetIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type BoardAssetData struct {
	ID        string `json:"id"`
	ProjectID int    `json:"project_id"`
	FileName  string `json:"file_name"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	DataURL   string `json:"data_url"`
}

func newBoardAssetID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func validateBoardImage(data []byte, fileName string) (string, error) {
	if len(data) == 0 || len(data) > maxBoardImageBytes {
		return "", fmt.Errorf("l'immagine è vuota o supera 10 MiB")
	}
	mimeType := http.DetectContentType(data)
	allowed := map[string]bool{
		"image/png":  true,
		"image/jpeg": true,
		"image/gif":  true,
		"image/webp": true,
	}
	if !allowed[mimeType] {
		return "", fmt.Errorf("formato immagine non supportato")
	}
	if extension := strings.ToLower(filepath.Ext(fileName)); extension != "" {
		if expected := strings.Split(mime.TypeByExtension(extension), ";")[0]; expected != "" && expected != mimeType {
			return "", fmt.Errorf("estensione e contenuto dell'immagine non corrispondono")
		}
	}
	return mimeType, nil
}

func assetDataURL(mimeType string, data []byte) string {
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func (a *App) ImportBoardImage(projectID int) (*BoardAssetData, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("ID progetto non valido")
	}
	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Seleziona un'immagine per la lavagna",
		Filters: []runtime.FileFilter{{
			DisplayName: "Immagini (*.png;*.jpg;*.jpeg;*.gif;*.webp)",
			Pattern:     "*.png;*.jpg;*.jpeg;*.gif;*.webp",
		}},
	})
	if err != nil || filePath == "" {
		return nil, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("impossibile leggere l'immagine: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() <= 0 || info.Size() > maxBoardImageBytes {
		return nil, fmt.Errorf("l'immagine è vuota o supera 10 MiB")
	}
	data := make([]byte, info.Size())
	if _, err := io.ReadFull(file, data); err != nil {
		return nil, fmt.Errorf("impossibile leggere l'immagine: %w", err)
	}
	mimeType, err := validateBoardImage(data, filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	hashBytes := sha256.Sum256(data)
	hash := hex.EncodeToString(hashBytes[:])

	asset := &BoardAssetData{ProjectID: projectID}
	var storedData []byte
	err = a.db.QueryRow(`
		SELECT id, file_name, mime_type, size, data
		FROM board_assets WHERE project_id = ? AND sha256 = ?`, projectID, hash,
	).Scan(&asset.ID, &asset.FileName, &asset.MimeType, &asset.Size, &storedData)
	if err == nil {
		asset.DataURL = assetDataURL(asset.MimeType, storedData)
		return asset, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("errore ricerca immagine: %w", err)
	}

	asset.ID, err = newBoardAssetID()
	if err != nil {
		return nil, fmt.Errorf("impossibile creare l'identificativo immagine: %w", err)
	}
	asset.FileName = filepath.Base(filePath)
	asset.MimeType = mimeType
	asset.Size = int64(len(data))
	if _, err := a.db.Exec(`
		INSERT INTO board_assets (id, project_id, file_name, mime_type, data, size, sha256)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		asset.ID, projectID, asset.FileName, mimeType, data, asset.Size, hash,
	); err != nil {
		return nil, fmt.Errorf("errore salvataggio immagine: %w", err)
	}
	asset.DataURL = assetDataURL(mimeType, data)
	return asset, nil
}

func (a *App) GetBoardAsset(projectID int, assetID string) (*BoardAssetData, error) {
	if projectID <= 0 || !boardAssetIDPattern.MatchString(assetID) {
		return nil, fmt.Errorf("riferimento immagine non valido")
	}
	asset := &BoardAssetData{ID: assetID, ProjectID: projectID}
	var data []byte
	if err := a.db.QueryRow(`
		SELECT file_name, mime_type, size, data
		FROM board_assets WHERE id = ? AND project_id = ?`, assetID, projectID,
	).Scan(&asset.FileName, &asset.MimeType, &asset.Size, &data); err != nil {
		return nil, fmt.Errorf("immagine non trovata")
	}
	asset.DataURL = assetDataURL(asset.MimeType, data)
	return asset, nil
}

func (a *App) CleanupBoardAssets(projectID int, retainedAssetIDs []string) error {
	if projectID <= 0 || len(retainedAssetIDs) > 10000 {
		return fmt.Errorf("riferimenti immagini non validi")
	}
	retained := make(map[string]struct{}, len(retainedAssetIDs))
	for _, assetID := range retainedAssetIDs {
		if !boardAssetIDPattern.MatchString(assetID) {
			return fmt.Errorf("riferimento immagine non valido")
		}
		retained[assetID] = struct{}{}
	}

	tx, err := a.db.Begin()
	if err != nil {
		return fmt.Errorf("errore avvio pulizia immagini: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT id FROM board_assets WHERE project_id = ?`, projectID)
	if err != nil {
		return fmt.Errorf("errore lettura immagini: %w", err)
	}
	var obsolete []string
	for rows.Next() {
		var assetID string
		if err := rows.Scan(&assetID); err != nil {
			rows.Close()
			return err
		}
		if _, keep := retained[assetID]; !keep {
			obsolete = append(obsolete, assetID)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, assetID := range obsolete {
		if _, err := tx.Exec(`DELETE FROM board_assets WHERE id = ? AND project_id = ?`, assetID, projectID); err != nil {
			return fmt.Errorf("errore eliminazione immagine inutilizzata: %w", err)
		}
	}
	return tx.Commit()
}
