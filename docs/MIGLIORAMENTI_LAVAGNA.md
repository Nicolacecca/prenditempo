# Miglioramenti futuri della lavagna

Questo documento conserva il pacchetto di modifiche richiesto e applicato nel ciclo di sviluppo di settembre 2026.

## Implementato nel ciclo corrente

### Rimozione del riepilogo delle dimensioni nel modificatore delle tabelle

Rimuovere la scritta riepilogativa, ad esempio `6 righe × 3 colonne`, visualizzata sotto la griglia nel pannello `Modifica tabella`. L'informazione è già disponibile nei campi `Righe` e `Colonne` presenti nella parte superiore e risulta quindi ridondante.

La rimozione dovrà interessare sia l'elemento visivo sia il codice che ne aggiorna il contenuto, senza modificare i campi numerici o i comandi per aggiungere e rimuovere righe e colonne. La modifica verrà applicata insieme ai prossimi miglioramenti indicati dall'utente.

### Etichette sulle frecce di collegamento

Ogni collegamento fra oggetti deve poter contenere un'etichetta testuale. L'interazione deve risultare familiare a chi utilizza PowerPoint e applicazioni desktop analoghe:

- doppio click sul collegamento, oppure digitazione dopo averlo selezionato, per inserire o modificare l'etichetta;
- testo posizionato al centro della linea e mantenuto leggibile sopra gli altri oggetti;
- etichetta che rimane associata alla freccia durante spostamenti, ridimensionamenti, cambio degli agganci, salvataggio e riapertura;
- possibilità di formattare carattere, dimensione, stile e colore con gli stessi comandi testuali della lavagna;
- area di testo leggibile anche sopra linee o oggetti sottostanti, dotata di sfondo di contrasto;
- modifica, svuotamento ed eliminazione dell'etichetta senza eliminare involontariamente il collegamento.

### Testo modificabile direttamente nelle forme geometriche

Le forme geometriche devono accettare testo interno con un comportamento simile a quello appreso in PowerPoint, senza richiedere la creazione e il raggruppamento manuale di una casella di testo separata:

- click sulla forma seguito dalla digitazione, oppure doppio click, per iniziare a scrivere;
- testo centrato inizialmente nella forma e modificabile direttamente sul posto;
- testo inizialmente centrato, con margini interni e ritorno a capo;
- formattazione tramite i normali comandi di carattere, dimensione, colore e stile;
- ridimensionamento della forma senza variazioni automatiche della dimensione del carattere;
- testo sempre associato alla forma durante spostamento, duplicazione, rotazione, ridimensionamento, salvataggio e riapertura;
- selezione ed eliminazione prevedibili: il primo click seleziona la forma, la modalità di modifica agisce sul testo e `Esc` ritorna alla selezione dell'oggetto.

Queste interazioni dovranno rispettare le convenzioni della suite Office già assunte come riferimento di usabilità, mantenendo però identità grafica e terminologia proprie del prodotto.

### Apertura contestuale delle palette colore

La palette dei colori deve aprirsi come pannello contestuale ancorato alla casella o al pulsante del colore che l'utente ha azionato. Non deve comparire al centro dello schermo come una finestra indipendente, perché ciò interrompe il rapporto visivo fra comando e risultato.

Il comportamento dovrà seguire le convenzioni delle applicazioni desktop della suite Office:

- apertura immediatamente sotto o accanto al campione colore interessato;
- riposizionamento automatico sopra o lateralmente quando manca spazio vicino ai bordi della finestra;
- indicazione chiara del colore corrente e dell'eventuale destinazione, come testo, linea o riempimento;
- chiusura dopo la selezione, con `Esc`, oppure facendo click fuori dal pannello;
- mantenimento della selezione dell'oggetto mentre la palette è aperta;
- nessuna copertura non necessaria dell'oggetto su cui si sta lavorando;
- comportamento coerente per colore del testo, contorno, collegamento, disegno e riempimento.

### Formattazione visiva delle singole celle della tabella

La tabella deve consentire di selezionare e formattare una singola cella oppure un intervallo di celle, senza applicare obbligatoriamente lo stesso stile all'intera tabella. L'esperienza deve richiamare l'uso essenziale e immediatamente riconoscibile di Excel.

