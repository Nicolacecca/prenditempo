# PrendiTempo

App desktop nativa per Windows per il tracciamento del tempo lavorativo, sviluppata con Go e Wails.

## Funzionalità

- **Tracciamento tempo** - Avvia e ferma il tracciamento con un click, selezionando progetto e tipo di attività
- **Timeline visuale** - Visualizza le sessioni di lavoro su una timeline interattiva con marker temporali
- **Tipi di attività** - Crea tipi di attività personalizzati con colori e pattern diversi (solido, strisce, puntini)
- **Note** - Aggiungi note ai progetti con timestamp automatico
- **Archiviazione progetti** - Chiudi e archivia progetti completati con report finale
- **Report** - Genera report dettagliati con statistiche su tempo totale, sessioni e attività
- **Esportazione JSON** - Esporta progetti in formato JSON per backup o reimportazione futura
- **Stampa PDF** - Stampa i report direttamente in PDF
- **Rilevamento inattività** - Rileva i periodi di inattività e permette di attribuire il tempo al progetto corretto
- **Avvio automatico** - Opzione per avviare l'app automaticamente con Windows
- **System tray** - L'app rimane attiva nella system tray per un accesso rapido

## Screenshot
<img width="1907" height="1031" alt="dashboard" src="https://github.com/user-attachments/assets/c7446f69-b017-4cdd-ae9e-789d09007fac" />
<img width="1920" height="1031" alt="Archivio" src="https://github.com/user-attachments/assets/3d7e6c4c-605e-4f6e-8d24-1a7f25e3aa65" />
<img width="1905" height="1032" alt="Impostazioni" src="https://github.com/user-attachments/assets/b429c734-847c-48c3-b2bb-f81c90b14c18" />

## Requisiti

- Windows 10/11
- WebView2 Runtime (incluso in Windows 11, installabile su Windows 10)

## Installazione

1. Scarica l'ultima release dalla pagina [Releases](../../releases)
2. Estrai `PrendiTempo.exe` in una cartella a tua scelta
3. Avvia l'applicazione

## Compilazione da sorgente

### Requisiti di sviluppo
- Go 1.21+
- Node.js 18+
- Wails CLI v2

### Procedura di build

```bash
# Installa Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Clona il repository
git clone https://github.com/Nicolacecca/prenditempo.git
cd prenditempo

# Compila l'applicazione
wails build -tags wails
```

L'eseguibile sarà disponibile in `build/bin/PrendiTempo.exe`

### Modalità sviluppo

```bash
wails dev -tags wails
```

## Tecnologie utilizzate

- **Backend**: Go
- **Frontend**: HTML/CSS/JavaScript (vanilla)
- **Framework desktop**: [Wails v2](https://wails.io/)
- **Database**: SQLite
- **UI Theme**: Dark mode con accenti arancioni

## Struttura del progetto

```
prenditempo/
├── build/              # Configurazione build Wails
├── frontend/           # Frontend HTML/CSS/JS
│   ├── src/
│   │   ├── app.js      # Logica applicazione
│   │   └── style.css   # Stili
│   └── index.html      # Pagina principale
├── tracker/            # Package database
├── main.go             # Entry point
├── wails_app.go        # Metodi esposti al frontend
└── wails.json          # Configurazione Wails
```

## Utilizzo

1. **Avvia l'app** - L'icona apparirà nella system tray
2. **Crea un progetto** - Clicca su "Nuovo Progetto" nella dashboard
3. **Avvia il tracciamento** - Seleziona progetto e tipo di attività, poi clicca "Avvia"
4. **Visualizza la timeline** - Vai nella sezione "Timeline Progetti" per vedere le sessioni
5. **Archivia progetti** - Clicca sull'icona archivio per chiudere un progetto e generare il report
6. **Esporta backup** - Dal report puoi esportare in JSON per backup

## Licenza

MIT License

## Crediti

Sviluppato con l'assistenza di [Claude AI](https://claude.ai) (Anthropic).
