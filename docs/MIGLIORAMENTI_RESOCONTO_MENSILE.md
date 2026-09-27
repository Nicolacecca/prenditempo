# Miglioramenti futuri del resoconto mensile

Questo documento conserva le modifiche al calcolo e alla presentazione del resoconto mensile applicate nel ciclo di sviluppo di settembre 2026.

## Implementato

### Trattamento di ferie, permessi e festività

Il resoconto mensile deve distinguere chiaramente ferie, permessi e festività.

#### Ferie

- La registrazione di ferie non deve modificare il totale delle ore contrattuali previste nel mese.
- Il resoconto deve mostrare separatamente le ferie godute nel mese.
- Le ferie devono essere riepilogate in giorni e, quando l'inserimento o la configurazione lo consente, anche nelle ore corrispondenti.

#### Permessi

- La registrazione di un permesso non deve modificare il totale delle ore contrattuali previste nel mese.
- Il resoconto deve mostrare separatamente i permessi goduti nel mese.
- I permessi devono essere riepilogati in ore e, per quelli registrati a giornata intera, anche in giorni.

#### Festività

- Una festività deve diminuire le ore contrattuali richieste nel mese.
- La riduzione deve corrispondere alle ore lavorative contrattualmente previste per il giorno interessato.
- Una festività in un giorno senza ore contrattuali previste non deve produrre una riduzione aggiuntiva.
- Il resoconto deve mostrare separatamente le festività considerate e le ore contrattuali sottratte.

### Controlli richiesti

- Evitare che la stessa giornata o lo stesso intervallo venga conteggiato due volte in presenza di registrazioni sovrapposte.
- Mantenere distinti i totali di ore lavorate, ore contrattuali, ferie, permessi e festività.
- Verificare il calcolo con orari giornalieri differenti, lavoro part-time, mesi incompleti e festività che cadono in giorni non lavorativi.
- Assicurare coerenza fra resoconto visualizzato ed eventuali esportazioni.

## Verifiche eseguite

- Ferie e permessi non riducono le ore contrattuali mensili.
- Le festività riducono soltanto le ore previste nei giorni lavorativi.
- Una festività nel fine settimana non produce riduzioni.
- Il resoconto e il CSV mostrano separatamente giorni e ore di ferie, permessi e festività.
