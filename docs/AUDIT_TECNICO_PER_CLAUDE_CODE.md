# PrendiTempo — audit tecnico e piano di correzione per Claude Code

## Scopo del documento

Questo documento descrive le principali perplessità emerse da un audit del codice di PrendiTempo e le traduce in interventi verificabili. L'obiettivo non è una riscrittura generale: prima bisogna rendere affidabili il modello delle ore, l'integrità dello storico e il recupero delle sessioni; solo dopo ha senso rifattorizzare o aggiungere funzionalità.

Claude Code deve:

1. ispezionare nuovamente i percorsi indicati e confermare ogni problema prima di modificarli;
2. implementare le correzioni in ordine di priorità;
3. mantenere compatibilità con i database e i backup già esistenti;
4. aggiungere test di regressione per ogni correzione;
5. evitare modifiche estetiche o funzionalità non necessarie al lavoro descritto;
6. documentare eventuali regole aziendali ancora ambigue invece di inventarle.

## Stato verificato

Al momento dell'audit:

- `go test ./...` passa;
- `go vet ./...` passa;
- `npm test` passa con 8 test;
- `npm run build` passa;
- copertura Go rilevata: circa 29% nel package principale e 34% nel package `tracker`.

Il progetto ha buone fondamenta: SQLite con foreign key e WAL, transazioni in diversi flussi delicati, validazione backend, backup preventivo prima dell'importazione, recupero del tracking pendente e gestione corretta di alcuni confini temporali. Le correzioni seguenti devono preservare questi aspetti.

---

## Modello contabile vincolante

Il report deve mantenere separate almeno queste quantità:

1. **Ore contrattuali previste**: monte ore teorico del periodo.
2. **Ore realmente tracciate/lavorate**: durata delle sessioni valide registrate.
3. **Ore ordinarie lavorate**: parte del tracciato che ricade nel lavoro ordinario secondo la regola configurata.
4. **Straordinario**: parte del lavoro tracciato classificata come straordinario da una regola esplicita.
5. **Ore coperte da assenza**: ferie, permessi e malattia, mostrate separatamente dal lavoro effettivo.
6. **Ore mancanti**: quota contrattuale non coperta né dal lavoro ordinario né da un'assenza valida.
7. **Tempo non attribuito o anomalo**: sessioni senza progetto, sovrapposizioni, timer aperti o altri dati che richiedono attenzione.

Regole già richieste dal prodotto:

- ferie, permessi e malattia non devono ridurre il monte contrattuale mostrato;
- una festività in un giorno lavorativo riduce il monte contrattuale del periodo;
- lo straordinario deve restare separato dalle ore ordinarie e dalle assenze;
- totali giornalieri, mensili, per progetto, per attività e CSV devono derivare dagli stessi record e riconciliarsi;
- gli intervalli devono essere trattati come `[inizio, fine)`, così due sessioni adiacenti non si sovrappongono;
- non inventare regole su soglie, arrotondamenti o lavoro durante ferie/malattia: se non definite, isolarle in una policy documentata e segnalare l'ambiguità.

---

## P1 — Il report mensile presenta ore contrattuali come ore lavorate

### Evidenza

- `tracker/monthly_report.go`, funzione `GetMonthlyReport`, circa righe 421–449.
- `frontend/src/modules/monthly.js`, funzione `renderMonthlyReport`, circa righe 266–310.
- `monthly_app.go`, funzione `SaveMonthlyReportCSV`.

Per i normali giorni lavorativi il calcolo attuale usa sostanzialmente:

```text
effectiveWorked = contractedHoursPerDay + outsideHours
```

Di conseguenza, con un contratto di 8 ore, una giornata con una sola ora tracciata può essere mostrata come 8 ore lavorate. Anche il CSV usa questa quantità sotto un'intestazione assimilabile a "ore lavorate".

### Conseguenza

Il report può materialmente sovrastimare il lavoro svolto e nascondere ore mancanti. Non è quindi sicuro per rendicontazione a un responsabile, fatturazione, presenze o calcolo dello straordinario.

### Correzione richiesta

- Eliminare l'uso delle ore contrattuali come valore sostitutivo delle ore lavorate.
- Introdurre campi espliciti nel report giornaliero e mensile, con nomi non ambigui. Indicativamente:
  - `expected_hours`;
  - `tracked_hours`;
  - `ordinary_worked_hours`;
  - `overtime_hours`;
  - `absence_covered_hours`;
  - `missing_hours`;
  - `unallocated_hours`;
  - eventuali `anomalies`.