Per ogni cella o selezione di celle devono essere disponibili almeno:

- colore di riempimento indipendente;
- colore del testo;
- carattere, dimensione, grassetto, corsivo e sottolineato;
- allineamento orizzontale a sinistra, al centro e a destra;
- allineamento verticale in alto, al centro e in basso;
- ritorno a capo e contenimento leggibile del testo nella cella.

La selezione deve essere evidente e i comandi devono agire soltanto sulle celle interessate. Le proprietà dovranno essere conservate nel salvataggio della lavagna e ripristinate correttamente alla riapertura.

### Conservazione dello stile quando cambia la struttura della tabella

Correggere il difetto per cui, dopo aver cambiato i colori o altri aspetti della tabella, la modifica del numero di righe o colonne ripristina lo stile predefinito.

Quando la tabella viene ridimensionata:

- le celle già esistenti devono conservare contenuto e formattazione;
- le nuove celle devono ricevere uno stile iniziale coerente, senza alterare quelle esistenti;
- l'eliminazione di righe o colonne non deve modificare lo stile delle celle rimanenti;
- i colori e gli stili dell'intestazione devono rimanere distinti, se impostati dall'utente;
- annullamento, ripetizione, salvataggio e riapertura devono preservare lo stesso risultato visivo.

Questa correzione ha priorità rispetto all'introduzione delle formule, perché riguarda l'affidabilità delle funzioni già disponibili.

## Sviluppi futuri

### Editor tabella compatto e tabella sempre completamente visibile

Nel prossimo aggiornamento, eliminare i quattro pulsanti `+ Riga`, `− Riga`, `+ Colonna` e `− Colonna`. I campi numerici `Righe` e `Colonne` dispongono già delle frecce per aumentare o diminuire i valori e i pulsanti costituiscono quindi una duplicazione dei comandi.

Aggiungere due controlli numerici compatti per permettere all'utente di stabilire:

- la larghezza delle celle;
- l'altezza delle celle.

Le dimensioni scelte devono essere applicate all'intera tabella, conservate nel salvataggio e mantenute dopo riapertura, aggiunta o rimozione di righe e colonne. In un'evoluzione successiva si potrà valutare anche la regolazione indipendente di singole colonne e righe, secondo un modello essenziale ispirato a Excel.

La tabella deve rimanere interamente visibile nella finestra di modifica senza richiedere l'uso delle barre di scorrimento. L'editor dovrà quindi:

- assegnare alla tabella la maggior parte dell'area disponibile;
- ridurre l'ingombro verticale e orizzontale dei comandi;
- disporre i controlli in una barra compatta;
- adattare automaticamente la scala di visualizzazione della tabella allo spazio disponibile;
- aggiornare l'adattamento quando cambiano righe, colonne, dimensioni delle celle o dimensioni della finestra;
- mantenere testo, bordi e selezione sufficientemente leggibili e utilizzabili con mouse e tastiera;
- evitare sia lo scorrimento verticale sia quello orizzontale nella normale modifica.

Principio generale di progettazione: il contenuto su cui l'utente sta lavorando deve sempre avere precedenza sui comandi nell'assegnazione dello spazio. Barre, pannelli e strumenti devono restare accessibili, ma non devono dominare o comprimere inutilmente il soggetto principale.

### Palette colore unica a due livelli

Ogni comando colore dell'applicazione, compresi quelli delle singole celle della tabella, deve seguire la stessa interazione a due livelli:

1. al primo click si apre una palette rapida di colori prestabiliti;
2. soltanto scegliendo `Altri colori…` si apre la tavolozza estesa per un colore personalizzato.

La palette rapida deve essere unica e coerente in tutta la lavagna: stessi colori, stesso ordine, stessa disposizione e stessa indicazione del colore attualmente selezionato per testo, riempimento, contorno, collegamenti, disegno e celle. Non deve aprirsi direttamente la tavolozza completa del sistema.

