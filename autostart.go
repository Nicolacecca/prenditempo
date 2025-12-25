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

// EnableAutoStart aggiunge lo script di notifica all'avvio automatico di Windows
func EnableAutoStart() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("auto-start supportato solo su Windows")
	}

	// Ottieni il percorso della directory dell'eseguibile
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("impossibile ottenere percorso eseguibile: %v", err)
	}

	// Ottieni la directory dell'eseguibile
	exeDir := filepath.Dir(exePath)

	// Percorso dello script VBS di notifica
	vbsPath := filepath.Join(exeDir, "startup_notification.vbs")

	// Verifica che lo script esista
	if _, err := os.Stat(vbsPath); os.IsNotExist(err) {
		return fmt.Errorf("script di notifica non trovato: %s", vbsPath)
	}

	// Apri la chiave del registro
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("impossibile aprire chiave registro: %v", err)
	}
	defer key.Close()

	// Imposta il valore per eseguire lo script VBS
	// wscript.exe esegue lo script in modo invisibile
	startupCmd := fmt.Sprintf("wscript.exe \"%s\"", vbsPath)
	err = key.SetStringValue(appName, startupCmd)
	if err != nil {
		return fmt.Errorf("impossibile impostare valore registro: %v", err)
	}

	log.Printf("[AUTOSTART] Notifica di avvio abilitata: %s", vbsPath)
	return nil
}

// DisableAutoStart rimuove l'applicazione dall'avvio automatico di Windows
func DisableAutoStart() error {
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
func IsAutoStartEnabled() (bool, error) {
	if runtime.GOOS != "windows" {
		return false, fmt.Errorf("auto-start supportato solo su Windows")
	}

	// Apri la chiave del registro
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.QUERY_VALUE)
	if err != nil {
		return false, fmt.Errorf("impossibile aprire chiave registro: %v", err)
	}
	defer key.Close()

	// Leggi il valore
	_, _, err = key.GetStringValue(appName)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, fmt.Errorf("impossibile leggere valore registro: %v", err)
	}

	return true, nil
}