- Valutare la rimozione o la rinomina di `EffectiveWorkedHours`, `TotalEffectiveWorkedHours` e `UsesDefault`: non devono continuare a suggerire che ore non tracciate siano state lavorate.
- Aggiornare insieme backend, UI e CSV.
- Conservare, se utile, un indicatore visivo della quota contrattuale non tracciata, ma chiamarlo esplicitamente “mancante/non tracciata”.
- Fare in modo che il totale dei progetti, incluso “Nessun progetto”, coincida con il totale tracciato.

### Matrice minima di accettazione

Assumere: lunedì–venerdì, 8 ore contrattuali, fascia 08:00–17:00, pausa 13:00–14:00.

| Scenario | Contratto | Tracciato | Assenza coperta | Mancanti | Straordinario |
|---|---:|---:|---:|---:|---:|
| Giorno normale, nessuna sessione | 8 | 0 | 0 | 8 | 0 |
| Giorno normale, 1h dentro fascia | 8 | 1 | 0 | 7 | 0 |
| Giorno normale, 8h dentro fascia | 8 | 8 | 0 | 0 | 0 |
| 8h ordinarie + 1h fuori fascia | 8 | 9 | 0 | 0 | 1 |
| Sabato, 2h tracciate | 0 | 2 | 0 | 0 | 2 |
| Festività infrasettimanale, nessun lavoro | 0 | 0 | 0 | 0 | 0 |
| Festività infrasettimanale, 2h lavorate | 0 | 2 | 0 | 0 | 2 |
| Ferie in giorno lavorativo | 8 | 0 | 8 | 0 | 0 |
| Permesso 2h + lavoro ordinario 6h | 8 | 6 | 2 | 0 | 0 |

Per lavoro registrato durante ferie o malattia, definire una policy esplicita e un test: non nascondere mai quelle ore dentro il valore contrattuale predefinito.

### Test richiesti

- Un test per ogni riga della matrice.
- Sessione che attraversa mezzanotte.
- Sessione che attraversa fine mese e fine anno.
- Festività nel weekend, che non deve ridurre due volte il contratto.
- Confronto tra valori del report e righe CSV.
- Verifica che somma progetti = totale tracciato.

---

## P1 — Le sessioni sovrapposte possono gonfiare tutti i totali

### Evidenza

- `wails_app.go`: `CreateSession`, `UpdateSessionDuration`, `UpdateSessionComplete`, `SplitSession`.
- `tracker/database.go`: `CreaSessione`, `AggiornaDurataSessione`, `AggiornaSessioneCompleta`.

Le API validano durata, timestamp, progetto e attività, ma non cercano altre sessioni che si sovrappongono all'intervallo. Il database memorizza `timestamp + seconds` senza un controllo applicativo condiviso sulle collisioni.

### Conseguenza

Due sessioni 09:00–11:00 e 10:00–12:00 producono quattro ore in tre ore reali. I totali giornalieri, mensili e per progetto diventano falsi.

### Correzione richiesta

- Creare una funzione backend unica per verificare conflitti usando intervalli `[inizio, fine)`.
- Usarla in creazione, modifica completa, modifica durata, split, import di progetto e import del backup.
- Durante un update, escludere dal controllo la sessione modificata.
- Le sessioni adiacenti, ad esempio 09:00–10:00 e 10:00–11:00, devono essere valide.
- Il tracking in corso deve essere considerato: non consentire inserimenti manuali che si sovrappongano silenziosamente alla sessione aperta.
- Per backup esistenti che contengono sovrapposizioni, non distruggere dati: interrompere l'import con un errore chiaro oppure importare evidenziando anomalie, ma non sommare in silenzio.
- Aggiungere un indice utile alla query se necessario, verificando il piano SQLite.

### Test richiesti

- sovrapposizione completa, parziale e contenuta;
- intervalli adiacenti consentiti;
- aggiornamento che crea una sovrapposizione;
- split valido senza falso positivo con la sessione originale;
- sovrapposizione oltre mezzanotte;
- import contenente intervalli sovrapposti;
- concorrenza o doppio invio della stessa creazione.

---

## P1 — La cancellazione o rinomina dei tipi di attività compromette lo storico

### Evidenza

- `tracker/database.go`, `AggiornaTipoAttivita`, circa riga 1190.
- `tracker/database.go`, `EliminaTipoAttivita`, circa righe 1211–1235.
- `sessions.activity_type` è testo, non una foreign key verso `activity_types`.

La cancellazione imposta a `NULL` l'attività di tutte le sessioni storiche. Inoltre l'UPDATE delle sessioni e il DELETE del tipo non sono nella stessa transazione. La rinomina modifica il catalogo ma non aggiorna i nomi già memorizzati nelle sessioni.

