package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"work-time-tracker-go/tracker"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// === RESOCONTO MENSILE ===

// MonthlyReportData rappresenta il resoconto mensile esposto al frontend
type MonthlyReportData struct {
	Year                      int                    `json:"year"`
	Month                     int                    `json:"month"`
	MonthName                 string                 `json:"month_name"`
	ContractedHoursPerDay     float64                `json:"contracted_hours_per_day"`
	WorkingDaysInMonth        int                    `json:"working_days_in_month"`
	TotalContractedHours      float64                `json:"total_contracted_hours"`
	EventsCoveredHours        float64                `json:"events_covered_hours"`
	EffectiveContractedHours  float64                `json:"effective_contracted_hours"`
	HolidayDays               int                    `json:"holiday_days"`
	HolidayHours              float64                `json:"holiday_hours"`
	VacationDays              int                    `json:"vacation_days"`
	VacationHours             float64                `json:"vacation_hours"`
	SickDays                  int                    `json:"sick_days"`
	SickHours                 float64                `json:"sick_hours"`
	TrasfertaDays             int                    `json:"trasferta_days"`
	PermissionDays            float64                `json:"permission_days"`
	PermissionHours           float64                `json:"permission_hours"`
	TotalTrackedHours         float64                `json:"total_tracked_hours"`
	TotalEffectiveWorkedHours float64                `json:"total_effective_worked_hours"`
	OvertimeHours             float64                `json:"overtime_hours"`
	Days                      []tracker.DailyStats   `json:"days"`
	Projects                  []tracker.ProjectStats `json:"projects"`
}

// GetWorkingDaysBitmask restituisce la bitmask dei giorni lavorativi
func (a *App) GetWorkingDaysBitmask() (int, error) {
	val, err := tracker.GetSetting(a.db, "working_days_bitmask")
	if err != nil || val == "" {
		return tracker.DefaultWorkingDaysBitmask, nil
	}
	var bitmask int
	if _, err := fmt.Sscanf(val, "%d", &bitmask); err != nil {
		return tracker.DefaultWorkingDaysBitmask, nil
	}
	return bitmask, nil
}

// SetWorkingDaysBitmask imposta la bitmask dei giorni lavorativi
func (a *App) SetWorkingDaysBitmask(bitmask int) error {
	if bitmask < 0 || bitmask > 127 {
		return fmt.Errorf("bitmask non valida")
	}
	return tracker.SetSetting(a.db, "working_days_bitmask", fmt.Sprintf("%d", bitmask))
}

// GetPatronSaintDate restituisce la data della festa patronale configurata (formato "MM-GG", vuota se non impostata)
func (a *App) GetPatronSaintDate() (string, error) {
	return tracker.GetSetting(a.db, "patron_saint_date")
}

// SetPatronSaintDate imposta la data della festa patronale locale (formato "MM-GG"); stringa vuota per rimuoverla
func (a *App) SetPatronSaintDate(monthDay string) error {
	if monthDay != "" {
		if _, err := time.Parse("01-02", monthDay); err != nil {
			return fmt.Errorf("data non valida, usa il formato MM-GG")
		}
	}
	return tracker.SetSetting(a.db, "patron_saint_date", monthDay)
}

// GetTrasfertaLabel restituisce il nome personalizzato della categoria "trasferta" (default "Trasferta")
func (a *App) GetTrasfertaLabel() (string, error) {
	val, err := tracker.GetSetting(a.db, "trasferta_label")
	if err != nil || val == "" {
		return "Trasferta", nil
	}
	return val, nil
}

// SetTrasfertaLabel imposta il nome personalizzato della categoria "trasferta"
func (a *App) SetTrasfertaLabel(label string) error {
	label = strings.TrimSpace(label)
	if label == "" || len([]rune(label)) > 40 {
		return fmt.Errorf("il nome deve essere compreso tra 1 e 40 caratteri")
	}
	return tracker.SetSetting(a.db, "trasferta_label", label)
}

// GetYearHolidays restituisce le date marcate come festività nell'anno indicato
func (a *App) GetYearHolidays(year int) ([]string, error) {
	if year < 1900 || year > 2200 {
		return nil, fmt.Errorf("anno non valido")
	}
	return tracker.GetYearHolidays(a.db, year)
}

// RecurringHoliday è una festività personalizzata che ricorre ogni anno nello
// stesso giorno/mese (es. una ricorrenza aziendale o locale diversa dalla
// festa patronale).
type RecurringHoliday struct {
	Month int    `json:"month"`
	Day   int    `json:"day"`
	Label string `json:"label"`
}

