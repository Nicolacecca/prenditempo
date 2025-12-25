package tracker

import "fmt"

// Applicazione rappresenta un'app tracciata
type Applicazione struct {
	Nome      string
	Tempo     int
	Categoria string
}

// StampaInfo stampa le informazioni dell'app
func (a Applicazione) StampaInfo() {
	fmt.Printf("[APP] %s (%s) - %d minuti\n", a.Nome, a.Categoria, a.Tempo)
}

// AggiungiTempo aggiunge minuti all'app
func (a *Applicazione) AggiungiTempo(minuti int) error {
	if minuti < 0 {
		return fmt.Errorf("tempo negativo non permesso: %d", minuti)
	}
	a.Tempo += minuti
	return nil
}

// Project rappresenta un progetto su cui si lavora
type Project struct {
	ID          int
	Name        string
	Description string
	CreatedAt   string
	Archived    bool
	ClosedAt    string
}

// StampaInfo stampa le informazioni del progetto
func (p Project) StampaInfo() {
	fmt.Printf("[PROJECT] %s - %s\n", p.Name, p.Description)
}
