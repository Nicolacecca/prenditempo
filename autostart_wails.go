//go:build wails

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/windows/registry"
)

const registryPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const appName = "PrendiTempo"

// getResolvedExePath restituisce il percorso assoluto e risolto dell'eseguibile
func getResolvedExePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	// Risolvi eventuali symlink
	resolvedPath, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		// Se non riesce a risolvere i symlink, usa il percorso originale
		resolvedPath = exePath
	}

	// Converti in percorso assoluto
	absPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return resolvedPath, nil
	}

	return absPath, nil
}

// EnableAutoStart aggiunge l'applicazione all'avvio automatico di Windows
func (a *App) EnableAutoStart() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("auto-start supportato solo su Windows")
	}

	// Ottieni il percorso dell'eseguibile corrente (risolto)
	exePath, err := getResolvedExePath()
	if err != nil {
		return fmt.Errorf("impossibile ottenere percorso eseguibile: %v", err)
	}

	log.Printf("[AUTOSTART] Tentativo di abilitare autostart per: %s", exePath)

	// Apri la chiave del registro
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE)
	if err != nil {
		log.Printf("[AUTOSTART] Errore apertura registro: %v", err)
		return fmt.Errorf("impossibile aprire chiave registro: %v", err)
	}
	defer key.Close()

	// Imposta il valore per eseguire l'app all'avvio
	registryValue := fmt.Sprintf("\"%s\"", exePath)
	err = key.SetStringValue(appName, registryValue)
	if err != nil {
		log.Printf("[AUTOSTART] Errore impostazione valore: %v", err)
		return fmt.Errorf("impossibile impostare valore registro: %v", err)
	}

	log.Printf("[AUTOSTART] Avvio automatico abilitato con successo!")
	log.Printf("[AUTOSTART] Chiave: HKCU\\%s\\%s", registryPath, appName)
	log.Printf("[AUTOSTART] Valore: %s", registryValue)
	return nil
}

// DisableAutoStart rimuove l'applicazione dall'avvio automatico di Windows
func (a *App) DisableAutoStart() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("auto-start supportato solo su Windows")
	}

	// Apri la chiave del registro
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("impossibile aprire chiave registro: %v", err)
	}
	defer key.Close()

	// Elimina il valore
	err = key.DeleteValue(appName)
	if err != nil {
		// Se il valore non esiste, non è un errore
		if err == registry.ErrNotExist {
			log.Println("[AUTOSTART] Avvio automatico già disabilitato")
			return nil
		}
		return fmt.Errorf("impossibile eliminare valore registro: %v", err)
	}

	log.Println("[AUTOSTART] Avvio automatico disabilitato")
	return nil
}

// IsAutoStartEnabled verifica se l'avvio automatico è abilitato
func (a *App) IsAutoStartEnabled() bool {
	if runtime.GOOS != "windows" {
		return false
	}

	// Apri la chiave del registro
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.QUERY_VALUE)
	if err != nil {
		log.Printf("[AUTOSTART] Errore apertura registro per verifica: %v", err)
		return false
	}
	defer key.Close()

	// Leggi il valore
	value, _, err := key.GetStringValue(appName)
	if err != nil {
		log.Printf("[AUTOSTART] Chiave '%s' non trovata: %v", appName, err)
		return false
	}

	// Verifica che il percorso registrato corrisponda all'eseguibile corrente
	exePath, _ := os.Executable()
	expectedValue := fmt.Sprintf("\"%s\"", exePath)

	log.Printf("[AUTOSTART] Valore nel registro: %s", value)
	log.Printf("[AUTOSTART] Percorso atteso: %s", expectedValue)

	return true
}

// GetAutoStartInfo restituisce informazioni di debug sull'autostart
func (a *App) GetAutoStartInfo() map[string]string {
	info := make(map[string]string)

	exePath, err := os.Executable()
	if err != nil {
		info["exe_path"] = "Errore: " + err.Error()
	} else {
		info["exe_path"] = exePath
	}

	if runtime.GOOS != "windows" {
		info["status"] = "Non supportato su questo sistema"
		return info
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.QUERY_VALUE)
	if err != nil {
		info["registry_status"] = "Errore apertura registro: " + err.Error()
		return info
	}
	defer key.Close()

	value, _, err := key.GetStringValue(appName)
	if err != nil {
		info["registry_value"] = "Non impostato"
	} else {
		info["registry_value"] = value
	}

	return info
}
