package main

import (
	"context"
	"database/sql"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct per Wails - espone metodi al frontend
type App struct {
	ctx   context.Context
	db    *sql.DB
	track trackingState
}

// NewApp crea una nuova istanza App
func NewApp() *App {
	return &App{}
}

// startup viene chiamato all'avvio dell'app
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// SetDB imposta il database e inizializza lo stato del tracking (soglia di inattività inclusa)
func (a *App) SetDB(db *sql.DB) {
	a.db = db
	a.resetTrackingState()
}

// BringWindowToFront porta la finestra dell'applicazione in primo piano
func (a *App) BringWindowToFront() {
	// De-minimizza la finestra se minimizzata
	runtime.WindowUnminimise(a.ctx)
	// Mostra la finestra
	runtime.WindowShow(a.ctx)
	// Imposta always on top per forzare il primo piano
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
}

// RestoreNormalWindow rimuove always on top (da chiamare dopo che l'utente ha interagito)
func (a *App) RestoreNormalWindow() {
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
}
