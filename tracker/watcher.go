package tracker

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

const (
	PROCESS_QUERY_INFORMATION = 0x0400
	PROCESS_VM_READ           = 0x0010
)

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWin         = user32.NewProc("GetForegroundWindow")
	procGetWindowText            = user32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procGetLastInputInfo         = user32.NewProc("GetLastInputInfo")

	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procGetTickCount    = kernel32.NewProc("GetTickCount")

	psapi                       = syscall.NewLazyDLL("psapi.dll")
	procGetProcessImageFileName = psapi.NewProc("GetProcessImageFileNameW")
)

// LASTINPUTINFO struct per GetLastInputInfo
type LASTINPUTINFO struct {
	CbSize uint32
	DwTime uint32
}

// GetActiveWindow ritorna il titolo della finestra attualmente attiva
func GetActiveWindow() (string, error) {
	// Ottieni handle della finestra attiva
	hwnd, _, _ := procGetForegroundWin.Call()
	if hwnd == 0 {
		return "", fmt.Errorf("nessuna finestra attiva")
	}

	// Buffer per il titolo (max 256 caratteri)
	buf := make([]uint16, 256)

	// Ottieni il titolo della finestra
	procGetWindowText.Call(
		hwnd,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)

	// Converti da UTF-16 a string
	title := syscall.UTF16ToString(buf)

	if title == "" {
		return "", fmt.Errorf("finestra senza titolo")
	}

	return title, nil
}

// GetActiveProcessName ritorna il nome del processo attivo
func GetActiveProcessName() (string, error) {
	// Ottieni handle della finestra attiva
	hwnd, _, _ := procGetForegroundWin.Call()
	if hwnd == 0 {
		return "", fmt.Errorf("nessuna finestra attiva")
	}

	// Ottieni Process ID dalla finestra
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))

	if pid == 0 {
		return "", fmt.Errorf("impossibile ottenere PID")
	}

	// Apri il processo
	handle, err := syscall.OpenProcess(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ, false, pid)
	if err != nil {
		return "", fmt.Errorf("impossibile aprire processo: %v", err)
	}
	defer syscall.CloseHandle(handle)

	// Buffer per il path
	buf := make([]uint16, 260)
	size := uint32(len(buf))

	// Ottieni il path completo dell'exe
	ret, _, _ := procGetProcessImageFileName.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(size),
	)

	if ret == 0 {
		return "", fmt.Errorf("impossibile ottenere path processo")
	}

	// Converti il path
	fullPath := syscall.UTF16ToString(buf)

	// Estrai solo il nome del file (es. "Code.exe" da "C:\...\Code.exe")
	processName := extractFileName(fullPath)

	return processName, nil
}

// extractFileName estrae il nome del file dal path completo
func extractFileName(path string) string {
	// Trova l'ultima barra
	lastSlash := -1
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '\\' || path[i] == '/' {
			lastSlash = i
			break
		}
	}

	if lastSlash == -1 {
		return path
	}

	return path[lastSlash+1:]
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

// AppSession rappresenta una sessione di utilizzo di un'applicazione
type AppSession struct {
	AppName   string
	StartTime time.Time
	Duration  int // secondi
}

// SaveCallback è la funzione chiamata per salvare periodicamente i dati
type SaveCallback func(totalSeconds int) error

// TimeWatcher traccia il tempo delle applicazioni
type TimeWatcher struct {
	appTimes           map[string]int // nome app -> secondi (per statistiche dettagliate)
	sessions           []AppSession   // sessioni dettagliate con timestamp
	currentApp         string         // app correntemente tracciata
	currentStartTime   time.Time      // quando è iniziata la sessione corrente
	stopChan           chan bool
	running            bool
	idleThreshold      int         // soglia idle in secondi (es. 300 = 5 minuti)
	isIdle             bool        // stato idle corrente
	idleStartTime      time.Time   // quando è iniziato l'idle corrente
	pendingIdlePeriod  *IdlePeriod // periodo idle in attesa di attribuzione
	trackingStartTime  time.Time   // quando è iniziato il tracking (per sessione unica)
	totalActiveSeconds int         // secondi totali attivi (escluso idle)
	saveCallback       SaveCallback // callback per salvataggio periodico
	saveInterval       int          // intervallo salvataggio in secondi (default 300 = 5 min)
	lastSaveSeconds    int          // secondi all'ultimo salvataggio
}

