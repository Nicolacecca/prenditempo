package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var supportedBoardVideoExtensions = map[string]bool{
	".mp4":  true,
	".m4v":  true,
	".mov":  true,
	".avi":  true,
	".mkv":  true,
	".webm": true,
	".wmv":  true,
	".mpeg": true,
	".mpg":  true,
}

type BoardVideoLink struct {
	Path     string `json:"path"`
	FileName string `json:"file_name"`
	Size     int64  `json:"size"`
}

func normalizeBoardVideoPath(raw string, checkExists bool) (*BoardVideoLink, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxBoardLinkLength || strings.ContainsRune(raw, '\x00') {
		return nil, fmt.Errorf("percorso video non valido")
	}
	if strings.HasPrefix(raw, `\\?\`) || strings.HasPrefix(raw, `\\.\`) || !filepath.IsAbs(raw) {
		return nil, fmt.Errorf("il percorso video deve essere assoluto")
	}
	path := filepath.Clean(raw)
	extension := strings.ToLower(filepath.Ext(path))
	if !supportedBoardVideoExtensions[extension] {
		return nil, fmt.Errorf("formato video non supportato")
	}
	result := &BoardVideoLink{Path: path, FileName: filepath.Base(path)}
	if checkExists {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("video non raggiungibile: %w", err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("il collegamento non punta a un file video")
		}
		result.Size = info.Size()
	}
	return result, nil
}

func (a *App) SelectBoardVideo() (*BoardVideoLink, error) {
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Seleziona un video da collegare",
		Filters: []wailsruntime.FileFilter{{
			DisplayName: "Video (*.mp4;*.m4v;*.mov;*.avi;*.mkv;*.webm;*.wmv;*.mpeg;*.mpg)",
			Pattern:     "*.mp4;*.m4v;*.mov;*.avi;*.mkv;*.webm;*.wmv;*.mpeg;*.mpg",
		}},
	})
	if err != nil || path == "" {
		return nil, err
	}
	return normalizeBoardVideoPath(path, true)
}

func (a *App) OpenBoardVideo(path string) error {
	video, err := normalizeBoardVideoPath(path, true)
	if err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		return fmt.Errorf("apertura video supportata solo su Windows")
	}
	if err := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", video.Path).Start(); err != nil {
		return fmt.Errorf("impossibile aprire il video: %w", err)
	}
	return nil
}