// GetRecurringHolidays restituisce le festività ricorrenti personalizzate configurate
func (a *App) GetRecurringHolidays() ([]RecurringHoliday, error) {
	val, err := tracker.GetSetting(a.db, "recurring_holidays")
	if err != nil || val == "" {
		return []RecurringHoliday{}, nil
	}
	var list []RecurringHoliday
	if err := json.Unmarshal([]byte(val), &list); err != nil {
		return []RecurringHoliday{}, nil
	}
	return list, nil
}

// daysInMonth ritorna il numero di giorni del mese indicato (usa un anno bisestile
// di riferimento per ammettere il 29 febbraio come giorno ricorrente valido)
func daysInMonth(month int) int {
	return time.Date(2000, time.Month(month+1), 0, 0, 0, 0, 0, time.Local).Day()
}

// AddRecurringHoliday aggiunge una festività personalizzata ricorrente ogni anno
func (a *App) AddRecurringHoliday(month, day int, label string) error {
	if month < 1 || month > 12 {
		return fmt.Errorf("mese non valido")
	}
	if day < 1 || day > daysInMonth(month) {
		return fmt.Errorf("giorno non valido per il mese scelto")
	}
	label = strings.TrimSpace(label)
	if len([]rune(label)) > 60 {
		return fmt.Errorf("il nome non può superare 60 caratteri")
	}

	list, err := a.GetRecurringHolidays()
	if err != nil {
		return err
	}
	for _, h := range list {
		if h.Month == month && h.Day == day {
			return fmt.Errorf("esiste già una festività ricorrente in questa data")
		}
	}
	list = append(list, RecurringHoliday{Month: month, Day: day, Label: label})

	data, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return tracker.SetSetting(a.db, "recurring_holidays", string(data))
}

// RemoveRecurringHoliday rimuove la festività ricorrente in quel giorno/mese
func (a *App) RemoveRecurringHoliday(month, day int) error {
	list, err := a.GetRecurringHolidays()
	if err != nil {
		return err
	}
	filtered := make([]RecurringHoliday, 0, len(list))
	for _, h := range list {
		if h.Month == month && h.Day == day {
			continue
		}
		filtered = append(filtered, h)
	}
	data, err := json.Marshal(filtered)
	if err != nil {
		return err
	}
	return tracker.SetSetting(a.db, "recurring_holidays", string(data))
}

// PopulateItalianHolidays inserisce nel calendario le festività nazionali italiane
// dell'anno indicato (incluse Pasqua/Pasquetta, l'eventuale festa patronale e le
// festività ricorrenti personalizzate configurate), senza sovrascrivere giorni
// già personalizzati dall'utente. Ritorna quante date sono state effettivamente aggiunte.
func (a *App) PopulateItalianHolidays(year int) (int, error) {
	if year < 1900 || year > 2200 {
		return 0, fmt.Errorf("anno non valido")
	}
	dates := make([]string, 0, len(italianNationalHolidays(year))+1)
	for _, h := range italianNationalHolidays(year) {
		dates = append(dates, h.Date.Format("2006-01-02"))
	}
	if patronMD, err := a.GetPatronSaintDate(); err == nil && patronMD != "" {
		if pd, err := time.Parse("01-02", patronMD); err == nil {
			dates = append(dates, time.Date(year, pd.Month(), pd.Day(), 0, 0, 0, 0, time.Local).Format("2006-01-02"))
		}
	}
	if recurring, err := a.GetRecurringHolidays(); err == nil {
		for _, h := range recurring {
			dates = append(dates, time.Date(year, time.Month(h.Month), h.Day, 0, 0, 0, 0, time.Local).Format("2006-01-02"))
		}
	}
	return tracker.PopulateHolidays(a.db, dates)
}

