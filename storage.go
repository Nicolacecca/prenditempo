package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	dataDirectoryName = "PrendiTempo"
	databaseFileName  = "timetracker.db"
	logFileName       = "timetracker.log"
)

// getDataDir restituisce la directory persistente per i dati dell'applicazione.
// Su Windows corrisponde a %APPDATA%\PrendiTempo.
func getDataDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("impossibile determinare la directory dati utente: %w", err)
	}

	dataDir := filepath.Join(configDir, dataDirectoryName)
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return "", fmt.Errorf("impossibile creare la directory dati %q: %w", dataDir, err)
	}
	return dataDir, nil
}

// migrateLegacyDatabase copia il database dalla vecchia posizione accanto
// all'eseguibile. La copia originale viene mantenuta come backup di sicurezza.
func migrateLegacyDatabase(legacyPath, destinationPath string) (bool, error) {
	if filepath.Clean(legacyPath) == filepath.Clean(destinationPath) {
		return false, nil
	}
	if _, err := os.Stat(destinationPath); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("impossibile verificare il nuovo database: %w", err)
	}
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("impossibile verificare il database precedente: %w", err)
	}

	createdPaths := make([]string, 0, 3)
	cleanup := func() {
		for _, path := range createdPaths {
			_ = os.Remove(path)
		}
	}

	if err := copyFileAtomic(legacyPath, destinationPath); err != nil {
		return false, fmt.Errorf("impossibile migrare il database: %w", err)
	}
	createdPaths = append(createdPaths, destinationPath)

	// Copia anche eventuali file di recovery prodotti da una chiusura anomala.
	for _, suffix := range []string{"-wal", "-journal"} {
		source := legacyPath + suffix
		if _, err := os.Stat(source); os.IsNotExist(err) {
			continue
		} else if err != nil {
			cleanup()
			return false, fmt.Errorf("impossibile verificare il file di recovery %q: %w", source, err)
		}

		target := destinationPath + suffix
		if err := copyFileAtomic(source, target); err != nil {
			cleanup()
			return false, fmt.Errorf("impossibile migrare il file di recovery %q: %w", source, err)
		}
		createdPaths = append(createdPaths, target)
	}

	return true, nil
}

func copyFileAtomic(sourcePath, destinationPath string) (err error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	temporaryPath := destinationPath + ".migrating"
	_ = os.Remove(temporaryPath)
	destination, err := os.OpenFile(temporaryPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	defer func() {
		_ = destination.Close()
		if err != nil {
			_ = os.Remove(temporaryPath)
		}
	}()

	if _, err = io.Copy(destination, source); err != nil {
		return err
	}
	if err = destination.Sync(); err != nil {
		return err
	}
	if err = destination.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, destinationPath)
}
