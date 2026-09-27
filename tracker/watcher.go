package tracker

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")

	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procGetTickCount = kernel32.NewProc("GetTickCount")
)

// LASTINPUTINFO struct per GetLastInputInfo
type LASTINPUTINFO struct {
	CbSize uint32
	DwTime uint32
}

// GetIdleTime restituisce da quanto tempo (in secondi) il PC è idle
func GetIdleTime() (int, error) {
	var lastInputInfo LASTINPUTINFO
	lastInputInfo.CbSize = uint32(unsafe.Sizeof(lastInputInfo))

	// Chiama GetLastInputInfo
	ret, _, err := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lastInputInfo)))
	if ret == 0 {
		return 0, fmt.Errorf("errore GetLastInputInfo: %v", err)
	}

	// Ottieni il tick count corrente (millisecondi da avvio sistema)
	currentTick, _, _ := procGetTickCount.Call()

	// Calcola la differenza in millisecondi
	idleMillis := uint32(currentTick) - lastInputInfo.DwTime

	// Converti in secondi
	idleSeconds := int(idleMillis / 1000)

	return idleSeconds, nil
}

// IdlePeriod rappresenta un periodo di inattività
type IdlePeriod struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  int // secondi
}

// SaveCallback è la funzione chiamata per salvare periodicamente i dati
type SaveCallback func(totalSeconds int) error

// OnIdleCallback è la funzione chiamata quando viene rilevato l'idle
type OnIdleCallback func()

// OnIdleReturnCallback è la funzione chiamata quando l'utente torna dall'idle
type OnIdleReturnCallback func(minutes int)

// TimeWatcher traccia il tempo delle applicazioni
type TimeWatcher struct {
	mu                   sync.Mutex // mutex per proteggere accesso concorrente
	stopChan             chan struct{}
	running              bool
	countingEnabled      bool                 // il watcher può restare vivo solo per rilevare il rientro dall'idle
	stopOnce             sync.Once            // previene doppia chiusura del canale
	idleThreshold        int                  // soglia idle in secondi (es. 300 = 5 minuti)
	isIdle               bool                 // stato idle corrente
	idleStartTime        time.Time            // quando è iniziato l'idle corrente
	pendingIdlePeriod    *IdlePeriod          // periodo idle in attesa di attribuzione
	trackingStartTime    time.Time            // quando è iniziato il tracking (per sessione unica)
	totalActiveSeconds   int                  // secondi totali attivi (escluso idle)
	saveCallback         SaveCallback         // callback per salvataggio periodico
	saveInterval         int                  // intervallo salvataggio in secondi (default 300 = 5 min)
	lastSaveSeconds      int                  // secondi all'ultimo salvataggio
	onIdleCallback       OnIdleCallback       // callback chiamata quando viene rilevato idle
	onIdleReturnCallback OnIdleReturnCallback // callback chiamata quando l'utente torna dall'idle
}

// NewTimeWatcher crea un nuovo watcher
func NewTimeWatcher() *TimeWatcher {
	return &TimeWatcher{
		stopChan:           make(chan struct{}),
		running:            false,
		idleThreshold:      300, // default: 5 minuti
		isIdle:             false,
		pendingIdlePeriod:  nil,
		trackingStartTime:  time.Time{},
		totalActiveSeconds: 0,
		saveCallback:       nil,
		saveInterval:       300, // default: 5 minuti
		lastSaveSeconds:    0,
	}
}

// SetSaveCallback imposta la callback per il salvataggio periodico
func (w *TimeWatcher) SetSaveCallback(callback SaveCallback, intervalSeconds int) {
	w.saveCallback = callback
	if intervalSeconds > 0 {
		w.saveInterval = intervalSeconds
	}
}

// SetIdleThreshold imposta la soglia idle (in secondi)
func (w *TimeWatcher) SetIdleThreshold(seconds int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.idleThreshold = seconds
}

// SetOnIdleCallback imposta la callback da chiamare quando viene rilevato l'idle
func (w *TimeWatcher) SetOnIdleCallback(callback OnIdleCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onIdleCallback = callback
}

// SetOnIdleReturnCallback imposta la callback da chiamare quando l'utente torna dall'idle
func (w *TimeWatcher) SetOnIdleReturnCallback(callback OnIdleReturnCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onIdleReturnCallback = callback
}