// GetMonthlyReport genera il resoconto mensile per anno e mese specificati
func (a *App) GetMonthlyReport(year, month int) (*MonthlyReportData, error) {
	if err := validateYearMonth(year, month); err != nil {
		return nil, err
	}
	contractedHours, err := a.GetContractedHoursPerDay()
	if err != nil {
		contractedHours = 8.0
	}

	workingDaysBitmask, err := a.GetWorkingDaysBitmask()
	if err != nil {
		workingDaysBitmask = tracker.DefaultWorkingDaysBitmask
	}

	sched := a.GetWorkSchedule()
	schedule := tracker.WorkScheduleConfig{
		StartMinutes:      parseTimeToMinutes(sched.StartTime),
		EndMinutes:        parseTimeToMinutes(sched.EndTime),
		LunchBreakEnabled: sched.LunchBreakEnabled,
		LunchStartMinutes: parseTimeToMinutes(sched.LunchBreakStart),
		LunchEndMinutes:   parseTimeToMinutes(sched.LunchBreakEnd),
	}

	report, err := tracker.GetMonthlyReport(a.db, year, month, contractedHours, workingDaysBitmask, schedule)
	if err != nil {
		return nil, err
	}

	return &MonthlyReportData{
		Year:                      report.Year,
		Month:                     report.Month,
		MonthName:                 report.MonthName,
		ContractedHoursPerDay:     report.ContractedHoursPerDay,
		WorkingDaysInMonth:        report.WorkingDaysInMonth,
		TotalContractedHours:      report.TotalContractedHours,
		EventsCoveredHours:        report.EventsCoveredHours,
		EffectiveContractedHours:  report.EffectiveContractedHours,
		HolidayDays:               report.HolidayDays,
		HolidayHours:              report.HolidayHours,
		VacationDays:              report.VacationDays,
		VacationHours:             report.VacationHours,
		SickDays:                  report.SickDays,
		SickHours:                 report.SickHours,
		TrasfertaDays:             report.TrasfertaDays,
		PermissionDays:            report.PermissionDays,
		PermissionHours:           report.PermissionHours,
		TotalTrackedHours:         report.TotalTrackedHours,
		TotalEffectiveWorkedHours: report.TotalEffectiveWorkedHours,
		OvertimeHours:             report.OvertimeHours,
		Days:                      report.Days,
		Projects:                  report.Projects,
	}, nil
}

// SetDayEvent imposta un evento (festa/ferie/permesso) per un giorno
func (a *App) SetDayEvent(date, eventType string, hours float64) error {
	parsedDate, err := time.Parse("2006-01-02", date)
	if err != nil || parsedDate.Format("2006-01-02") != date {
		return fmt.Errorf("data evento non valida")
	}
	if eventType != "holiday" && eventType != "vacation" && eventType != "permesso" && eventType != "malattia" && eventType != "trasferta" {
		return fmt.Errorf("tipo evento non valido: %s", eventType)
	}
	if eventType == "permesso" {
		contracted, _ := a.GetContractedHoursPerDay()
		if hours <= 0 || hours > contracted {
			return fmt.Errorf("le ore di permesso devono essere comprese tra 0 e %.2f", contracted)
		}
	} else {
		hours = 0
	}
	return tracker.SetDayEvent(a.db, date, eventType, hours)
}

// DeleteDayEvent rimuove l'evento associato a un giorno
func (a *App) DeleteDayEvent(date string) error {
	if parsed, err := time.Parse("2006-01-02", date); err != nil || parsed.Format("2006-01-02") != date {
		return fmt.Errorf("data evento non valida")
	}
	return tracker.DeleteDayEvent(a.db, date)
}

// GetContractedHoursPerDay restituisce le ore contrattuali giornaliere
func (a *App) GetContractedHoursPerDay() (float64, error) {
	val, err := tracker.GetSetting(a.db, "contracted_hours_per_day")
	if err != nil || val == "" {
		return 8.0, nil
	}
	var hours float64
	if _, err := fmt.Sscanf(val, "%f", &hours); err != nil {
		return 8.0, nil
	}
	return hours, nil
}

// SetContractedHoursPerDay imposta le ore contrattuali giornaliere
func (a *App) SetContractedHoursPerDay(hours float64) error {
	if hours < 0.5 || hours > 24 {
		return fmt.Errorf("ore contrattuali non valide: deve essere tra 0.5 e 24")
	}
	return tracker.SetSetting(a.db, "contracted_hours_per_day", fmt.Sprintf("%.2f", hours))
}

// WorkSchedule rappresenta l'orario di lavoro configurato
type WorkSchedule struct {
	StartTime         string  `json:"start_time"`
	EndTime           string  `json:"end_time"`
	LunchBreakEnabled bool    `json:"lunch_break_enabled"`
	LunchBreakStart   string  `json:"lunch_break_start"`
	LunchBreakEnd     string  `json:"lunch_break_end"`
	ContractedHours   float64 `json:"contracted_hours"`
}

