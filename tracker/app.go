package tracker

import "fmt"

// Project rappresenta un progetto su cui si lavora
type Project struct {
	ID          int
	Name        string
	Description string
	CreatedAt   string
	Archived    bool
	ClosedAt    string
	NoteText    string
}

// StampaInfo stampa le informazioni del progetto
func (p Project) StampaInfo() {
	fmt.Printf("[PROJECT] %s - %s\n", p.Name, p.Description)
}