// Start avvia il tracciamento
func (w *TimeWatcher) Start(intervalSeconds int) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		fmt.Println("[WATCHER] Gia in esecuzione")
		return
	}

	w.running = true
	w.countingEnabled = true
	w.trackingStartTime = time.Now() // Memorizza quando è iniziato il tracking
	w.totalActiveSeconds = 0         // Reset contatore
	w.lastSaveSeconds = 0            // Reset ultimo salvataggio
	w.stopOnce = sync.Once{}         // Reset stopOnce per permettere nuove chiamate a Stop
	w.stopChan = make(chan struct{}) // Un watcher riavviato usa un nuovo segnale di stop
	saveInterval := w.saveInterval
	saveCallback := w.saveCallback
	w.mu.Unlock()

	fmt.Printf("[WATCHER] Avviato (intervallo: %d secondi, auto-save ogni %d secondi)\n", intervalSeconds, saveInterval)

	// Goroutine per il tracking
	go func() {
		ticker := time.NewTicker(time.Duration(intervalSeconds) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-w.stopChan:
				fmt.Println("[WATCHER] Fermato")
				return
			case <-ticker.C:
				// Leggi soglia idle corrente (può essere aggiornata)
				w.mu.Lock()
				currentIdleThreshold := w.idleThreshold
				w.mu.Unlock()

				// Controlla idle time
				idleTime, err := GetIdleTime()
				if err != nil {
					fmt.Printf("[IDLE] Errore rilevamento idle: %v\n", err)
					continue
				}

				w.mu.Lock()
				// Se idle time supera la soglia, PC è inattivo
				if idleTime >= currentIdleThreshold {
					// Se non era già in idle, registra inizio periodo idle
					var idleCallback OnIdleCallback
					if !w.isIdle {
						w.isIdle = true
						// Calcola il vero inizio dell'idle sottraendo il tempo già trascorso
						w.idleStartTime = time.Now().Add(-time.Duration(idleTime) * time.Second)

						// CORREZIONE: Sottrai il tempo idle che è stato erroneamente contato come attivo
						// Durante il periodo prima di rilevare l'idle, continuavamo a tracciare
						// ma l'utente era già inattivo
						w.totalActiveSeconds -= idleTime
						if w.totalActiveSeconds < 0 {
							w.totalActiveSeconds = 0
						}

						idleCallback = w.onIdleCallback // Cattura callback prima di unlock
						fmt.Printf("[IDLE] Sistema inattivo da %d secondi (soglia: %d sec), inizio idle: %s, secondi attivi corretti: %d\n",
							idleTime, currentIdleThreshold, w.idleStartTime.Format("15:04:05"), w.totalActiveSeconds)
					}
					w.mu.Unlock()

					// Chiama callback DOPO unlock per evitare deadlock
					if idleCallback != nil {
						fmt.Println("[IDLE] Chiamata callback onIdle...")
						idleCallback()
					}

					// NON tracciare quando idle
					continue
				}

				// Se era in idle ed è tornato attivo
				var idleReturnCallback OnIdleReturnCallback
				var idleMinutes int
				if w.isIdle {
					w.isIdle = false
					endTime := time.Now()
					duration := int(endTime.Sub(w.idleStartTime).Seconds())
					idleMinutes = duration / 60

					// Crea periodo idle in attesa di attribuzione
					w.pendingIdlePeriod = &IdlePeriod{
						StartTime: w.idleStartTime,
						EndTime:   endTime,
						Duration:  duration,
					}

					// Cattura callback prima di unlock
					idleReturnCallback = w.onIdleReturnCallback

					fmt.Printf("[IDLE] Sistema riattivato - periodo idle: %d minuti (dal %s al %s)\n",
						idleMinutes,
						w.idleStartTime.Format("15:04:05"),
						endTime.Format("15:04:05"))
				}
				w.mu.Unlock()

				// Chiama callback DOPO unlock per evitare deadlock
				if idleReturnCallback != nil && idleMinutes > 0 {
					fmt.Println("[IDLE] Chiamata callback onIdleReturn...")
					idleReturnCallback(idleMinutes)
				}

				// Dopo l'auto-stop il watcher rimane attivo solo per intercettare
				// il rientro dall'idle e non deve ricominciare a conteggiare.
				w.mu.Lock()
				countingEnabled := w.countingEnabled
				w.mu.Unlock()
				if !countingEnabled {
					continue
				}

				// Sistema attivo: accumula tempo per la sessione corrente
				w.mu.Lock()
				w.totalActiveSeconds += intervalSeconds
				totalActive := w.totalActiveSeconds
				lastSave := w.lastSaveSeconds
				w.mu.Unlock()

				fmt.Printf("[TRACK] Totale sessione: %d sec (%d min)\n", totalActive, totalActive/60)

				// Salvataggio periodico (ogni saveInterval secondi)
				if saveCallback != nil && (totalActive-lastSave) >= saveInterval {
					err := saveCallback(totalActive)
					if err != nil {
						fmt.Printf("[AUTOSAVE] Errore salvataggio: %v\n", err)
					} else {
						w.mu.Lock()
						w.lastSaveSeconds = totalActive
						w.mu.Unlock()
						fmt.Printf("[AUTOSAVE] Salvato automaticamente: %d secondi (%d min)\n",
							totalActive, totalActive/60)
					}
				}
			}
		}
	}()
}

// Stop ferma il tracciamento
func (w *TimeWatcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		fmt.Println("[WATCHER] Non in esecuzione")
		return
	}

	if w.totalActiveSeconds > 0 {
		fmt.Printf("[TRACK] Sessione unica salvata: %d secondi (%d min) dal %s\n",
			w.totalActiveSeconds, w.totalActiveSeconds/60,
			w.trackingStartTime.Format("15:04:05"))
	}

	w.running = false
	w.countingEnabled = false
	w.mu.Unlock()

	// La chiusura del canale è non bloccante: Stop può quindi essere chiamato
	// anche mentre la goroutine sta eseguendo una callback.
	w.stopOnce.Do(func() {
		close(w.stopChan)
	})
}

// PauseCounting interrompe conteggio e autosalvataggi lasciando vivo il watcher
// per rilevare il rientro da un periodo di inattività.
func (w *TimeWatcher) PauseCounting() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.countingEnabled = false
}

// GetPendingIdlePeriod restituisce il periodo idle in attesa di attribuzione
func (w *TimeWatcher) GetPendingIdlePeriod() *IdlePeriod {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pendingIdlePeriod
}

// ClearPendingIdlePeriod rimuove il periodo idle pendente
func (w *TimeWatcher) ClearPendingIdlePeriod() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pendingIdlePeriod = nil
}

// GetTotalActiveSeconds restituisce i secondi totali attivi
func (w *TimeWatcher) GetTotalActiveSeconds() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.totalActiveSeconds
}
