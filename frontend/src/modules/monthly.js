import {
    AddRecurringHoliday,
    DeleteDayEvent,
    GetMonthlyReport,
    GetPatronSaintDate,
    GetRecurringHolidays,
    GetTrasfertaLabel,
    GetWorkingDaysBitmask,
    GetWorkSchedule,
    GetYearHolidays,
    PopulateItalianHolidays,
    RemoveRecurringHoliday,
    SaveMonthlyReportCSV,
    SetDayEvent,
    SetPatronSaintDate,
    SetTrasfertaLabel,
    SetWorkingDaysBitmask,
    SetWorkSchedule
} from '../wailsjs/go/main/App.js';
import { showNotification } from '../notifications.js';
import { calculateContractedHours, timeToMinutes } from '../schedule.js';
import { escapeHtml } from '../utils.js';

// === ORARIO DI LAVORO ===

window.updateComputedHours = function() {
    const start = document.getElementById('workStartTime')?.value || '08:00';
    const end   = document.getElementById('workEndTime')?.value   || '17:00';
    const lunchEnabled = document.getElementById('lunchBreakEnabled')?.checked;
    const lunchStart = document.getElementById('lunchBreakStart')?.value || '13:00';
    const lunchEnd   = document.getElementById('lunchBreakEnd')?.value   || '14:00';

    const hours = calculateContractedHours(start, end, lunchEnabled, lunchStart, lunchEnd);
    const el = document.getElementById('computedContractedHours');
    if (el) el.textContent = hours.toFixed(2);
}

window.toggleLunchBreak = function() {
    const enabled = document.getElementById('lunchBreakEnabled')?.checked;
    const fields = document.getElementById('lunchBreakFields');
    if (fields) fields.style.display = enabled ? '' : 'none';
}

export async function loadContractedHours() {
    try {
        const schedule = await GetWorkSchedule();
        const setVal = (id, val) => { const el = document.getElementById(id); if (el) el.value = val; };
        setVal('workStartTime', schedule.start_time);
        setVal('workEndTime',   schedule.end_time);
        const lunchCb = document.getElementById('lunchBreakEnabled');
        if (lunchCb) lunchCb.checked = schedule.lunch_break_enabled;
        setVal('lunchBreakStart', schedule.lunch_break_start);
        setVal('lunchBreakEnd',   schedule.lunch_break_end);
        toggleLunchBreak();
        updateComputedHours();
    } catch (error) {
        console.error('Errore caricamento orario di lavoro:', error);
    }
}

window.saveWorkSchedule = async function() {
    const start      = document.getElementById('workStartTime')?.value;
    const end        = document.getElementById('workEndTime')?.value;
    const lunch      = document.getElementById('lunchBreakEnabled')?.checked ?? false;
    const lunchStart = document.getElementById('lunchBreakStart')?.value;
    const lunchEnd   = document.getElementById('lunchBreakEnd')?.value;

    if (!start || !end) {
        showNotification('Inserisci inizio e fine turno', 'error');
        return;
    }
    if (timeToMinutes(end) <= timeToMinutes(start)) {
        showNotification('L\'orario di fine deve essere dopo l\'inizio', 'error');
        return;
    }
    if (lunch && timeToMinutes(lunchEnd) <= timeToMinutes(lunchStart)) {
        showNotification('L\'orario di fine pausa deve essere dopo l\'inizio pausa', 'error');
        return;
    }
    try {
        await SetWorkSchedule(start, end, lunch, lunchStart || '13:00', lunchEnd || '14:00');
        showNotification('Orario di lavoro salvato!', 'success');
    } catch (error) {
        showNotification('Errore: ' + error, 'error');
    }
}

// === GIORNI LAVORATIVI ===

export async function loadWorkingDays() {
    try {
        const bitmask = await GetWorkingDaysBitmask();
        applyWorkingDaysBitmask(bitmask);
    } catch (error) {
        console.error('Errore caricamento giorni lavorativi:', error);
    }
}