Il pannello deve restare contestuale e ancorato al relativo campione colore, indicando chiaramente quale proprietà verrà modificata. La scelta di un colore rapido deve applicare subito il risultato; `Altri colori…` rappresenta invece il secondo livello dell'interazione.

### Tabelle con calcoli essenziali in stile foglio elettronico

In un ciclo successivo, la tabella dovrà evolvere verso un utilizzo molto basilare simile a Excel, mantenendo però la semplicità propria della lavagna. Le celle dovranno poter eseguire almeno somma, sottrazione, moltiplicazione e divisione tramite riferimenti ad altre celle.

Prima dell'implementazione andranno definiti sintassi, riferimenti di cella, gestione degli errori, ricalcolo automatico e compatibilità con inserimento o eliminazione di righe e colonne. Questa parte resta esplicitamente fuori dal prossimo pacchetto di modifiche visive e di correzione.

## Implementato nei cicli precedenti

### Otto punti di aggancio per i collegamenti

Sostituire gli attuali sei punti di aggancio con otto punti disposti sul rettangolo di selezione di ogni oggetto:

- i quattro vertici;
- il punto di mezzeria del lato superiore;
- il punto di mezzeria del lato destro;
- il punto di mezzeria del lato inferiore;
- il punto di mezzeria del lato sinistro.

Durante lo spostamento dell'estremità di un collegamento, l'utente deve poter scegliere uno degli otto punti avvicinando il puntatore. L'estremità deve effettuare automaticamente lo snap al punto più vicino, mostrando chiaramente l'aggancio che verrà utilizzato prima del rilascio.

La scelta deve rimanere associata al collegamento dopo spostamento, ridimensionamento, rotazione, salvataggio e riapertura della lavagna. In fase di implementazione dovrà essere prevista anche la conversione automatica dei collegamenti già salvati con il precedente sistema a sei punti.

### Pulsanti fissi nel menu contestuale delle tabelle

Quando si aggiungono numerose righe a una tabella, lo scorrimento del contenuto non deve spostare fuori dalla finestra i pulsanti `Applica` e `Annulla`.

Il menu deve quindi essere organizzato in aree distinte:

- intestazione e comandi principali, se presenti;
- contenuto centrale scorrevole, comprendente le righe della tabella;
- barra inferiore fissa con i pulsanti `Applica` e `Annulla`, sempre visibili e utilizzabili.

La finestra deve inoltre rispettare l'altezza disponibile dello schermo, evitando che una parte del menu finisca oltre i bordi dell'applicazione. Il comportamento andrà verificato con tabelle molto lunghe e con finestre di dimensioni ridotte.

### Palette rapida e colore applicabile a tutti gli oggetti

Il menu di selezione del colore deve mostrare subito una raccolta di colori preimpostati, selezionabili con un solo click, senza obbligare l'utente a cercare ogni volta il colore nella tavolozza completa.

La palette rapida dovrà includere almeno:

- colori neutri adatti al testo, come nero, grigio scuro, grigio e bianco;
- colori principali, come rosso, arancione, giallo, verde, azzurro, blu e viola;
- alcune varianti chiare e scure coerenti con l'aspetto della lavagna.

Il colore attualmente applicato deve essere riconoscibile visivamente. La tavolozza completa deve rimanere disponibile tramite un comando dedicato, ad esempio `Altri colori…`, per consentire la scelta di qualsiasi colore personalizzato.

La scelta del colore non deve essere limitata alle scritte: deve funzionare in modo coerente su tutti gli elementi della lavagna, compresi:

- testi e note;
- forme geometriche;
- linee e disegni;
- frecce libere;
- collegamenti fra oggetti;
- cornici;
- tabelle e altri oggetti che espongono parti colorabili.

Il comando deve adattarsi all'oggetto selezionato: colore del testo per le scritte, colore della linea per linee, frecce e collegamenti, e proprietà appropriate per forme, cornici e oggetti composti. Per gli elementi che possiedono sia riempimento sia contorno, l'interfaccia dovrà permettere di distinguerli chiaramente. La palette dovrà funzionare anche sulle selezioni multiple compatibili.

### Comandi essenziali per il testo

Il gruppo dedicato alla formattazione del testo deve contenere esclusivamente:

