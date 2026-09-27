# Lavagna infinita per progetto

Stato: versione avanzata implementata nel codice. Sono disponibili ribbon desktop a schede, testo, note, tabelle, immagini persistenti, collegamenti video, disegno libero, raccolta forme, frecce libere e collegate, cartelle locali/rete, URL Internet, gruppi, cornici, blocco, pannello oggetti e livelli, ricerca, minimappa navigabile, allineamento, griglia magnetica, stili, menu contestuale, copia/taglia/incolla, cestino, versioni automatiche e nominate, esportazione PNG/PDF, pan, zoom, undo/redo e autosalvataggio.

## Obiettivo

Ogni progetto dispone di una sola lavagna infinita locale. La lavagna consente di organizzare liberamente:

- testo e note adesive;
- immagini;
- video locali;
- tabelle modificabili;
- rettangoli, ellissi e altre forme;
- frecce e collegamenti fra elementi;
- disegno libero, linee ed evidenziatori;
- gruppi, livelli e cornici;
- collegamenti e allegati.

La lavagna deve funzionare anche sui progetti archiviati. L'archiviazione non modifica o elimina il suo contenuto.

## Interazione desktop-first

La progettazione assume come dispositivo principale un PC senza touchscreen.

- Click sinistro: selezione e trascinamento.
- Doppio click o `Invio`: modifica del testo o della cella selezionata.
- `Shift` + click: selezione multipla.
- Rotellina: zoom centrato sulla posizione del puntatore.
- Pulsante centrale oppure `Spazio` + trascinamento: spostamento della visuale.
- Click destro: menu contestuale.
- `Ctrl+C`, `Ctrl+V`, `Ctrl+X`: copia, incolla e taglia.
- `Ctrl+Z`, `Ctrl+Y`: annulla e ripristina.
- `Canc`: elimina la selezione con possibilità di annullamento.
- Frecce direzionali: spostamento di 1 px; con `Shift`, di 10 px.
- `Ctrl+A`: seleziona tutto; `Esc`: termina lo strumento o deseleziona.
- `Ctrl+0`: adatta il contenuto; `Ctrl++` e `Ctrl+-`: controllo dello zoom.
- `Ctrl+S`: salvataggio immediato.
- `Alt+H`, `Alt+I`, `Alt+D`, `Alt+V`, `Alt+E`: apertura delle schede principali del ribbon.

La barra degli strumenti usa un ribbon originale PrendiTempo ispirato ai modelli di interazione delle applicazioni desktop: schede Home, Inserisci, Disegno, Visualizza ed Esporta, gruppi di comandi, accesso rapido e scheda contestuale Formato. Il ribbon mostra tooltip e scorciatoie, disabilita i comandi non applicabili, è comprimibile, rimane utilizzabile con tastiera e non richiede gesti touch. Aspetto, colori, icone e componenti devono restare originali e compatibili con una futura distribuzione commerciale.

Ogni oggetto collegabile espone otto agganci perimetrali: i quattro vertici e le quattro mezzerie dei lati. La coppia iniziale viene scelta in base alla distanza minima. Selezionando un connettore vengono visualizzati gli agganci e due maniglie colorate; trascinando una maniglia, l’estremità scatta automaticamente sul punto più vicino e conserva la scelta anche dopo spostamento, ridimensionamento, rotazione, salvataggio e riapertura. I collegamenti salvati con il precedente sistema a sei punti vengono convertiti automaticamente.

La scheda Home distingue la formattazione del testo dallo stile degli oggetti. Il testo dispone di colore, carattere, dimensione, normale, grassetto, corsivo, sottolineato, pedice e apice, oltre a elenchi puntati, numerati e checklist. Il comando `Eseguito` alterna le attività fra `☐ da fare` e `☑ completata`. Linee, frecce, collegamenti, forme e cornici dispongono di colore e spessore del contorno; gli oggetti con un'area interna consentono inoltre di scegliere riempimento e trasparenza in modo indipendente. I selettori di colore offrono una palette rapida e l'accesso alla tavolozza completa.

Le Note conservano dimensioni proprie per il foglietto: ridimensionare la Nota non scala il carattere. Larghezza e altezza restano memorizzate anche modificando successivamente il contenuto, aggiungendo elenchi o riaprendo la lavagna.

Nel modificatore delle tabelle scorre soltanto la griglia delle celle; i pulsanti `Applica` e `Annulla` restano fissi e visibili anche con molte righe o con una finestra ridotta.

## Architettura proposta

### Motore grafico

Usare Fabric.js, isolato in un modulo frontend dedicato. Offre selezione, trasformazioni, viewport, immagini e serializzazione JSON senza introdurre React. Frecce con collegamenti persistenti, tabelle e video richiedono componenti applicativi specifici.

Il documento della lavagna deve avere un proprio `schema_version`: il JSON prodotto dalla libreria non va considerato un formato stabile sufficiente da solo.

