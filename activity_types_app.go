package main

import (
	"fmt"

	"work-time-tracker-go/tracker"
)

// ActivityTypeData rappresenta un tipo di attività
type ActivityTypeData struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	ColorVariant float64 `json:"color_variant"`
	Pattern      string  `json:"pattern"`
	DisplayOrder int     `json:"display_order"`
}

// GetActivityTypes restituisce tutti i tipi di attività
func (a *App) GetActivityTypes() ([]ActivityTypeData, error) {
	types, err := tracker.CaricaTipiAttivita(a.db)
	if err != nil {
		return nil, err
	}

	var result []ActivityTypeData
	for _, t := range types {
		result = append(result, ActivityTypeData{
			ID:           t.ID,
			Name:         t.Name,
			ColorVariant: t.ColorVariant,
			Pattern:      t.Pattern,
			DisplayOrder: t.DisplayOrder,
		})
	}
	return result, nil
}

// CreateActivityType crea un nuovo tipo di attività
func (a *App) CreateActivityType(name string, colorVariant float64, pattern string, displayOrder int) (int64, error) {
	if err := validateActivityType(name, colorVariant, pattern, displayOrder); err != nil {
		return 0, err
	}
	return tracker.CreaTipoAttivita(a.db, name, colorVariant, pattern, displayOrder)
}

// UpdateActivityType aggiorna un tipo di attività
func (a *App) UpdateActivityType(id int, name string, colorVariant float64, pattern string, displayOrder int) error {
	if id <= 0 {
		return fmt.Errorf("ID tipo attività non valido")
	}
	if err := validateActivityType(name, colorVariant, pattern, displayOrder); err != nil {
		return err
	}
	return tracker.AggiornaTipoAttivita(a.db, id, name, colorVariant, pattern, displayOrder)
}

// DeleteActivityType elimina un tipo di attività mai usato oppure, se presente nello
// storico, lo archivia. Restituisce true se è stato archiviato.
func (a *App) DeleteActivityType(id int) (bool, error) {
	if id <= 0 {
		return false, fmt.Errorf("ID tipo attività non valido")
	}
	return tracker.EliminaTipoAttivita(a.db, id)
}

// GetAllActivityTypes include i tipi archiviati: serve a colorare le sessioni storiche
func (a *App) GetAllActivityTypes() ([]ActivityTypeData, error) {
	types, err := tracker.CaricaTuttiTipiAttivita(a.db)
	if err != nil {
		return nil, err
	}
	result := make([]ActivityTypeData, 0, len(types))
	for _, t := range types {
		result = append(result, ActivityTypeData{
			ID: t.ID, Name: t.Name, ColorVariant: t.ColorVariant, Pattern: t.Pattern, DisplayOrder: t.DisplayOrder,
		})
	}
	return result, nil
}

// ReorderActivityTypes aggiorna l'ordine dei tipi di attività
func (a *App) ReorderActivityTypes(updates []map[string]int) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	seen := make(map[int]struct{}, len(updates))
	for _, u := range updates {
		id, idOK := u["id"]
		order, orderOK := u["display_order"]
		if !idOK || !orderOK || id <= 0 || order < 0 {
			return fmt.Errorf("riordinamento attività non valido")
		}
		if _, duplicate := seen[id]; duplicate {
			return fmt.Errorf("tipo attività duplicato nel riordinamento")
		}
		seen[id] = struct{}{}
		result, err := tx.Exec("UPDATE activity_types SET display_order = ? WHERE id = ?", order, id)
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return fmt.Errorf("tipo attività %d non trovato", id)
		}
	}
	return tx.Commit()
}