// parseTimeToMinutes converte "HH:MM" in minuti dalla mezzanotte
// GetWorkSchedule restituisce l'orario di lavoro configurato
func (a *App) GetWorkSchedule() WorkSchedule {
	start, _ := tracker.GetSetting(a.db, "work_start_time")
	end, _ := tracker.GetSetting(a.db, "work_end_time")
	lunchEnabledStr, _ := tracker.GetSetting(a.db, "lunch_break_enabled")
	lunchStart, _ := tracker.GetSetting(a.db, "lunch_break_start")
	lunchEnd, _ := tracker.GetSetting(a.db, "lunch_break_end")

	if start == "" {
		start = "08:00"
	}
	if end == "" {
		end = "17:00"
	}
	if lunchStart == "" {
		lunchStart = "13:00"
	}
	if lunchEnd == "" {
		lunchEnd = "14:00"
	}
	lunchEnabled := lunchEnabledStr != "false"
	startMinutes, startErr := parseClock(start)
	endMinutes, endErr := parseClock(end)
	lunchStartMinutes, lunchStartErr := parseClock(lunchStart)
	lunchEndMinutes, lunchEndErr := parseClock(lunchEnd)
	if startErr != nil || endErr != nil || lunchStartErr != nil || lunchEndErr != nil ||
		endMinutes <= startMinutes || (lunchEnabled && (lunchStartMinutes < startMinutes || lunchEndMinutes > endMinutes || lunchEndMinutes <= lunchStartMinutes)) {
		start, end = "08:00", "17:00"
		lunchStart, lunchEnd = "13:00", "14:00"
		lunchEnabled = true
	}

	return WorkSchedule{
		StartTime:         start,
		EndTime:           end,
		LunchBreakEnabled: lunchEnabled,
		LunchBreakStart:   lunchStart,
		LunchBreakEnd:     lunchEnd,
		ContractedHours:   computeContractedHours(start, end, lunchEnabled, lunchStart, lunchEnd),
	}
}