### Conseguenza

I report passati possono cambiare o perdere classificazioni. Una rinomina può creare categorie storiche non più selezionabili. Questo rende difficile spiegare a un responsabile come il tempo sia stato utilizzato.

### Correzione richiesta

Soluzione minima sicura:

- aggiungere lo stato `archived` ai tipi di attività;
- se un tipo è già usato, archiviarlo invece di cancellarlo;
- conservarlo nei report storici ma non proporlo per nuove sessioni;
- racchiudere qualsiasi aggiornamento multi-tabella in una transazione;
- impedire che una cancellazione fallita lasci sessioni già azzerate.

Soluzione strutturale preferibile, se implementabile senza rischiare la compatibilità:

- introdurre `activity_type_id` nelle sessioni con migrazione versionata;
- mantenere temporaneamente il campo testuale per compatibilità/import;
- stabilire esplicitamente se una rinomina debba aggiornare anche l'etichetta storica oppure se serva uno snapshot del nome;
- usare `ON DELETE RESTRICT` o archiviazione, mai cancellazione distruttiva dello storico.

### Test richiesti

- archiviazione di un tipo già usato;
- report storico invariato dopo archiviazione;
- tipo archiviato non disponibile per nuove sessioni;
- rinomina con comportamento storico documentato;
- rollback completo in caso di errore durante la modifica.

---

## P2 — Un crash può perdere fino a cinque minuti di tracking

### Evidenza

- `wails_app.go`, `StartTracking`: callback di salvataggio impostata a 300 secondi.
- `tracker/database.go`, `RecoverPendingTracking`: recupero basato su `LastSavedSeconds`.

### Conseguenza

In caso di chiusura anomala, il recupero può troncare la sessione all'ultimo salvataggio e perdere quasi cinque minuti. Il problema è piccolo per uso occasionale, ma mina la fiducia in un'app di contabilizzazione del tempo.

### Correzione richiesta

- Ridurre il checkpoint a 15–30 secondi, misurando che non produca problemi apprezzabili su SQLite.
- In alternativa o in aggiunta, al riavvio presentare una sessione “da verificare” con:
  - ora iniziale;
  - ultimo checkpoint;
  - ora dell'ultimo aggiornamento;
  - proposta di durata;
  - possibilità di correggere, confermare o scartare.
- Non estendere automaticamente fino all'ora di riavvio: il computer potrebbe essere rimasto spento.
- Rendere il recupero di ogni record atomico: aggiornamento/eliminazione della sessione e rimozione del pending devono avvenire nella stessa transazione.
- Se `StopTracking` non riesce a finalizzare, mantenere uno stato recuperabile e mostrare un errore all'utente; non far apparire lo stop come concluso con successo.

### Test richiesti

- crash prima del primo checkpoint;
- crash dopo uno o più checkpoint;
- errore durante la finalizzazione;
- recupero ripetuto/idempotente;
- più record pendenti provenienti da un database corrotto o da una vecchia versione.

---

## P2 — Migrazioni e schema devono essere versionati

### Evidenza

`tracker/database.go::InitDB` esegue diversi `ALTER TABLE` e ricrea trigger a ogni avvio, intercettando gli errori di colonna già esistente.

### Rischio

Con l'evoluzione dello schema diventa difficile sapere quali passaggi siano già avvenuti, testare gli upgrade e garantire rollback o diagnosi comprensibili.

### Correzione richiesta

- Usare `PRAGMA user_version` o una tabella `schema_migrations`.
- Applicare ogni migrazione una sola volta e dentro una transazione dove SQLite lo consente.
- Testare almeno:
  - database vuoto;
  - database della versione attuale;
  - fixture rappresentativa di una versione precedente;
  - riapertura dopo migrazione già completata.
- Creare automaticamente un backup prima di migrazioni distruttive o non banali.
- Non eliminare immediatamente la compatibilità con i backup versioni 1.0–2.3.

---

## P2 — Date, fusi orari e ora legale

### Evidenza

I timestamp sono salvati come stringhe locali senza offset. `parseLocalTimestamp` accetta anche suffissi come `Z` o `+02:00`, ma li tronca e interpreta l'orario risultante come locale invece di convertirlo.

### Rischio

Un timestamp importato con offset può cambiare significato. Le giornate del passaggio tra ora solare e legale possono essere difficili da ricostruire in modo univoco.

### Correzione richiesta