function applyWorkingDaysBitmask(bitmask) {
    for (let bit = 0; bit <= 6; bit++) {
        const cb = document.getElementById('wd' + bit);
        if (!cb) continue;
        const isWorking = (bitmask >> bit) & 1;
        cb.checked = !!isWorking;
        // Aggiorna colore label
        const label = cb.nextElementSibling;
        if (label) label.style.color = isWorking ? '#ffffff' : '#999999';
    }
}

window.onWorkingDayChange = function() {
    // Aggiorna colore label in tempo reale
    for (let bit = 0; bit <= 6; bit++) {
        const cb = document.getElementById('wd' + bit);
        if (!cb) continue;
        const label = cb.nextElementSibling;
        if (label) label.style.color = cb.checked ? '#ffffff' : '#999999';
    }
}

window.saveWorkingDays = async function() {
    let bitmask = 0;
    for (let bit = 0; bit <= 6; bit++) {
        const cb = document.getElementById('wd' + bit);
        if (cb && cb.checked) bitmask |= (1 << bit);
    }
    if (bitmask === 0) {
        showNotification('Seleziona almeno un giorno lavorativo', 'error');
        return;
    }
    try {
        await SetWorkingDaysBitmask(bitmask);
        showNotification('Giorni lavorativi salvati!', 'success');
    } catch (error) {
        showNotification('Errore: ' + error, 'error');
    }
}

// === EVENTI GIORNALIERI (popover) ===

let _currentEventDate = '';
let _popoverAnchor = null; // punto di ancoraggio del click, riusato ad ogni riposizionamento

window.openDayEventPopover = function(evt, date, dayLabel, currentType, currentHours) {
    evt.stopPropagation();
    _currentEventDate = date;
    document.getElementById('dayEventDate').value = date;
    document.getElementById('dayEventPopoverTitle').textContent = `${dayLabel} ${date}`;

    // Resetta selezione
    selectEventType(currentType || 'none');
    document.getElementById('permessoHoursInput').value = currentHours > 0 ? currentHours : 2;

    document.getElementById('dayEventPopover').style.display = 'block';
    document.getElementById('dayEventOverlay').style.display = 'block';

    _popoverAnchor = { x: evt.clientX, y: evt.clientY + 10 };
    repositionDayEventPopover();
}

// Riposiziona il popover in base all'ancoraggio salvato e alle dimensioni
// ATTUALI del contenuto. Va richiamato ogni volta che il contenuto può
// cambiare altezza/larghezza (es. una selezione che rivela un campo in più),
// non solo all'apertura: una soglia calcolata una sola volta si sfasa appena
// il popover cresce dopo essere già stato posizionato.
function repositionDayEventPopover() {
    const popover = document.getElementById('dayEventPopover');
    if (!_popoverAnchor || popover.style.display !== 'block') return;

    const margin = 12;
    const width = popover.offsetWidth;
    const height = popover.offsetHeight;
    const maxX = Math.max(margin, window.innerWidth - width - margin);
    const maxY = Math.max(margin, window.innerHeight - height - margin);
    const x = Math.min(Math.max(_popoverAnchor.x, margin), maxX);
    const y = Math.min(Math.max(_popoverAnchor.y, margin), maxY);
    popover.style.left = x + 'px';
    popover.style.top = y + 'px';
}

window.closeDayEventPopover = function() {
    document.getElementById('dayEventPopover').style.display = 'none';
    document.getElementById('dayEventOverlay').style.display = 'none';
    _popoverAnchor = null;
}

window.selectEventType = function(type) {
    const opts = ['none', 'holiday', 'vacation', 'malattia', 'permesso', 'trasferta'];
    opts.forEach(t => {
        const el = document.getElementById('eventOpt' + t.charAt(0).toUpperCase() + t.slice(1));
        if (el) el.classList.toggle('selected', t === type);
        const radio = el ? el.querySelector('input[type=radio]') : null;
        if (radio) radio.checked = t === type;
    });
    document.getElementById('permessoHoursRow').style.display = type === 'permesso' ? '' : 'none';
    repositionDayEventPopover();
}