// SetWorkSchedule salva l'orario di lavoro e aggiorna le ore contrattuali giornaliere
func (a *App) SetWorkSchedule(startTime, endTime string, lunchEnabled bool, lunchStart, lunchEnd string) error {
	startMinutes, err := parseClock(startTime)
	if err != nil {
		return err
	}
	endMinutes, err := parseClock(endTime)
	if err != nil {
		return err
	}
	lunchStartMinutes, err := parseClock(lunchStart)
	if err != nil {
		return err
	}
	lunchEndMinutes, err := parseClock(lunchEnd)
	if err != nil {
		return err
	}
	if endMinutes <= startMinutes {
		return fmt.Errorf("l'orario di fine deve essere successivo a quello di inizio")
	}
	if lunchEnabled && (lunchStartMinutes < startMinutes || lunchEndMinutes > endMinutes || lunchEndMinutes <= lunchStartMinutes) {
		return fmt.Errorf("la pausa pranzo deve essere positiva e compresa nell'orario di lavoro")
	}

	lunchEnabledStr := "false"
	if lunchEnabled {
		lunchEnabledStr = "true"
	}
	contracted := computeContractedHours(startTime, endTime, lunchEnabled, lunchStart, lunchEnd)
	if contracted < 0.5 || contracted > 24 {
		return fmt.Errorf("l'orario deve produrre tra 0.5 e 24 ore contrattuali")
	}

	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	settings := map[string]string{
		"work_start_time":          startTime,
		"work_end_time":            endTime,
		"lunch_break_enabled":      lunchEnabledStr,
		"lunch_break_start":        lunchStart,
		"lunch_break_end":          lunchEnd,
		"contracted_hours_per_day": fmt.Sprintf("%.2f", contracted),
	}
	for key, value := range settings {
		if _, err := tx.Exec(`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value); err != nil {
			return fmt.Errorf("errore salvataggio orario: %v", err)
		}
	}
	return tx.Commit()
}

// SaveMonthlyReportCSV esporta il resoconto mensile in CSV
func (a *App) SaveMonthlyReportCSV(year, month int) (string, error) {
	if err := validateYearMonth(year, month); err != nil {
		return "", err
	}
	contractedHours, err := a.GetContractedHoursPerDay()
	if err != nil {
		contractedHours = 8.0
	}

	workingDaysBitmask, err := a.GetWorkingDaysBitmask()
	if err != nil {
		workingDaysBitmask = tracker.DefaultWorkingDaysBitmask
	}

	sched := a.GetWorkSchedule()
	schedule := tracker.WorkScheduleConfig{
		StartMinutes:      parseTimeToMinutes(sched.StartTime),
		EndMinutes:        parseTimeToMinutes(sched.EndTime),
		LunchBreakEnabled: sched.LunchBreakEnabled,
		LunchStartMinutes: parseTimeToMinutes(sched.LunchBreakStart),
		LunchEndMinutes:   parseTimeToMinutes(sched.LunchBreakEnd),
	}

	report, err := tracker.GetMonthlyReport(a.db, year, month, contractedHours, workingDaysBitmask, schedule)
	if err != nil {
		return "", err
	}

	var csvBuffer bytes.Buffer
	csvBuffer.WriteString("\xEF\xBB\xBF")
	csvWriter := csv.NewWriter(&csvBuffer)
	csvWriter.Comma = ';'
	writeRow := func(values ...string) error {
		for i, value := range values {
			values[i] = neutralizeCSVFormula(value)
		}
		return csvWriter.Write(values)
	}
	if err := writeRow("Data", "Giorno", "Ore Lavorate", "Tracciato", "Evento"); err != nil {
		return "", err
	}
	for _, day := range report.Days {
		eventLabel := ""
		switch day.EventType {
		case "holiday":
			eventLabel = fmt.Sprintf("Festività -%.1fh contrattuali", day.EventHours)
		case "vacation":
			eventLabel = fmt.Sprintf("Ferie %.1fh", day.EventHours)
		case "malattia":
			eventLabel = fmt.Sprintf("Malattia %.1fh", day.EventHours)
		case "trasferta":
			eventLabel, _ = a.GetTrasfertaLabel()
		case "permesso":
			eventLabel = fmt.Sprintf("Permesso %.1fh", day.EventHours)
		}
		if err := writeRow(day.Date, day.DayOfWeek, fmt.Sprintf("%.2f", day.EffectiveWorkedHours), fmt.Sprintf("%.2f", day.TotalHours), eventLabel); err != nil {
			return "", err
		}
	}
	_ = writeRow()
	_ = writeRow("Giorni lavorativi nel mese", fmt.Sprintf("%d", report.WorkingDaysInMonth))
	_ = writeRow("Ore contrattuali giornaliere", fmt.Sprintf("%.2f", report.ContractedHoursPerDay))
	_ = writeRow("Ore contrattuali previste", fmt.Sprintf("%.2f", report.TotalContractedHours))
	_ = writeRow("Riduzione per festività", fmt.Sprintf("-%.2f", report.HolidayHours))
	_ = writeRow("Ore contrattuali del mese", fmt.Sprintf("%.2f", report.EffectiveContractedHours))
	_ = writeRow("Ferie godute (giorni)", fmt.Sprintf("%d", report.VacationDays))
	_ = writeRow("Ferie godute (ore)", fmt.Sprintf("%.2f", report.VacationHours))
	_ = writeRow("Malattia (giorni)", fmt.Sprintf("%d", report.SickDays))
	_ = writeRow("Malattia (ore)", fmt.Sprintf("%.2f", report.SickHours))
	if report.TrasfertaDays > 0 {
		trasfertaLabel, _ := a.GetTrasfertaLabel()
		_ = writeRow(fmt.Sprintf("Giorni in %s", trasfertaLabel), fmt.Sprintf("%d", report.TrasfertaDays))
	}
	_ = writeRow("Permessi goduti (giorni equivalenti)", fmt.Sprintf("%.2f", report.PermissionDays))
	_ = writeRow("Permessi goduti (ore)", fmt.Sprintf("%.2f", report.PermissionHours))
	_ = writeRow("Festività considerate (giorni)", fmt.Sprintf("%d", report.HolidayDays))
	_ = writeRow("Totale ore lavorate", fmt.Sprintf("%.2f", report.TotalEffectiveWorkedHours))
	_ = writeRow("di cui tracciate", fmt.Sprintf("%.2f", report.TotalTrackedHours))
	if report.OvertimeHours >= 0 {
		_ = writeRow("Straordinari", fmt.Sprintf("+%.2f", report.OvertimeHours))
	} else {
		_ = writeRow("Ore mancanti", fmt.Sprintf("%.2f", report.OvertimeHours))
	}

	if len(report.Projects) > 0 {
		_ = writeRow()
		_ = writeRow("Progetto", "Ore")
		for _, p := range report.Projects {
			_ = writeRow(p.ProjectName, fmt.Sprintf("%.2f", p.TotalHours))
		}
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return "", err
	}

	// Scegli dove salvare
	defaultName := fmt.Sprintf("Resoconto_%04d-%02d.csv", year, month)
	filePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: defaultName,
		Title:           "Salva Resoconto Mensile",
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
		},
	})
	if err != nil {
		return "", err
	}
	if filePath == "" {
		return "", nil
	}

	err = os.WriteFile(filePath, csvBuffer.Bytes(), 0600)
	if err != nil {
		return "", err
	}
	return filePath, nil
}
