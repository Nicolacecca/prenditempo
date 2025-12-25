package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"work-time-tracker-go/tracker"
)

func init() {
	// Crea file di log
	logFile, err := os.OpenFile("timetracker.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(logFile)
	}
}

func main() {
	// Inizializza database
	db, err := tracker.InitDB("timetracker.db")
	if err != nil {
		log.Fatal("Errore DB:", err)
	}
	defer db.Close()

	// Recupera sessioni pendenti da crash precedenti
	recovered, err := tracker.RecoverPendingTracking(db)
	if err != nil {
		log.Printf("[STARTUP] Errore recupero sessioni pendenti: %v\n", err)
	} else if recovered > 0 {
		fmt.Printf("[STARTUP] Recuperate %d sessioni da chiusure anomale precedenti\n", recovered)
	}

	// Se nessun argomento, avvia interfaccia web + system tray
	if len(os.Args) < 2 {
		// Avvia web server in background
		go StartWebServer(db)

		// Avvia system tray (blocca fino alla chiusura)
		StartSystemTray(db)
		return
	}

	command := os.Args[1]

	// Esegui comando
	switch command {
	case "web":
		StartWebServer(db)
	case "tray":
		// Avvia web server in background
		go StartWebServer(db)
		// Avvia system tray (blocca fino alla chiusura)
		StartSystemTray(db)
	case "start":
		// Controlla se c'è il nome del progetto
		if len(os.Args) < 3 {
			fmt.Println("Errore: specificare il nome del progetto")
			fmt.Println("Uso: go run main.go start <nome-progetto> [idle-threshold-minuti]")
			return
		}
		projectName := os.Args[2]

		// Controlla se c'è la soglia idle opzionale
		idleThresholdMinutes := 5 // default: 5 minuti
		if len(os.Args) >= 4 {
			// Prova a parsare il valore
			var err error
			_, err = fmt.Sscanf(os.Args[3], "%d", &idleThresholdMinutes)
			if err != nil {
				fmt.Printf("Errore: soglia idle non valida '%s', uso default 5 minuti\n", os.Args[3])
				idleThresholdMinutes = 5
			}
		}

		cmdStart(db, projectName, idleThresholdMinutes)
	case "projects":
		// Gestisci sottocomandi per progetti
		if len(os.Args) < 3 {
			fmt.Println("Errore: specificare sottocomando (list, add, delete)")
			printUsage()
			return
		}
		subcommand := os.Args[2]
		switch subcommand {
		case "list":
			cmdProjectsList(db)
		case "add":
			if len(os.Args) < 4 {
				fmt.Println("Errore: specificare nome progetto")
				fmt.Println("Uso: go run main.go projects add <nome> [descrizione]")
				return
			}
			projectName := os.Args[3]
			description := ""
			if len(os.Args) >= 5 {
				description = os.Args[4]
			}
			cmdProjectsAdd(db, projectName, description)
		case "delete":
			if len(os.Args) < 4 {
				fmt.Println("Errore: specificare nome progetto")
				fmt.Println("Uso: go run main.go projects delete <nome>")
				return
			}
			projectName := os.Args[3]
			cmdProjectsDelete(db, projectName)
		default:
			fmt.Printf("Sottocomando sconosciuto: %s\n\n", subcommand)
			printUsage()
		}
	case "stats":
		// Controlla se c'è un sottocomando
		if len(os.Args) >= 3 {
			subcommand := os.Args[2]
			switch subcommand {
			case "week":
				cmdStatsWeek(db)
			case "month":
				cmdStatsMonth(db)
			case "top":
				cmdStatsTop(db)
			case "project":
				if len(os.Args) < 4 {
					fmt.Println("Errore: specificare nome progetto")
					fmt.Println("Uso: go run main.go stats project <nome>")
					return
				}
				projectName := os.Args[3]
				cmdStatsProject(db, projectName)
			default:
				fmt.Printf("Sottocomando sconosciuto: %s\n\n", subcommand)
				printUsage()
			}
		} else {
			cmdStats(db)
		}
	case "status":
		cmdStatus()
	case "help":
		printUsage()
	default:
		fmt.Printf("Comando sconosciuto: %s\n\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("=== WORK TIME TRACKER ===")
	fmt.Println("\nUSO:")
	fmt.Println("  go run main.go <comando> [sottocomando]")
	fmt.Println("\nCOMANDI:")
	fmt.Println("  (nessun comando)              - Avvia web server + system tray icon")
	fmt.Println("  tray                          - Avvia web server + system tray icon")
	fmt.Println("  web                           - Avvia solo il web server")
	fmt.Println("  start <progetto> [idle-min]  - Avvia il tracking per un progetto")
	fmt.Println("                                  idle-min: soglia inattivita in minuti (default: 5)")
	fmt.Println("  projects list                 - Lista tutti i progetti")
	fmt.Println("  projects add <nome>           - Crea un nuovo progetto")
	fmt.Println("  projects delete <nome>        - Elimina un progetto")
	fmt.Println("  stats                         - Mostra statistiche di oggi")
	fmt.Println("  stats week                    - Mostra statistiche della settimana")
	fmt.Println("  stats month                   - Mostra statistiche del mese")
	fmt.Println("  stats top                     - Mostra top 10 app piu usate oggi")
	fmt.Println("  stats project <nome>          - Mostra statistiche per progetto")
	fmt.Println("  status                        - Mostra app attualmente attiva")
	fmt.Println("  help                          - Mostra questo messaggio")
	fmt.Println("\nESEMPI:")
	fmt.Println("  go run main.go                           # avvia con tray icon")
	fmt.Println("  go run main.go projects add my-project")
	fmt.Println("  go run main.go start my-project          # usa 5 min idle threshold")
	fmt.Println("  go run main.go start my-project 3        # usa 3 min idle threshold")
	fmt.Println("  go run main.go stats project my-project")
	fmt.Println("  go run main.go projects list")
}

func cmdStart(db *sql.DB, projectName string, idleThresholdMinutes int) {
	// Verifica che il progetto esista
	project, err := tracker.TrovaProgetto(db, projectName)
	if err != nil {
		fmt.Printf("Errore: %v\n", err)
		fmt.Println("Usa 'go run main.go projects add <nome>' per creare un nuovo progetto")
		return
	}

	fmt.Printf("=== TRACKING AVVIATO - Progetto: %s ===\n", project.Name)
	fmt.Printf("Soglia idle: %d minuti\n", idleThresholdMinutes)
	fmt.Println("Premi Ctrl+C per fermare e salvare")
	fmt.Println()

	// Crea watcher
	watcher := tracker.NewTimeWatcher()

	// Imposta soglia idle (converti minuti in secondi)
	watcher.SetIdleThreshold(idleThresholdMinutes * 60)

	// Gestisci Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Avvia tracking
	watcher.Start(5) // Ogni 5 secondi

	// Aspetta Ctrl+C
	<-sigChan

	fmt.Println("\n\n=== FERMANDO TRACKER ===")
	watcher.Stop()

	// Salva statistiche
	stats := watcher.GetStats()
	if len(stats) > 0 {
		fmt.Println("\n=== SALVATAGGIO ===")
		if err := tracker.SalvaStatistiche(db, stats, &project.ID); err != nil {
			log.Printf("Errore salvataggio: %v\n", err)
		}

		// Mostra sessione
		fmt.Printf("\n=== SESSIONE COMPLETATA - Progetto: %s ===\n", project.Name)
		for app, sec := range stats {
			fmt.Printf("- %s: %d secondi (%d minuti)\n", app, sec, sec/60)
		}
	} else {
		fmt.Println("Nessun dato da salvare")
	}

	fmt.Println("\nTracker fermato!")
}

func cmdStats(db *sql.DB) {
	fmt.Println("=== STATISTICHE DI OGGI ===\n")

	stats, err := tracker.CaricaSessioniOggi(db)
	if err != nil {
		log.Fatal("Errore:", err)
	}

	if len(stats) == 0 {
		fmt.Println("Nessuna sessione registrata oggi")
		return
	}

	// Calcola totale
	totale := 0
	for _, sec := range stats {
		totale += sec
	}

	// Mostra statistiche
	fmt.Printf("Tempo totale: %d minuti (%.1f ore)\n\n", totale/60, float64(totale)/3600)
	fmt.Println("Per applicazione:")

	for app, sec := range stats {
		minuti := sec / 60
		percentuale := float64(sec) / float64(totale) * 100
		fmt.Printf("- %s: %d min (%.1f%%)\n", app, minuti, percentuale)
	}
}

func cmdStatsWeek(db *sql.DB) {
	fmt.Println("=== STATISTICHE DELLA SETTIMANA ===\n")

	stats, err := tracker.CaricaSessioniSettimana(db)
	if err != nil {
		log.Fatal("Errore:", err)
	}

	if len(stats) == 0 {
		fmt.Println("Nessuna sessione registrata questa settimana")
		return
	}

	// Calcola totale
	totale := 0
	for _, sec := range stats {
		totale += sec
	}

	// Mostra statistiche
	fmt.Printf("Tempo totale: %d minuti (%.1f ore)\n\n", totale/60, float64(totale)/3600)
	fmt.Println("Per applicazione:")

	for app, sec := range stats {
		minuti := sec / 60
		percentuale := float64(sec) / float64(totale) * 100
		fmt.Printf("- %s: %d min (%.1f%%)\n", app, minuti, percentuale)
	}
}

func cmdStatsMonth(db *sql.DB) {
	fmt.Println("=== STATISTICHE DEL MESE ===\n")

	stats, err := tracker.CaricaSessioniMese(db)
	if err != nil {
		log.Fatal("Errore:", err)
	}

	if len(stats) == 0 {
		fmt.Println("Nessuna sessione registrata questo mese")
		return
	}

	// Calcola totale
	totale := 0
	for _, sec := range stats {
		totale += sec
	}

	// Mostra statistiche
	fmt.Printf("Tempo totale: %d minuti (%.1f ore)\n\n", totale/60, float64(totale)/3600)
	fmt.Println("Per applicazione:")

	for app, sec := range stats {
		minuti := sec / 60
		percentuale := float64(sec) / float64(totale) * 100
		fmt.Printf("- %s: %d min (%.1f%%)\n", app, minuti, percentuale)
	}
}

func cmdStatsTop(db *sql.DB) {
	fmt.Println("=== TOP 10 APPLICAZIONI (OGGI) ===\n")

	topApps, err := tracker.CaricaTopApp(db, 10)
	if err != nil {
		log.Fatal("Errore:", err)
	}

	if len(topApps) == 0 {
		fmt.Println("Nessuna sessione registrata oggi")
		return
	}

	// Calcola totale per percentuali
	totale := 0
	for _, app := range topApps {
		totale += app.Secondi
	}

	// Mostra top 10
	for i, app := range topApps {
		minuti := app.Secondi / 60
		percentuale := float64(app.Secondi) / float64(totale) * 100
		fmt.Printf("%d. %s: %d min (%.1f%%)\n", i+1, app.Nome, minuti, percentuale)
	}

	fmt.Printf("\nTempo totale: %d minuti (%.1f ore)\n", totale/60, float64(totale)/3600)
}

func cmdStatus() {
	fmt.Println("=== APPLICAZIONE ATTIVA ===\n")

	// Ottieni app attiva
	processName, err := tracker.GetActiveProcessName()
	if err != nil {
		log.Fatal("Errore:", err)
	}

	windowTitle, err := tracker.GetActiveWindow()
	if err != nil {
		log.Fatal("Errore:", err)
	}

	fmt.Printf("Processo: %s\n", processName)
	fmt.Printf("Finestra: %s\n", windowTitle)
}

// === COMANDI PER PROGETTI ===

func cmdProjectsList(db *sql.DB) {
	fmt.Println("=== LISTA PROGETTI ===\n")

	projects, err := tracker.CaricaTuttiProgetti(db)
	if err != nil {
		log.Fatal("Errore:", err)
	}

	if len(projects) == 0 {
		fmt.Println("Nessun progetto trovato")
		fmt.Println("Usa 'go run main.go projects add <nome>' per creare un progetto")
		return
	}

	for _, p := range projects {
		if p.Description != "" {
			fmt.Printf("- %s: %s\n", p.Name, p.Description)
		} else {
			fmt.Printf("- %s\n", p.Name)
		}
	}

	fmt.Printf("\nTotale progetti: %d\n", len(projects))
}

func cmdProjectsAdd(db *sql.DB, name, description string) {
	fmt.Printf("=== CREAZIONE PROGETTO ===\n\n")

	id, err := tracker.CreaProgetto(db, name, description)
	if err != nil {
		log.Fatal("Errore:", err)
	}

	fmt.Printf("\nProgetto '%s' creato con successo (ID: %d)\n", name, id)
	fmt.Printf("Ora puoi iniziare il tracking con: go run main.go start %s\n", name)
}

func cmdProjectsDelete(db *sql.DB, name string) {
	fmt.Printf("=== ELIMINAZIONE PROGETTO ===\n\n")

	err := tracker.EliminaProgetto(db, name)
	if err != nil {
		log.Fatal("Errore:", err)
	}

	fmt.Printf("Progetto '%s' eliminato con successo\n", name)
}

func cmdStatsProject(db *sql.DB, projectName string) {
	// Trova il progetto
	project, err := tracker.TrovaProgetto(db, projectName)
	if err != nil {
		log.Fatal("Errore:", err)
	}

	fmt.Printf("=== STATISTICHE PROGETTO: %s ===\n\n", project.Name)

	// Carica statistiche
	stats, err := tracker.CaricaSessioniProgetto(db, project.ID)
	if err != nil {
		log.Fatal("Errore:", err)
	}

	if len(stats) == 0 {
		fmt.Println("Nessuna sessione registrata per questo progetto")
		return
	}

	// Calcola totale
	totale := 0
	for _, sec := range stats {
		totale += sec
	}

	// Mostra statistiche
	fmt.Printf("Tempo totale: %d minuti (%.1f ore)\n\n", totale/60, float64(totale)/3600)
	fmt.Println("Per applicazione:")

	for app, sec := range stats {
		minuti := sec / 60
		percentuale := float64(sec) / float64(totale) * 100
		fmt.Printf("- %s: %d min (%.1f%%)\n", app, minuti, percentuale)
	}
}