window.saveDayEvent = async function() {
    const date = document.getElementById('dayEventDate').value;
    const checkedRadio = document.querySelector('input[name="dayEventType"]:checked');
    if (!checkedRadio) return;
    const type = checkedRadio.value;

    try {
        if (type === 'none') {
            await DeleteDayEvent(date);
        } else {
            const hours = type === 'permesso'
                ? parseFloat(document.getElementById('permessoHoursInput').value)
                : 0;
            await SetDayEvent(date, type, hours);
        }
        closeDayEventPopover();
        // Ricarica il resoconto per aggiornare calcoli e badge
        await loadMonthlyReport();
    } catch (error) {
        showNotification('Errore: ' + error, 'error');
    }
}

// === RESOCONTO MENSILE ===

let currentMonthlyReport = null;

export function initResocontoMonth() {
    const now = new Date();
    const monthSel = document.getElementById('resocontoMonth');
    const yearInput = document.getElementById('resocontoYear');
    if (monthSel) monthSel.value = now.getMonth() + 1;
    if (yearInput) yearInput.value = now.getFullYear();
}

window.loadMonthlyReport = async function() {
    const month = parseInt(document.getElementById('resocontoMonth').value);
    const year = parseInt(document.getElementById('resocontoYear').value);

    if (isNaN(month) || isNaN(year)) {
        showNotification('Seleziona mese e anno', 'error');
        return;
    }

    try {
        const report = await GetMonthlyReport(year, month);
        currentMonthlyReport = report;
        renderMonthlyReport(report);
        document.getElementById('btnExportCSV').disabled = false;
    } catch (error) {
        console.error('Errore caricamento resoconto:', error);
        showNotification('Errore caricamento resoconto: ' + error, 'error');
    }
}

const EVENT_LABELS = {
    holiday:   { text: 'Festa',     color: '#B1583F', bg: 'rgba(177,88,63,0.15)' },
    vacation:  { text: 'Ferie',     color: '#6C89A8', bg: 'rgba(108,137,168,0.15)' },
    malattia:  { text: 'Malattia',  color: '#93829A', bg: 'rgba(147,130,154,0.15)' },
    permesso:  { text: 'Permesso',  color: '#7A9B8E', bg: 'rgba(122,155,142,0.15)' },
    trasferta: { text: 'Trasferta', color: '#C99A54', bg: 'rgba(201,154,84,0.15)' },
};

function formatReportNumber(value, maximumFractionDigits = 2) {
    return Number(value || 0).toLocaleString('it-IT', { maximumFractionDigits });
}

