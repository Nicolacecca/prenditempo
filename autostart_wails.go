//go:build wails

package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"golang.org/x/sys/windows/registry"
)

const registryPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const appName = "PrendiTempo"

// EnableAutoStart aggiunge l'applicazione all'avvio automatico di Windows
func (a *App) EnableAutoStart() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("auto-start supportato solo su Windows")
	}

	// Ottieni il percorso dell'eseguibile corrente
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("impossibile ottenere percorso eseguibile: %v", err)
	}

	// Apri la chiave del registro
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("impossibile aprire chiave registro: %v", err)
	}
	defer key.Close()

	// Imposta il valore per eseguire l'app all'avvio
	err = key.SetStringValue(appName, fmt.Sprintf("\"%s\"", exePath))
	if err != nil {
		return fmt.Errorf("impossibile impostare valore registro: %v", err)
	}

	log.Printf("[AUTOSTART] Avvio automatico abilitato: %s", exePath)
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
		return false
	}
	defer key.Close()

	// Leggi il valore
	_, _, err = key.GetStringValue(appName)
	if err != nil {
		return false
	}

	return true
}
