package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"work-time-tracker-go/tracker"
)

//go:embed all:frontend/dist
var assets embed.FS

// getExeDir restituisce la directory dell'eseguibile
func getExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func main() {
	dataDir, err := getDataDir()
	if err != nil {
		log.Fatal("Errore inizializzazione directory dati:", err)
	}

	logPath := filepath.Join(dataDir, logFileName)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err == nil {
		defer logFile.Close()
		log.SetOutput(logFile)
	}

	// Le versioni precedenti salvavano il database accanto all'eseguibile.
	// Se necessario lo copiamo nella directory dati, mantenendo l'originale.
	exeDir := getExeDir()
	dbPath := filepath.Join(dataDir, databaseFileName)
	legacyDBPath := filepath.Join(exeDir, databaseFileName)
	migrated, err := migrateLegacyDatabase(legacyDBPath, dbPath)
	if err != nil {
		log.Fatal("Errore migrazione database:", err)
	}
	if migrated {
		log.Printf("[STARTUP] Database migrato da %s a %s; la copia originale è stata mantenuta\n", legacyDBPath, dbPath)
	}

	db, err := tracker.InitDB(dbPath)
	if err != nil {
		log.Fatal("Errore DB:", err)
	}

	// Recupera sessioni pendenti da crash precedenti
	recovered, err := tracker.RecoverPendingTracking(db)
	if err != nil {
		log.Printf("[STARTUP] Errore recupero sessioni pendenti: %v\n", err)
	} else if recovered > 0 {
		log.Printf("[STARTUP] Recuperate %d sessioni da chiusure anomale precedenti\n", recovered)
	}

	// Crea istanza App
	app := NewApp()
	app.SetDB(db) // inizializza anche lo stato del tracking e la soglia di inattività

	// Crea applicazione Wails
	err = wails.Run(&options.App{
		Title:            "PrendiTempo",
		Width:            1024,
		Height:           700,
		MinWidth:         800,
		MinHeight:        500,
		Frameless:        false,
		WindowStartState: options.Maximised,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 26, G: 26, B: 26, A: 1},
		OnStartup:        app.startup,
		OnShutdown: func(ctx context.Context) {
			// StopTracking gestisce in modo sincronizzato anche il caso inattivo.
			_, _ = app.StopTracking()
			db.Close()
		},
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			DisableFramelessWindowDecorations: true,
		},
	})

	if err != nil {
		log.Fatal("Errore avvio Wails:", err)
	}
}
