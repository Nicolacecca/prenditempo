package tracker

import "fmt"

// CalcolaTotale calcola il tempo totale di una lista di app
func CalcolaTotale(apps []Applicazione) int {
	totale := 0
	for _, app := range apps {
		totale += app.Tempo
	}
	return totale
}

// TrovaAppPiuUsata trova l'app con più tempo
func TrovaAppPiuUsata(apps []Applicazione) (Applicazione, error) {
	if len(apps) == 0 {
		return Applicazione{}, fmt.Errorf("nessuna applicazione disponibile")
	}

	max := apps[0]
	for _, app := range apps {
		if app.Tempo > max.Tempo {
			max = app
		}
	}
	return max, nil
}

// StampaStatistiche stampa statistiche generali
func StampaStatistiche(apps []Applicazione) {
	fmt.Println("=== STATISTICHE ===")
	fmt.Printf("Totale app: %d\n", len(apps))

	totale := CalcolaTotale(apps)
	fmt.Printf("Tempo totale: %d minuti\n", totale)

	if appMax, err := TrovaAppPiuUsata(apps); err == nil {
		fmt.Printf("App piu usata: %s (%d minuti)\n", appMax.Nome, appMax.Tempo)
	}
}
