package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxBoardLinkLength = 4096

func normalizeBoardWebURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxBoardLinkLength || strings.ContainsRune(raw, '\x00') {
		return "", fmt.Errorf("URL non valido")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("sono consentiti solo link HTTP e HTTPS completi")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("i link con credenziali incorporate non sono consentiti")
	}
	return parsed.String(), nil
}

func normalizeBoardFolderPath(raw string, requireNetwork bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxBoardLinkLength || strings.ContainsRune(raw, '\x00') {
		return "", fmt.Errorf("percorso cartella non valido")
	}
	if strings.HasPrefix(raw, `\\?\`) || strings.HasPrefix(raw, `\\.\`) {
		return "", fmt.Errorf("i percorsi dispositivo non sono consentiti")
	}
	isNetwork := strings.HasPrefix(raw, `\\`)
	if requireNetwork && !isNetwork {
		return "", fmt.Errorf("inserisci un percorso di rete UNC, ad esempio \\\\server\\condivisione")
	}
	if isNetwork {
		parts := strings.Split(strings.TrimPrefix(raw, `\\`), `\`)
		if len(parts) < 1 || strings.TrimSpace(parts[0]) == "" {
			return "", fmt.Errorf("il percorso di rete deve includere il nome del server o NAS")
		}
	} else if !filepath.IsAbs(raw) {
		return "", fmt.Errorf("il percorso della cartella deve essere assoluto")
	}
	return filepath.Clean(raw), nil
}

func validateBoardLink(linkType, target string, checkExists bool) (string, error) {
	switch linkType {
	case "web":
		return normalizeBoardWebURL(target)
	case "folder", "network":
		path, err := normalizeBoardFolderPath(target, linkType == "network")
		if err != nil {
			return "", err
		}
		if checkExists {
			info, err := os.Stat(path)
			if err != nil {
				return "", fmt.Errorf("cartella non raggiungibile: %w", err)
			}
			if !info.IsDir() {
				return "", fmt.Errorf("il collegamento non punta a una cartella")
			}
		}
		return path, nil
	default:
		return "", fmt.Errorf("tipo di collegamento non valido")
	}
}

func (a *App) SelectBoardFolder() (string, error) {
	path, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Seleziona la cartella da collegare",
	})
	if err != nil || path == "" {
		return path, err
	}
	return validateBoardLink("folder", path, true)
}

func (a *App) ValidateBoardLink(linkType, target string) (string, error) {
	// I percorsi UNC possono richiedere l'autenticazione interattiva di Esplora file.
	// Ne verifichiamo la sintassi senza bloccarli con os.Stat prima che Windows li apra.
	return validateBoardLink(linkType, target, linkType == "folder")
}

func (a *App) OpenBoardLink(linkType, target string) error {
	normalized, err := validateBoardLink(linkType, target, false)
	if err != nil {
		return err
	}
	if linkType == "web" {
		if runtime.GOOS == "windows" {
			if err := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", normalized).Start(); err != nil {
				return fmt.Errorf("impossibile aprire il link Internet: %w", err)
			}
			return nil
		}
		wailsruntime.BrowserOpenURL(a.ctx, normalized)
		return nil
	}
	if runtime.GOOS != "windows" {
		return fmt.Errorf("apertura cartelle supportata solo su Windows")
	}
	if err := exec.Command("explorer.exe", normalized).Start(); err != nil {
		return fmt.Errorf("impossibile aprire la cartella: %w", err)
	}
	return nil
}