function renderMonthlyReport(report) {
    // Titolo
    document.getElementById('resocontoTitle').textContent = report.month_name;

    // Cifre riepilogo — mostra ore lavorate effettive (con default giorni non tracciati)
    document.getElementById('summaryTracked').textContent = report.total_effective_worked_hours.toFixed(2) + ' h';
    const subEl = document.getElementById('summaryTrackedSub');
    if (subEl) {
        const diff = report.total_effective_worked_hours - report.total_tracked_hours;
        if (diff > 0.01) {
            subEl.textContent = `tracciato: ${report.total_tracked_hours.toFixed(2)}h`;
        } else {
            subEl.textContent = '';
        }
    }

    // Solo le festività riducono le ore contrattuali; ferie e permessi restano separati.
    const contractedEl = document.getElementById('summaryContracted');
    if (report.holiday_hours > 0) {
        contractedEl.textContent = report.effective_contracted_hours.toFixed(2) + ' h';
        contractedEl.title = `Previste: ${report.total_contracted_hours.toFixed(2)}h − ${report.holiday_hours.toFixed(2)}h di festività`;
    } else {
        contractedEl.textContent = report.total_contracted_hours.toFixed(2) + ' h';
        contractedEl.title = '';
    }

    document.getElementById('summaryWorkingDays').textContent = report.working_days_in_month;
    document.getElementById('summaryVacation').textContent = `${formatReportNumber(report.vacation_hours)} h`;
    document.getElementById('summaryPermission').textContent = `${formatReportNumber(report.permission_hours)} h`;
    document.getElementById('summaryMalattia').textContent = `${formatReportNumber(report.sick_hours)} h`;
    document.getElementById('summaryTrasferta').textContent = `${formatReportNumber(report.trasferta_days, 0)} gg`;
    document.getElementById('summaryHoliday').textContent = `−${formatReportNumber(report.holiday_hours)} h`;

    const overtimeEl = document.getElementById('summaryOvertime');
    const overtimeLabelEl = document.getElementById('summaryOvertimeLabel');
    const overtime = report.overtime_hours;
    if (overtime >= 0) {
        overtimeEl.textContent = '+' + overtime.toFixed(2) + ' h';
        overtimeEl.style.color = '#C99A54';
        overtimeLabelEl.textContent = 'Straordinari';
    } else {
        overtimeEl.textContent = overtime.toFixed(2) + ' h';
        overtimeEl.style.color = '#B1583F';
        overtimeLabelEl.textContent = 'Ore Mancanti';
    }

    document.getElementById('resocontoSummary').style.display = '';

    // Tabella giornaliera
    const daysList = document.getElementById('resocontoDaysList');
    if (!report.days || report.days.length === 0) {
        daysList.innerHTML = '<p style="color: #6b7280;">Nessun dato per questo mese</p>';
    } else {
        let html = '<table style="width:100%; border-collapse: collapse; font-size: 0.9em;">';
        html += '<thead><tr style="color:#999; border-bottom: 1px solid #322B24;">';
        html += '<th style="text-align:left; padding: 8px 6px;">Data</th>';
        html += '<th style="text-align:left; padding: 8px 6px;">Giorno</th>';
        html += '<th style="text-align:right; padding: 8px 6px;">Ore</th>';
        html += '<th style="padding: 8px 6px; width:80px;"></th>';
        html += '<th style="padding: 8px 6px; width:110px;">Evento</th>';
        html += '</tr></thead><tbody>';

        const contracted = report.contracted_hours_per_day || 8;
        for (const day of report.days) {
            const hasTracked = day.total_hours > 0;
            const hasEvent = !!day.event_type;
            const isOff = day.event_type === 'holiday' || day.event_type === 'vacation' || day.event_type === 'malattia';
            const eff = day.effective_worked_hours;
            const tracked = day.total_hours;

            // Stile riga
            let rowStyle = 'border-bottom: 1px solid #2a2a2a;';
            if (day.is_weekend) rowStyle += 'color:#6B6259;';

            // Ore da mostrare
            const outsideH = day.outside_work_hours || 0;
            const insideTracked = Math.max(tracked - outsideH, 0);
            let hoursDisplay = '—';
            let hoursStyle = 'padding: 8px 6px; text-align:right; white-space:nowrap;';
            if (day.is_weekend) {
                if (tracked > 0) { hoursDisplay = tracked.toFixed(2); hoursStyle += 'color:#888;'; }
            } else if (isOff) {
                if (outsideH > 0) {
                    hoursDisplay = `<span style="color:#C99A54;font-weight:600;">${outsideH.toFixed(2)}</span> <span style="font-size:0.75em;color:#6B6259;">OT</span>`;
                }
            } else {
                // Giorno lavorativo normale
                if (outsideH > 0) {
                    // Straordinario fuori orario: mostra totale con badge OT
                    hoursDisplay = `<span style="font-weight:600;">${eff.toFixed(2)}</span> <span style="font-size:0.72em;color:#C99A54;font-weight:600;">+${outsideH.toFixed(2)}h OT</span>`;
                } else if (!day.uses_default) {
                    // Tutto tracciato, nessuno straordinario (copertura piena)
                    hoursDisplay = eff.toFixed(2);
                    hoursStyle += 'font-weight:600;';
                } else if (hasTracked) {
                    // Tracciamento parziale
                    hoursDisplay = `${eff.toFixed(2)} <span style="font-size:0.75em;color:#6B6259;font-style:italic;">${tracked.toFixed(2)}↑</span>`;
                    hoursStyle += 'color:#aaa;';
                } else {
                    // Nessun tracciamento: default puro
                    hoursDisplay = eff.toFixed(2);
                    hoursStyle += 'color:#6B6259; font-style:italic;';
                }
            }

            // Barra a tre segmenti:
            //  [arancione] = tracciato dentro l'orario
            //  [grigio]    = contrattuale assunto non tracciato
            //  [rosso]     = straordinario fuori orario
            let barHtml = '';
            if (!day.is_weekend) {
                const ref = contracted || 8;
                const insidePct  = Math.min(insideTracked / ref * 100, 100);
                const defaultPct = isOff ? 0 : Math.max(Math.min((contracted - insideTracked) / ref * 100, 100 - insidePct), 0);
                const otPct      = Math.min(outsideH / ref * 100, 60); // cap visivo al 60%
                barHtml = `<div style="background:#1e1e1e; border-radius:4px; height:8px; display:flex; overflow:hidden;">`;
                if (insidePct > 0)
                    barHtml += `<div style="background:#C99A54; width:${insidePct}%;"></div>`;
                if (defaultPct > 0)
                    barHtml += `<div style="background:#322B24; width:${defaultPct}%;"></div>`;
                if (otPct > 0)
                    barHtml += `<div style="background:#B1583F; width:${otPct}%; flex-shrink:0;"></div>`;
                barHtml += `</div>`;
            }

            // Badge evento
            let eventCell = '';
            if (hasEvent) {
                const ev = EVENT_LABELS[day.event_type] || { text: day.event_type, color: '#999', bg: 'rgba(255,255,255,0.05)' };
                const label = day.event_type === 'trasferta'
                    ? ev.text
                    : (day.event_type === 'permesso' || day.event_type === 'vacation' || day.event_type === 'malattia')
                        ? `${ev.text} ${formatReportNumber(day.event_hours)}h`
                        : `${ev.text} −${formatReportNumber(day.event_hours)}h`;
                eventCell = `<span data-day-event="true" data-date="${escapeHtml(day.date)}" data-day-label="${escapeHtml(day.day_of_week)}"
                    data-event-type="${escapeHtml(day.event_type)}" data-event-hours="${Number(day.event_hours) || 0}"
                    style="display:inline-block;padding:3px 8px;border-radius:8px;font-size:0.78em;font-weight:600;cursor:pointer;
                    color:${ev.color};background:${ev.bg};border:1px solid ${ev.color}33;">${escapeHtml(label)}</span>`;
            } else if (!day.is_weekend) {
                eventCell = `<span data-day-event="true" data-date="${escapeHtml(day.date)}" data-day-label="${escapeHtml(day.day_of_week)}"
                    data-event-type="" data-event-hours="0"
                    style="display:inline-block;padding:3px 8px;border-radius:8px;font-size:0.78em;cursor:pointer;
                    color:#6B6259;background:#1C1815;border:1px dashed #322B24;">+ evento</span>`;
            }

            html += `<tr style="${rowStyle}">`;
            html += `<td style="padding: 8px 6px; font-variant-numeric: tabular-nums;">${escapeHtml(day.date)}</td>`;
            html += `<td style="padding: 8px 6px;">${escapeHtml(day.day_of_week)}</td>`;
            html += `<td style="${hoursStyle}">${hoursDisplay}</td>`;
            html += `<td style="padding: 8px 6px;">${barHtml}</td>`;
            html += `<td style="padding: 6px 6px;">${eventCell}</td>`;
            html += `</tr>`;
        }
        html += '</tbody></table>';
        daysList.innerHTML = html;
        daysList.querySelectorAll('[data-day-event="true"]').forEach(element => {
            element.addEventListener('click', event => {
                window.openDayEventPopover(
                    event,
                    element.dataset.date,
                    element.dataset.dayLabel,
                    element.dataset.eventType,
                    Number(element.dataset.eventHours) || 0
                );
            });
        });
    }

    // Lista progetti
    const projectsList = document.getElementById('resocontoProjectsList');
    if (!report.projects || report.projects.length === 0) {
        projectsList.innerHTML = '<p style="color: #6b7280;">Nessun progetto</p>';
    } else {
        const maxHours = Math.max(...report.projects.map(p => p.total_hours));
        let html = '';
        for (const proj of report.projects) {
            const barWidth = maxHours > 0 ? (proj.total_hours / maxHours * 100) : 0;
            html += `<div style="margin-bottom: 14px;">`;
            html += `<div style="display:flex; justify-content:space-between; margin-bottom:4px;">`;
            html += `<span style="font-weight:600; font-size:0.9em;">${escapeHtml(proj.project_name)}</span>`;
            html += `<span style="color:#C99A54; font-weight:600;">${proj.total_hours.toFixed(2)} h</span>`;
            html += `</div>`;
            html += `<div style="background:#2a2a2a; border-radius:4px; height:8px;">`;
            html += `<div style="background:#C99A54; height:100%; width:${barWidth}%; border-radius:4px;"></div>`;
            html += `</div></div>`;
        }
        projectsList.innerHTML = html;
    }
}