// NewTimeWatcher crea un nuovo watcher
func NewTimeWatcher() *TimeWatcher {
	return &TimeWatcher{
		appTimes:           make(map[string]int),
		sessions:           []AppSession{},
		currentApp:         "",
		currentStartTime:   time.Time{},
		stopChan:           make(chan bool),
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
	w.idleThreshold = seconds
}

// Start avvia il tracciamento
func (w *TimeWatcher) Start(intervalSeconds int) {
	if w.running {
		fmt.Println("[WATCHER] Gia in esecuzione")
		return
	}

	w.running = true
	w.trackingStartTime = time.Now() // Memorizza quando è iniziato il tracking
	w.totalActiveSeconds = 0         // Reset contatore
	w.lastSaveSeconds = 0            // Reset ultimo salvataggio
	fmt.Printf("[WATCHER] Avviato (intervallo: %d secondi, auto-save ogni %d secondi)\n", intervalSeconds, w.saveInterval)

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
				// Controlla idle time
				idleTime, err := GetIdleTime()
				if err != nil {
					fmt.Printf("[IDLE] Errore rilevamento idle: %v\n", err)
					continue
				}

				// Se idle time supera la soglia, PC è inattivo
				if idleTime >= w.idleThreshold {
					// Se non era già in idle, registra inizio periodo idle
					if !w.isIdle {
						w.isIdle = true
						w.idleStartTime = time.Now()
						fmt.Printf("[IDLE] Sistema inattivo da %d secondi (soglia: %d sec)\n", idleTime, w.idleThreshold)
					}
					// NON tracciare quando idle
					continue
				}

				// Se era in idle ed è tornato attivo
				if w.isIdle {
					w.isIdle = false
					endTime := time.Now()
					duration := int(endTime.Sub(w.idleStartTime).Seconds())

					// Crea periodo idle in attesa di attribuzione
					w.pendingIdlePeriod = &IdlePeriod{
						StartTime: w.idleStartTime,
						EndTime:   endTime,
						Duration:  duration,
					}

					fmt.Printf("[IDLE] Sistema riattivato - periodo idle: %d minuti (dal %s al %s)\n",
						duration/60,
						w.idleStartTime.Format("15:04:05"),
						endTime.Format("15:04:05"))
				}

				// Sistema attivo: rileva app e traccia
				processName, err := GetActiveProcessName()
				if err != nil {
					continue
				}

				// Accumula tempo totale attivo (per la sessione unica)
				w.totalActiveSeconds += intervalSeconds

				// Accumula tempo per app (per statistiche dettagliate)
				w.appTimes[processName] += intervalSeconds

				// Aggiorna app corrente (solo per info)
				w.currentApp = processName

				fmt.Printf("[TRACK] %s: %d sec | Totale sessione: %d sec (%d min)\n",
					processName, w.appTimes[processName],
					w.totalActiveSeconds, w.totalActiveSeconds/60)

				// Salvataggio periodico (ogni saveInterval secondi)
				if w.saveCallback != nil && (w.totalActiveSeconds-w.lastSaveSeconds) >= w.saveInterval {
					err := w.saveCallback(w.totalActiveSeconds)
					if err != nil {
						fmt.Printf("[AUTOSAVE] Errore salvataggio: %v\n", err)
					} else {
						w.lastSaveSeconds = w.totalActiveSeconds
						fmt.Printf("[AUTOSAVE] Salvato automaticamente: %d secondi (%d min)\n",
							w.totalActiveSeconds, w.totalActiveSeconds/60)
					}
				}
			}
		}
	}()
}

// Stop ferma il tracciamento
func (w *TimeWatcher) Stop() {
	if !w.running {
		fmt.Println("[WATCHER] Non in esecuzione")
		return
	}

	// Crea una singola sessione con il tempo totale accumulato
	if w.totalActiveSeconds > 0 {
		w.sessions = []AppSession{
			{
				AppName:   "Sessione di lavoro", // Nome generico per la sessione unica
				StartTime: w.trackingStartTime,
				Duration:  w.totalActiveSeconds,
			},
		}
		fmt.Printf("[TRACK] Sessione unica salvata: %d secondi (%d min) dal %s\n",
			w.totalActiveSeconds, w.totalActiveSeconds/60,
			w.trackingStartTime.Format("15:04:05"))
	}

	w.running = false
	w.stopChan <- true
}

// GetStats ritorna le statistiche
func (w *TimeWatcher) GetStats() map[string]int {
	return w.appTimes
}

// GetPendingIdlePeriod restituisce il periodo idle in attesa di attribuzione
func (w *TimeWatcher) GetPendingIdlePeriod() *IdlePeriod {
	return w.pendingIdlePeriod
}

// ClearPendingIdlePeriod rimuove il periodo idle pendente
func (w *TimeWatcher) ClearPendingIdlePeriod() {
	w.pendingIdlePeriod = nil
}

// HasPendingIdlePeriod verifica se c'è un periodo idle in attesa
func (w *TimeWatcher) HasPendingIdlePeriod() bool {
	return w.pendingIdlePeriod != nil
}

// GetSessions restituisce le sessioni dettagliate con timestamp
func (w *TimeWatcher) GetSessions() []AppSession {
	return w.sessions
}

// GetTotalActiveSeconds restituisce i secondi totali attivi
func (w *TimeWatcher) GetTotalActiveSeconds() int {
	return w.totalActiveSeconds
}

// GetTrackingStartTime restituisce quando è iniziato il tracking
func (w *TimeWatcher) GetTrackingStartTime() time.Time {
	return w.trackingStartTime
}