- Definire e documentare una convenzione unica di persistenza.
- Opzioni accettabili:
  - istanti UTC più timezone applicativa; oppure
  - timestamp locali esplicitamente senza offset, rifiutando input con offset che verrebbero interpretati male.
- Non troncare silenziosamente un offset.
- Garantire compatibilità con i dati esistenti tramite migrazione o parser legacy isolato.

### Test richiesti

- input locale senza offset;
- input UTC `Z`;
- input con `+01:00` e `+02:00`;
- passaggio CET/CEST;
- sessione oltre mezzanotte e fine anno.

---

## P2 — Copertura test e struttura del codice

Questa attività va eseguita dopo le correzioni contabili, non prima.

Problemi di manutenibilità osservati:

- `tracker/database.go` concentra schema, migrazioni, CRUD, report e recovery;
- `wails_app.go` contiene molte API non correlate;
- `frontend/index.html` è molto esteso e contiene molto stile inline;
- il timer dipende da stato globale condiviso;
- alcune regole sono duplicate tra Go e JavaScript.

### Direzione raccomandata

- Estrarre moduli piccoli per `sessions`, `activities`, `reports`, `migrations`, `backup` e `tracking`.
- Rendere il calcolo del report una funzione pura o quasi pura, testabile con input tabellari.
- Incapsulare lo stato del timer in una struttura posseduta da `App`, evitando nuove variabili globali.
- Mantenere la validazione autorevole nel backend; il frontend può duplicarla solo per feedback immediato.
- Aumentare la copertura soprattutto sui calcoli e sulle transazioni, non inseguire una percentuale astratta.

---

## Requisiti trasversali per ogni intervento

### Compatibilità dati

- Non perdere progetti, sessioni, note, eventi, impostazioni o URL delle lavagne.
- Se cambia il formato del backup, incrementare la versione e continuare a importare le versioni precedenti supportate.
- Eseguire import e migrazioni in modo atomico.
- Conservare un backup recuperabile prima di operazioni distruttive.

### Interfaccia

- Usare terminologia non ambigua: “tracciate”, “contrattuali”, “coperte”, “mancanti”, “straordinario”.
- Segnalare sovrapposizioni, sessioni aperte e dati da verificare.
- Non presentare un valore stimato o predefinito come lavoro realmente svolto.
- Il CSV deve essere comprensibile senza conoscere l'implementazione interna.

### Sicurezza e privacy

- Continuare a mantenere i dati locali per impostazione predefinita.
- Non inserire telemetria o servizi cloud durante queste correzioni.
- Conservare la neutralizzazione delle formule CSV.
- Non scrivere nei log contenuti sensibili delle note o informazioni non necessarie.

---

## Ordine di implementazione raccomandato

1. **Aggiungere test che riproducano il report mensile errato.**
2. **Correggere il modello del report** e allineare UI e CSV.
3. **Aggiungere rilevamento delle sovrapposizioni** a tutti i percorsi di scrittura/import.
4. **Proteggere lo storico dei tipi di attività** con archiviazione e transazioni.
5. **Rendere più affidabile il recovery** e ridurre la finestra di perdita.
6. **Versionare le migrazioni** e coprire gli upgrade con fixture.
7. **Definire la convenzione temporale** e aggiungere test su offset/DST.
8. **Rifattorizzare i file grandi**, mantenendo invariato il comportamento già corretto.

Eseguire test e build dopo ogni gruppo, non soltanto alla fine.

## Comandi finali di verifica

```powershell
go test ./...
go vet ./...
Set-Location frontend
npm test
npm run build
```

Se l'ambiente impedisce l'uso della cache Go predefinita, usare una cache interna al repository senza modificare la configurazione permanente dell'utente.

## Definition of Done

Il lavoro è completo soltanto quando:

- nessuna ora non tracciata è presentata come lavorata;
- le sovrapposizioni sono prevenute o esplicitamente segnalate in ogni percorso di inserimento;
- eliminare o rinominare un'attività non distrugge lo storico;
- il recovery non perde silenziosamente fino a cinque minuti senza possibilità di verifica;
- report UI, CSV, progetti e attività riconciliano sugli stessi dati;
- i database esistenti e i backup supportati continuano a essere leggibili;
- tutti i nuovi scenari hanno test di regressione;
- `go test ./...`, `go vet ./...`, `npm test` e `npm run build` terminano con successo.

## Indicazione di prodotto

Fino al completamento almeno dei punti P1, PrendiTempo deve essere descritto come diario personale del lavoro e strumento di analisi locale, non come sistema affidabile per paghe, presenze, fatturazione o rendicontazione ufficiale.
