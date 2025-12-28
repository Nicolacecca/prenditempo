//go:build !wails

package main

import (
	"database/sql"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"sync"
	"time"
	"work-time-tracker-go/tracker"

	"github.com/getlantern/systray"
)

// Stato globale del tracker
type TrayState struct {
	db               *sql.DB
	isTracking       bool
	currentProject   *tracker.Project
	watcher          *tracker.TimeWatcher
	pendingSessionID int64 // ID della sessione pendente per auto-save
	mu               sync.Mutex
	mStart           *systray.MenuItem
	mStop            *systray.MenuItem
	mStatus          *systray.MenuItem
	mOpenWeb         *systray.MenuItem
	mProjectSubmenu  *systray.MenuItem
	mQuit            *systray.MenuItem
	projects         []tracker.Project
}

var trayState *TrayState

// Inizializza la system tray
func StartSystemTray(db *sql.DB) {
	trayState = &TrayState{
		db:         db,
		isTracking: false,
	}

	systray.Run(onReady, onExit)
}

// Chiamata quando la tray è pronta
func onReady() {
	// Salta l'icona se dà errore, non è critico
	systray.SetIcon(iconData)
	systray.SetTitle("⏱")  // Icona emoji come fallback
	systray.SetTooltip("PrendiTempo - Inattivo")

	// Nota: La notifica di avvio viene gestita da uno script esterno (startup_notification.vbs)
	// che viene eseguito all'avvio di Windows prima che l'app venga avviata dall'utente

	// Crea menu items - Dashboard in cima per accesso rapido
	trayState.mOpenWeb = systray.AddMenuItem("Apri Dashboard", "Apri l'interfaccia web nel browser")

	systray.AddSeparator()

	// Status
	trayState.mStatus = systray.AddMenuItem("Non in tracking", "Stato corrente")
	trayState.mStatus.Disable()

	// Menu per selezionare progetto e avviare tracking
	trayState.mProjectSubmenu = systray.AddMenuItem("Avvia Tracking", "Seleziona un progetto")

	// Menu stop (inizialmente disabilitato)
	trayState.mStop = systray.AddMenuItem("Ferma Tracking", "Ferma il tracking corrente")
	trayState.mStop.Disable()

	systray.AddSeparator()

	// Menu quit
	trayState.mQuit = systray.AddMenuItem("Esci", "Chiudi l'applicazione")

	// Carica progetti e crea sottomenu
	loadProjectsMenu()

	// Gestisci eventi
	go handleMenuEvents()

	// Aggiorna status periodicamente
	go updateStatusPeriodically()
}

// Carica la lista progetti nel submenu
func loadProjectsMenu() {
	projects, err := tracker.CaricaTuttiProgetti(trayState.db)
	if err != nil {
		log.Printf("Errore caricamento progetti: %v", err)
		return
	}

	trayState.projects = projects

	// Aggiungi sottomenu per ogni progetto
	for i := range projects {
		proj := &projects[i] // Crea puntatore alla variabile del loop
		menuItem := trayState.mProjectSubmenu.AddSubMenuItem(proj.Name, fmt.Sprintf("Avvia tracking per %s", proj.Name))

		// Handler per il click sul progetto
		go func(p *tracker.Project, item *systray.MenuItem) {
			for range item.ClickedCh {
				startTracking(p)
			}
		}(proj, menuItem)
	}

	// Aggiungi opzione per creare nuovo progetto
	systray.AddSeparator()
	mNewProject := trayState.mProjectSubmenu.AddSubMenuItem("Nuovo Progetto...", "Apri web per creare progetto")
	go func() {
		for range mNewProject.ClickedCh {
			openWebBrowser("http://localhost:8080/open-dashboard.html")
		}
	}()
}

// Gestisce gli eventi dei menu
func handleMenuEvents() {
	for {
		select {
		case <-trayState.mStop.ClickedCh:
			stopTracking()

		case <-trayState.mOpenWeb.ClickedCh:
			openWebBrowser("http://localhost:8080/open-dashboard.html")

		case <-trayState.mQuit.ClickedCh:
			if trayState.isTracking {
				stopTracking()
			}
			systray.Quit()
			return
		}
	}
}