### Persistenza

Nuova tabella SQLite:

```sql
CREATE TABLE project_boards (
    project_id INTEGER PRIMARY KEY,
    schema_version INTEGER NOT NULL,
    document_json TEXT NOT NULL,
    viewport_json TEXT NOT NULL DEFAULT '{}',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
```

Gli aggiornamenti devono essere salvati automaticamente con debounce, indicativamente 750–1000 ms, e scritti in transazione. L'interfaccia mostra gli stati `Salvataggio…`, `Salvato` ed `Errore` senza bloccare l'editing.

La tabella `board_versions` conserva versioni nominate e una cronologia automatica. Prima di sostituire lo stato corrente con una versione precedente viene generata una copia di sicurezza. Le versioni automatiche sono limitate alle 20 più recenti; per ogni progetto sono consentite fino a 50 versioni nominate.

Gli elementi eliminati vengono conservati nel cestino interno al documento della lavagna, fino a un massimo di 100 elementi, e possono essere ripristinati singolarmente o tutti insieme.

### Immagini

Le immagini PNG, JPEG, GIF e WebP fino a 10 MiB vengono verificate tramite firma del file, deduplicate con SHA-256 e salvate nella tabella `board_assets`. Il documento Fabric contiene soltanto l'identificativo dell'asset: i dati Base64 vengono generati al caricamento e non appesantiscono il JSON della lavagna.

La proprietà dell'asset viene verificata per progetto e l'eliminazione del progetto rimuove automaticamente le sue immagini. Il formato SVG non viene accettato per evitare contenuti attivi.

### Video

La prima versione tratta i video come collegamenti a file locali o di rete. La lavagna salva percorso, nome e dimensione e apre il file nel lettore predefinito di Windows. Il file non viene copiato nell'applicazione e non viene incluso nel backup: deve quindi restare disponibile nello stesso percorso.

### Backup

Il backup JSON versione 2.3 include immagini e cronologia delle lavagne. Le immagini sono codificate Base64 con dimensione e checksum SHA-256. Prima dell'importazione vengono verificati formato, proprietà del progetto, dimensioni, checksum e limiti complessivi. L'importazione continua a creare un backup preventivo automatico.

I collegamenti video vengono conservati nel documento della lavagna, mentre il contenuto del video rimane esterno al backup.

## Migrazione dalle note Markdown

La migrazione non deve cancellare `projects.note_text`.

1. Al primo accesso viene creata la lavagna del progetto.
2. Se esiste una nota Markdown, viene inserita come elemento di testo in una cornice denominata `Note importate`.
3. La lavagna viene salvata e riletta per verificarne l'integrità.
4. La nota originale resta disponibile in sola lettura e nel backup almeno per un intero ciclo di rilascio.
5. La rimozione definitiva del vecchio campo avviene solo in una migrazione successiva e reversibile.

## Fasi di sviluppo

1. Persistenza versionata, autosalvataggio e migrazione Markdown.
2. Canvas base: pan, zoom, selezione, testo, forme, undo/redo e scorciatoie.
3. Immagini, gestione asset e backup JSON 2.3. **Completata.**
4. Frecce collegate, snapping, gruppi, cornici e livelli. **Completata.**
5. Tabelle modificabili e navigazione da tastiera fra celle. **Completata.**
6. Collegamenti a video locali/rete con apertura nel lettore Windows. **Completata.**
7. Export PNG/PDF e recupero da immagini mancanti. **Completata.** Restano verifica accessibilità e test di carico.
8. Ribbon a schede, barra di stato, raccolta forme e schede contestuali. **Completata.**
9. Pannello oggetti/livelli, ricerca e minimappa. **Completata.**
10. Cestino e versioni persistenti automatiche/nominate. **Completata.**

## Criteri minimi di accettazione

- Una lavagna distinta e persistente per ogni progetto.
- Nessuna perdita delle note Markdown esistenti.
- Autosalvataggio resistente a chiusura e riapertura dell'app.
- Pan e zoom fluidi con almeno 1.000 elementi semplici.
- Immagini recuperabili dopo backup e ripristino; collegamenti video conservati e file video mantenuti esternamente.
- Undo/redo per tutte le operazioni modificabili.
- Tutte le operazioni principali disponibili con mouse e tastiera.
- Eliminazione del progetto comprensiva degli asset, effettuata in modo transazionale o recuperabile.

## Stima iniziale

Per una prima versione completa e collaudata: circa 55–80 ore.

- Base dati, API e migrazione: 8–12 ore.
- Canvas e interazioni desktop: 14–20 ore.
- Immagini, video e storage locale: 12–18 ore.
- Tabelle e frecce collegate: 10–16 ore.
- Backup, test, ottimizzazione e rifinitura: 11–14 ore.

Una versione MVP senza video e senza tabelle avanzate richiede indicativamente 28–40 ore.
