//go:build wails

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

func init() {
	// Crea file di log nella directory dell'eseguibile
	exeDir := getExeDir()
	logPath := filepath.Join(exeDir, "timetracker.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(logFile)
	}
}

func main() {
	// Inizializza database nella directory dell'eseguibile
	exeDir := getExeDir()
	dbPath := filepath.Join(exeDir, "timetracker.db")
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
	app.SetDB(db)

	// Inizializza variabili globali per tracking
	InitGlobalState()

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
			// Ferma tracking se attivo
			if globalIsTracking {
				app.StopTracking()
			}
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