// Avvia il tracking per un progetto
func startTracking(project *tracker.Project) {
	trayState.mu.Lock()
	defer trayState.mu.Unlock()

	if trayState.isTracking {
		log.Println("Tracking già in corso, fermalo prima di avviarne un altro")
		return
	}

	log.Printf("=== AVVIO TRACKING ===")
	log.Printf("Progetto: %s (ID: %d)", project.Name, project.ID)

	// Crea sessione pendente per auto-save
	startTime := time.Now().Format("2006-01-02 15:04:05")
	pendingSessionID, err := tracker.StartPendingTracking(trayState.db, &project.ID, nil, startTime)
	if err != nil {
		log.Printf("Errore avvio pending tracking: %v", err)
		return
	}
	trayState.pendingSessionID = pendingSessionID

	trayState.currentProject = project
	trayState.isTracking = true

	// Crea watcher
	trayState.watcher = tracker.NewTimeWatcher()
	trayState.watcher.SetIdleThreshold(5 * 60) // 5 minuti di idle

	// Imposta callback per salvataggio periodico (ogni 5 minuti = 300 secondi)
	trayState.watcher.SetSaveCallback(func(totalSeconds int) error {
		return tracker.UpdatePendingTracking(trayState.db, trayState.pendingSessionID, totalSeconds)
	}, 300)

	log.Printf("Watcher creato con auto-save, avvio polling ogni 5 secondi...")

	// Avvia tracking
	trayState.watcher.Start(5) // Controlla ogni 5 secondi

	log.Printf("Watcher.Start() chiamato - Session ID: %d", pendingSessionID)

	// Sincronizza con lo stato globale del web server
	SetGlobalTrackingState(trayState.watcher, project, true, pendingSessionID)

	// Aggiorna UI
	systray.SetIcon(iconDataTracking) // Icona arancione durante tracking
	trayState.mStatus.SetTitle(fmt.Sprintf("Tracking: %s", project.Name))
	systray.SetTooltip(fmt.Sprintf("PrendiTempo - Tracking: %s", project.Name))

	trayState.mProjectSubmenu.Disable()
	trayState.mStop.Enable()

	log.Printf("UI aggiornata - Tracking ATTIVO per: %s", project.Name)

	// Test immediato per vedere se rileva l'app corrente
	go func() {
		time.Sleep(2 * time.Second)
		stats := trayState.watcher.GetStats()
		log.Printf("Statistiche dopo 2 secondi: %+v", stats)
	}()
}

// Ferma il tracking
func stopTracking() {
	trayState.mu.Lock()
	defer trayState.mu.Unlock()

	if !trayState.isTracking {
		return
	}

	log.Println("Fermo tracking...")

	// Ferma watcher
	trayState.watcher.Stop()

	// Finalizza la sessione pendente con i secondi finali
	finalSeconds := trayState.watcher.GetTotalActiveSeconds()
	if trayState.pendingSessionID > 0 {
		err := tracker.FinalizePendingTracking(trayState.db, trayState.pendingSessionID, finalSeconds)
		if err != nil {
			log.Printf("Errore finalizzazione sessione: %v", err)
		} else {
			log.Printf("Sessione finalizzata: ID %d con %d secondi (%d min)",
				trayState.pendingSessionID, finalSeconds, finalSeconds/60)
		}
		trayState.pendingSessionID = 0
	}

	// Reset stato
	trayState.isTracking = false
	trayState.currentProject = nil
	trayState.watcher = nil

	// Sincronizza con lo stato globale del web server
	SetGlobalTrackingState(nil, nil, false, 0)

	// Aggiorna UI
	systray.SetIcon(iconData) // Icona normale quando non in tracking
	trayState.mStatus.SetTitle("Non in tracking")
	systray.SetTooltip("PrendiTempo - Inattivo")

	trayState.mProjectSubmenu.Enable()
	trayState.mStop.Disable()

	log.Println("Tracking fermato")
}

// Aggiorna periodicamente lo status nella tray
func updateStatusPeriodically() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		trayState.mu.Lock()
		if trayState.isTracking && trayState.watcher != nil {
			stats := trayState.watcher.GetStats()

			// Calcola tempo totale
			totalSeconds := 0
			for _, seconds := range stats {
				totalSeconds += seconds
			}

			minutes := totalSeconds / 60
			hours := minutes / 60
			mins := minutes % 60

			tooltip := fmt.Sprintf("PrendiTempo - Tracking: %s\nTempo: %dh %dm",
				trayState.currentProject.Name, hours, mins)
			systray.SetTooltip(tooltip)
		}
		trayState.mu.Unlock()
	}
}

// Apre il browser con l'URL specificato in una nuova finestra
func openWebBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// Usa PowerShell per aprire in una nuova finestra del browser predefinito
		psCmd := fmt.Sprintf(`
			$browserPath = (Get-ItemProperty 'HKCU:\Software\Microsoft\Windows\Shell\Associations\UrlAssociations\http\UserChoice').ProgId
			if ($browserPath -match 'Chrome') {
				Start-Process chrome.exe -ArgumentList '--new-window','%s'
			} elseif ($browserPath -match 'Firefox') {
				Start-Process firefox.exe -ArgumentList '-new-window','%s'
			} elseif ($browserPath -match 'Edge') {
				Start-Process msedge.exe -ArgumentList '--new-window','%s'
			} elseif ($browserPath -match 'Brave') {
				Start-Process brave.exe -ArgumentList '--new-window','%s'
			} else {
				Start-Process '%s'
			}
		`, url, url, url, url, url)
		cmd = exec.Command("powershell", "-WindowStyle", "Hidden", "-Command", psCmd)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux, freebsd, etc.
		cmd = exec.Command("xdg-open", url)
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Errore apertura browser: %v", err)
	}
}

// Chiamata quando la tray viene chiusa
func onExit() {
	log.Println("System tray chiusa")
	if trayState.isTracking {
		stopTracking()
	}
}