1. colore del testo;
2. stile normale, corsivo, grassetto e sottolineato;
3. pedice e apice;
4. scelta del carattere, se utile e compatibile con la semplicità dell'interfaccia;
5. dimensione del carattere.

I comandi devono agire sul testo selezionato durante la modifica oppure sull'intero oggetto di testo quando non è selezionato un intervallo. Lo stile `Normale` deve rimuovere corsivo, grassetto, sottolineato, pedice e apice dalla selezione interessata.

### Significato e applicazione dello spessore linea

Il valore numerico associato al comando `Linea` deve indicare esclusivamente lo spessore del tratto o del contorno. Deve essere applicabile in modo coerente a tutti gli elementi pertinenti, tra cui:

- linee e disegni;
- frecce libere;
- collegamenti fra oggetti;
- contorni di rettangoli, cerchi e altre forme;
- bordi delle cornici;
- altri oggetti dotati di linea o contorno modificabile.

La dimensione del carattere e lo spessore della linea devono restare due impostazioni separate e non influenzarsi reciprocamente. I comandi non applicabili all'oggetto selezionato dovranno risultare disabilitati o nascosti, per evitare ambiguità.

### Colore di sfondo e trasparenza degli oggetti

Per forme, cornici e altri oggetti dotati di un'area interna, l'utente deve poter modificare anche il colore dello sfondo o riempimento. Questa impostazione deve essere distinta dal colore del testo e dal colore del contorno.

Il riempimento deve offrire:

- la stessa palette di colori preimpostati prevista per gli altri comandi;
- l'accesso alla tavolozza completa tramite `Altri colori…`;
- un controllo della trasparenza indipendente dal colore e dal contorno;
- la possibilità di rendere lo sfondo completamente trasparente;
- un'anteprima chiara del risultato prima o durante l'applicazione.

La trasparenza deve interessare soltanto il riempimento: testo, contorno e altri componenti dell'oggetto devono conservare la propria opacità. Il controllo deve funzionare anche sulle selezioni multiple compatibili.

### Correzione dell'allineamento assiale

L'allineamento fra oggetti funziona, ma deve essere corretto il comportamento dell'allineamento assiale. Il calcolo deve utilizzare i punti di mezzeria, cioè i centri geometrici dei rettangoli di ingombro degli oggetti selezionati.

- Allineamento lungo X: tutti i punti di mezzeria devono assumere la stessa coordinata X, disponendo gli oggetti lungo un asse verticale.
- Allineamento lungo Y: tutti i punti di mezzeria devono assumere la stessa coordinata Y, disponendo gli oggetti lungo un asse orizzontale.

L'operazione deve mantenere invariata la posizione sull'altro asse e funzionare con oggetti di dimensioni, tipi e rotazioni differenti. Il riferimento da utilizzare dovrà essere coerente e prevedibile, senza modificare le distanze sull'asse non interessato.

### Ridimensionamento indipendente delle Note

Il foglietto della Nota conserva larghezza e altezza indipendenti dalla dimensione del carattere. Il ridimensionamento modifica soltanto lo spazio disponibile e la disposizione delle righe: non ingrandisce o rimpicciolisce il testo. Le dimensioni restano stabili anche dopo la modifica del contenuto, il salvataggio e la riapertura.

### Elenchi e checklist

La formattazione del testo comprende elenchi puntati, elenchi numerati e checklist. I comandi agiscono sulle righe selezionate durante la modifica oppure sull'intero testo quando l'oggetto non è in modifica. Le checklist usano `☐` per le attività da fare e `☑` per quelle completate; il comando `Eseguito` alterna i due stati senza eliminare il testo dell'attività.

## Verifiche eseguite

- Test automatici del frontend.
- Verifica grafica della palette e della formattazione.
- Verifica degli otto agganci e dello snap delle estremità.
- Verifica dell'allineamento assiale tramite i centri geometrici.
- Verifica del modificatore delle tabelle con 27 righe e finestra ridotta.
- Verifica del ridimensionamento di una Nota senza variazioni del carattere.
- Verifica dell'inserimento di checklist e del passaggio da attività aperta a completata.