// === FESTIVITÀ ITALIANE ===

export async function loadPatronSaintDate() {
    try {
        const monthDay = await GetPatronSaintDate(); // "MM-GG" o ""
        if (!monthDay) return;
        const [month, day] = monthDay.split('-').map(Number);
        const monthSel = document.getElementById('patronSaintMonth');
        const dayInput = document.getElementById('patronSaintDay');
        if (monthSel) monthSel.value = month;
        if (dayInput) dayInput.value = day;
    } catch (error) {
        console.error('Errore caricamento festa patronale:', error);
    }
}

window.savePatronSaintDate = async function() {
    const month = parseInt(document.getElementById('patronSaintMonth').value);
    const day = parseInt(document.getElementById('patronSaintDay').value);
    if (!day || day < 1 || day > 31) {
        showNotification('Inserisci un giorno valido', 'error');
        return;
    }
    const monthDay = `${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
    try {
        await SetPatronSaintDate(monthDay);
        showNotification('Festa patronale salvata!', 'success');
    } catch (error) {
        showNotification('Errore: ' + error, 'error');
    }
}

export async function loadTrasfertaLabel() {
    try {
        const label = await GetTrasfertaLabel();
        applyTrasfertaLabel(label);
        const input = document.getElementById('trasfertaLabelInput');
        if (input) input.value = label;
    } catch (error) {
        console.error('Errore caricamento nome categoria trasferta:', error);
    }
}

function applyTrasfertaLabel(label) {
    EVENT_LABELS.trasferta.text = label;
    const popoverSpan = document.getElementById('trasfertaLabelPopover');
    if (popoverSpan) popoverSpan.textContent = label;
    const summaryLabel = document.getElementById('summaryTrasfertaLabel');
    if (summaryLabel) summaryLabel.textContent = label;
}

window.saveTrasfertaLabel = async function() {
    const label = document.getElementById('trasfertaLabelInput').value.trim();
    if (!label) {
        showNotification('Inserisci un nome', 'error');
        return;
    }
    try {
        await SetTrasfertaLabel(label);
        applyTrasfertaLabel(label);
        showNotification('Nome categoria salvato!', 'success');
    } catch (error) {
        showNotification('Errore: ' + error, 'error');
    }
}

window.populateItalianHolidays = async function() {
    const year = parseInt(document.getElementById('holidaysYear').value);
    if (isNaN(year)) {
        showNotification('Seleziona un anno', 'error');
        return;
    }
    try {
        const added = await PopulateItalianHolidays(year);
        showNotification(added > 0 ? `${added} festività aggiunte al calendario ${year}` : 'Festività già presenti, nessuna aggiunta', 'success');
        await loadYearHolidays();
        if (currentMonthlyReport && currentMonthlyReport.year === year) {
            await window.loadMonthlyReport();
        }
    } catch (error) {
        showNotification('Errore: ' + error, 'error');
    }
}

// === ELENCO FESTIVITÀ DELL'ANNO (Impostazioni) ===

export async function loadYearHolidays() {
    const yearInput = document.getElementById('holidaysYear');
    const year = parseInt(yearInput?.value);
    if (isNaN(year)) return;
    try {
        const dates = await GetYearHolidays(year);
        renderYearHolidays(dates);
    } catch (error) {
        console.error('Errore caricamento festività:', error);
    }
}

function renderYearHolidays(dates) {
    const container = document.getElementById('holidaysYearList');
    if (!container) return;
    if (!dates || dates.length === 0) {
        container.innerHTML = '<p style="color: #6b7280;">Nessuna festività per questo anno. Genera il calendario italiano o aggiungine una.</p>';
        return;
    }
    let html = '<div style="display:flex; flex-direction:column; gap:6px;">';
    for (const date of dates) {
        const dayLabel = new Date(date + 'T00:00:00').toLocaleDateString('it-IT', { weekday: 'long', day: 'numeric', month: 'long' });
        html += `<div style="display:flex; justify-content:space-between; align-items:center; background:#1C1815; padding:8px 12px; border-radius:8px;">
            <span style="text-transform:capitalize;">${escapeHtml(dayLabel)}</span>
            <button data-action="removeYearHoliday" data-action-arg="${escapeHtml(date)}" style="background:none;border:none;color:#B1583F;cursor:pointer;font-size:0.85em;">Rimuovi</button>
        </div>`;
    }
    html += '</div>';
    container.innerHTML = html;
}

window.removeYearHoliday = async function(date) {
    try {
        await DeleteDayEvent(date);
        await loadYearHolidays();
    } catch (error) {
        showNotification('Errore: ' + error, 'error');
    }
}

// === FESTIVITÀ RICORRENTI PERSONALIZZATE ===

export async function loadRecurringHolidays() {
    try {
        const list = await GetRecurringHolidays();
        renderRecurringHolidays(list);
    } catch (error) {
        console.error('Errore caricamento festività ricorrenti:', error);
    }
}

const MONTH_NAMES_IT = ['Gennaio', 'Febbraio', 'Marzo', 'Aprile', 'Maggio', 'Giugno', 'Luglio', 'Agosto', 'Settembre', 'Ottobre', 'Novembre', 'Dicembre'];

function renderRecurringHolidays(list) {
    const container = document.getElementById('recurringHolidaysList');
    if (!container) return;
    if (!list || list.length === 0) {
        container.innerHTML = '<p style="color: #6B6259; font-size: 0.85em;">Nessuna</p>';
        return;
    }
    let html = '<div style="display:flex; flex-direction:column; gap:6px;">';
    for (const h of list) {
        const dateLabel = `${h.day} ${MONTH_NAMES_IT[h.month - 1]}`;
        const labelText = h.label ? `${dateLabel} — ${h.label}` : dateLabel;
        html += `<div style="display:flex; justify-content:space-between; align-items:center; background:#1C1815; padding:8px 12px; border-radius:8px;">
            <span style="font-size:0.9em;">${escapeHtml(labelText)}</span>
            <button data-action="removeRecurringHoliday" data-action-arg="${h.month}-${h.day}" style="background:none;border:none;color:#B1583F;cursor:pointer;font-size:0.85em;">Rimuovi</button>
        </div>`;
    }
    html += '</div>';
    container.innerHTML = html;
}

window.removeRecurringHoliday = async function(monthDay) {
    const [month, day] = monthDay.split('-').map(Number);
    try {
        await RemoveRecurringHoliday(month, day);
        await loadRecurringHolidays();
    } catch (error) {
        showNotification('Errore: ' + error, 'error');
    }
}

window.addRecurringHoliday = async function() {
    const month = parseInt(document.getElementById('recurringHolidayMonth').value);
    const day = parseInt(document.getElementById('recurringHolidayDay').value);
    const label = document.getElementById('recurringHolidayLabel').value.trim();
    if (!day) {
        showNotification('Inserisci un giorno valido', 'error');
        return;
    }
    try {
        await AddRecurringHoliday(month, day, label);
        document.getElementById('recurringHolidayDay').value = '';
        document.getElementById('recurringHolidayLabel').value = '';
        await loadRecurringHolidays();
        showNotification('Festività ricorrente aggiunta', 'success');
    } catch (error) {
        showNotification('Errore: ' + error, 'error');
    }
}

window.exportMonthlyReportCSV = async function() {
    const month = parseInt(document.getElementById('resocontoMonth').value);
    const year = parseInt(document.getElementById('resocontoYear').value);
    try {
        const filePath = await SaveMonthlyReportCSV(year, month);
        if (filePath) {
            showNotification('CSV salvato in: ' + filePath, 'success');
        }
    } catch (error) {
        showNotification('Errore export CSV: ' + error, 'error');
    }
}
