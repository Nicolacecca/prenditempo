// Import Wails bindings
import { DeleteProject } from './wailsjs/go/main/App.js';
import { GetSessions, DeleteSession } from './wailsjs/go/main/App.js';
import { MigrateLegacyNotes } from './wailsjs/go/main/App.js';
import { GetActivityTypes, GetAllActivityTypes, CreateActivityType, UpdateActivityType, DeleteActivityType, ReorderActivityTypes } from './wailsjs/go/main/App.js';
import { ExportData, ImportData } from './wailsjs/go/main/App.js';
import { ImportProjectJSON } from './wailsjs/go/main/App.js';
import { IsAutoStartEnabled, EnableAutoStart, DisableAutoStart } from './wailsjs/go/main/App.js';
import { SetIdleThreshold, GetIdleThreshold } from './wailsjs/go/main/App.js';
import { SetMergeGapSeconds, GetMergeGapSeconds } from './wailsjs/go/main/App.js';
import { escapeHtml } from './utils.js';
import { showNotification } from './notifications.js';
import { initResocontoMonth, loadContractedHours, loadPatronSaintDate, loadRecurringHolidays, loadTrasfertaLabel, loadWorkingDays, loadYearHolidays } from './modules/monthly.js';
import { showReportModal } from './modules/project-report.js';
import { configureSessionModule } from './modules/sessions.js';
import { getProjects, loadProjects } from './modules/projects.js';
import { loadArchivedProjects } from './modules/archive.js';
import { initializeTracking } from './modules/tracking.js';
import { initializeUIActions } from './ui-actions.js';
import './modules/board-link.js';

// === UTILITY FUNCTIONS ===

// Constants for validation
// Variabili globali
let activityTypes = [];
// Include i tipi archiviati: serve solo a colorare le sessioni storiche
let allActivityTypes = [];

initializeUIActions();

configureSessionModule({
    getActivityTypes: () => activityTypes,
    getProjects,
    loadTimeline: () => window.loadTimeline()
});

// Inizializzazione
document.addEventListener('DOMContentLoaded', async function() {
    await loadActivityTypes();
    await loadProjects();
    await initializeTracking(() => window.loadTimeline());
    window.setToday();
});

// === TIPI DI ATTIVITÀ ===

async function loadActivityTypes() {
    try {
        activityTypes = await GetActivityTypes();
        allActivityTypes = await GetAllActivityTypes();
        populateActivityTypeSelect();
    } catch (error) {
        console.error('Errore caricamento tipi attività:', error);
    }
}

function populateActivityTypeSelect() {
    const select = document.getElementById('activityTypeSelect');
    if (!select) return;

    select.innerHTML = '<option value="">Tipo attività (opzionale)...</option>';
    activityTypes.forEach(type => {
        const option = document.createElement('option');
        option.value = type.name;
        option.textContent = type.name;
        select.appendChild(option);
    });
}



// === TIMELINE ===

window.setToday = function() {
    const today = new Date().toISOString().split('T')[0];
    document.getElementById('timelineStartDate').value = today;
    document.getElementById('timelineEndDate').value = today;
    loadTimeline();
}

window.setThisWeek = function() {
    const today = new Date();
    const monday = new Date(today);
    monday.setDate(today.getDate() - today.getDay() + 1);
    const sunday = new Date(monday);
    sunday.setDate(monday.getDate() + 6);

    document.getElementById('timelineStartDate').value = monday.toISOString().split('T')[0];
    document.getElementById('timelineEndDate').value = sunday.toISOString().split('T')[0];
    loadTimeline();
}

window.loadTimeline = async function() {
    const startDate = document.getElementById('timelineStartDate').value;
    const endDate = document.getElementById('timelineEndDate').value;

    if (!startDate || !endDate) {
        return;
    }

    try {
        const sessions = await GetSessions(startDate, endDate);

        displayTimeline(sessions || [], startDate, endDate);
    } catch (error) {
        console.error('Errore caricamento timeline:', error);
        showNotification('Errore caricamento timeline', 'error');
    }
}

function displayTimeline(sessions, startDate, endDate) {
    const content = document.getElementById('timelineContent');

    if (!sessions || sessions.length === 0) {
        content.innerHTML = '<p style="color: #6b7280;">Nessuna attività registrata per questo periodo</p>';
        return;
    }

    // Ordina sessioni per timestamp
    sessions.sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp));

    // Trova tutti i progetti unici
    const uniqueProjects = [...new Set(sessions.map(s => s.project_name || 'Nessun progetto'))];

    // Calcola range temporale
    const [startYear, startMonth, startDay] = startDate.split('-').map(Number);
    const [endYear, endMonth, endDay] = endDate.split('-').map(Number);
    const startTime = new Date(startYear, startMonth - 1, startDay, 0, 0, 0);
    const endTime = new Date(endYear, endMonth - 1, endDay, 23, 59, 59);
    const totalMs = endTime - startTime;

    // Determina se è un singolo giorno e calcola numero di giorni
    const isSingleDay = startDate === endDate;
    const numDays = Math.ceil((endTime - startTime) / (24 * 60 * 60 * 1000));

    // Raggruppa sessioni per progetto
    const sessionsByProject = {};
    sessions.forEach(session => {
        const projectName = session.project_name || 'Nessun progetto';
        if (!sessionsByProject[projectName]) {
            sessionsByProject[projectName] = [];
        }
        sessionsByProject[projectName].push(session);
    });

    // Costruisci HTML
    let html = '<div style="margin-bottom: 10px; color: #6b7280; font-size: 0.9em;">';
    html += `Periodo: ${formatDate(startDate)} - ${formatDate(endDate)}`;
    html += '</div>';

    // Wrapper per timeline e marker con overflow hidden per limitare le linee verticali
    html += '<div style="position: relative; overflow: hidden;">';

    // Markers temporali adattivi in base al periodo
    html += '<div class="timeline-hour-markers">';

    if (numDays === 1) {
        // Timeline giornaliera: mostra ore ogni ora
        for (let hour = 0; hour <= 23; hour++) {
            const hourMs = hour * 60 * 60 * 1000;
            const percentage = (hourMs / totalMs) * 100;
            const spanStyle = hour === 0 ? 'style="position: relative; left: 50%;"' : '';
            html += `<div class="timeline-hour-marker" style="left: ${percentage}%;"><span ${spanStyle}>${hour}</span></div>`;
        }
        // Aggiungi marker per 24 (fine giornata)
        html += `<div class="timeline-hour-marker" style="left: 100%;"><span style="position: relative; right: 50%;">24</span></div>`;
    } else if (numDays <= 7) {
        // Timeline settimanale: mostra giorni
        for (let d = 0; d <= numDays; d++) {
            const dayMs = d * 24 * 60 * 60 * 1000;
            const percentage = (dayMs / totalMs) * 100;
            const date = new Date(startTime.getTime() + dayMs);
            const dayName = date.toLocaleDateString('it-IT', { weekday: 'short' });
            const spanStyle = d === 0 ? 'style="position: relative; left: 50%;"' : (d === numDays ? 'style="position: relative; right: 50%;"' : '');
            html += `<div class="timeline-hour-marker" style="left: ${percentage}%;"><span ${spanStyle}>${dayName} ${date.getDate()}/${date.getMonth() + 1}</span></div>`;
        }
    } else if (numDays <= 31) {
        // Timeline mensile: mostra date ogni 3-4 giorni
        const step = Math.ceil(numDays / 8);
        const markers = [];
        for (let d = 0; d <= numDays; d += step) {
            markers.push(d);
        }
        markers.forEach((d, index) => {
            const dayMs = d * 24 * 60 * 60 * 1000;
            const percentage = (dayMs / totalMs) * 100;
            const date = new Date(startTime.getTime() + dayMs);
            const spanStyle = index === 0 ? 'style="position: relative; left: 50%;"' : (index === markers.length - 1 ? 'style="position: relative; right: 50%;"' : '');
            html += `<div class="timeline-hour-marker" style="left: ${percentage}%;"><span ${spanStyle}>${date.getDate()}/${date.getMonth() + 1}</span></div>`;
        });
    } else if (numDays <= 62) {
        // Timeline 32-62 giorni: mostra ogni 7 giorni
        for (let d = 0; d <= numDays; d += 7) {
            const dayMs = d * 24 * 60 * 60 * 1000;
            const percentage = (dayMs / totalMs) * 100;
            const date = new Date(startTime.getTime() + dayMs);
            const spanStyle = d === 0 ? 'style="position: relative; left: 50%;"' : (d + 7 > numDays ? 'style="position: relative; right: 50%;"' : '');
            html += `<div class="timeline-hour-marker" style="left: ${percentage}%;"><span ${spanStyle}>${date.getDate()}/${date.getMonth() + 1}</span></div>`;
        }
    } else {
        // Timeline oltre 2 mesi: mostra solo il primo giorno di ogni mese
        let currentDate = new Date(startTime.getFullYear(), startTime.getMonth(), 1);
        if (currentDate < startTime) {
            currentDate = new Date(startTime.getFullYear(), startTime.getMonth() + 1, 1);
        }
        let isFirst = true;
        while (currentDate <= endTime) {
            const dayMs = currentDate - startTime;
            const percentage = (dayMs / totalMs) * 100;
            const monthNames = ['Gen', 'Feb', 'Mar', 'Apr', 'Mag', 'Giu', 'Lug', 'Ago', 'Set', 'Ott', 'Nov', 'Dic'];
            const label = `1 ${monthNames[currentDate.getMonth()]}`;
            const spanStyle = isFirst ? 'style="position: relative; left: 50%;"' : '';
            html += `<div class="timeline-hour-marker" style="left: ${percentage}%;"><span ${spanStyle}>${label}</span></div>`;
            isFirst = false;
            currentDate = new Date(currentDate.getFullYear(), currentDate.getMonth() + 1, 1);
        }
    }

    html += '</div>';

    // Timeline per ogni progetto
    uniqueProjects.forEach(projectName => {
        const projectSessions = sessionsByProject[projectName] || [];

        // Label del progetto (con background per interrompere le linee dei marker)
        html += `<div style="margin-top: 15px; margin-bottom: 5px; font-weight: 600; color: #E4DACB; position: relative; z-index: 200; background: #211C17; padding: 5px 0;">`;
        html += `${escapeHtml(projectName)}`;
        html += `</div>`;

        html += '<div class="timeline-bar">';

        projectSessions.forEach(session => {
            const segment = createTimelineSegment(session, startTime, totalMs);
            html += segment;
        });

        html += '</div>';
    });

    html += '</div>';

    // Statistiche riepilogo
    const projectStats = {};
    const activityStats = {};
    sessions.forEach(s => {
        const proj = s.project_name || 'Nessun progetto';
        projectStats[proj] = (projectStats[proj] || 0) + s.seconds;

        const activity = s.activity_type || 'Nessuna';
        activityStats[activity] = (activityStats[activity] || 0) + s.seconds;
    });

    const totalSeconds = Object.values(projectStats).reduce((sum, sec) => sum + sec, 0);

    html += '<div style="margin-top: 25px; display: grid; grid-template-columns: 1fr 1fr; gap: 20px;">';

    // Riepilogo per progetto
    html += '<div>';
    html += '<h3 style="margin-bottom: 10px; color: #E4DACB;">Per Progetto</h3>';
    html += '<div class="stats-grid">';

    for (const [proj, seconds] of Object.entries(projectStats).sort((a, b) => b[1] - a[1])) {
        const hours = (seconds / 3600).toFixed(1);
        const percentage = ((seconds / totalSeconds) * 100).toFixed(1);

        html += `
            <div class="stat-item">
                <span class="stat-label"><strong style="color: #E4DACB;">${escapeHtml(proj)}</strong></span>
                <span class="stat-value">${hours}h (${percentage}%)</span>
            </div>
        `;
    }
    html += '</div></div>';

    // Riepilogo per tipo attività
    html += '<div>';
    html += '<h3 style="margin-bottom: 10px; color: #E4DACB;">Per Tipo Attività</h3>';
    html += '<div class="stats-grid">';

    for (const [activity, seconds] of Object.entries(activityStats).sort((a, b) => b[1] - a[1])) {
        const hours = (seconds / 3600).toFixed(1);
        const percentage = ((seconds / totalSeconds) * 100).toFixed(1);

        html += `
            <div class="stat-item">
                <span class="stat-label"><strong style="color: #E4DACB;">${escapeHtml(activity)}</strong></span>
                <span class="stat-value">${hours}h (${percentage}%)</span>
            </div>
        `;
    }
    html += '</div></div>';

    html += '</div>';

    content.innerHTML = html;
    content.querySelectorAll('.timeline-segment').forEach(segment => {
        segment.addEventListener('click', event => {
            window.showSessionMenu(
                event,
                Number(segment.dataset.sessionId),
                Number(segment.dataset.seconds),
                segment.dataset.activityType,
                segment.dataset.projectName
            );
        });
    });
}

function createTimelineSegment(session, startTime, totalMs) {
    if (!session.timestamp) return '';

    // Parse timestamp trattandolo sempre come ora locale
    // Questo evita conversioni timezone indesiderate
    let sessionTime;
    let displayHours, displayMinutes, displayDay, displayMonth;

    if (session.timestamp.includes('T')) {
        // Formato ISO: "2025-01-15T14:30:00Z" o "2025-01-15T14:30:00"
        // Estrai i componenti direttamente dalla stringa senza conversione timezone
        const isoMatch = session.timestamp.match(/(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):?(\d{2})?/);
        if (isoMatch) {
            const [, year, month, day, hours, minutes, seconds] = isoMatch.map(Number);
            sessionTime = new Date(year, month - 1, day, hours, minutes, seconds || 0);
            displayHours = hours;
            displayMinutes = minutes;
            displayDay = day;
            displayMonth = month;
        } else {
            sessionTime = new Date(session.timestamp);
            displayHours = sessionTime.getHours();
            displayMinutes = sessionTime.getMinutes();
            displayDay = sessionTime.getDate();
            displayMonth = sessionTime.getMonth() + 1;
        }
    } else {
        // Formato SQLite: "2025-01-15 14:30:00"
        const parts = session.timestamp.split(' ');
        const datePart = parts[0];
        const timePart = parts[1];
        const [year, month, day] = datePart.split('-').map(Number);
        const [hours, minutes, seconds] = timePart.split(':').map(Number);
        sessionTime = new Date(year, month - 1, day, hours, minutes, seconds || 0);
        displayHours = hours;
        displayMinutes = minutes;
        displayDay = day;
        displayMonth = month;
    }

    const sessionMs = sessionTime - startTime;
    const left = (sessionMs / totalMs) * 100;
    const sessionDurationMs = session.seconds * 1000;
    const width = (sessionDurationMs / totalMs) * 100;

    const timeStr = `${String(displayHours).padStart(2, '0')}:${String(displayMinutes).padStart(2, '0')}`;
    const dateStr = `${displayDay}/${displayMonth}`;

    const activityTypeName = session.activity_type || 'Nessuna';
    const tooltipText = `${escapeHtml(activityTypeName)}\nInizio: ${dateStr} ${timeStr}\nDurata: ${Math.floor(session.seconds / 60)} min`;

    // Trova tipo attività per colore e pattern
    const activityTypeObj = allActivityTypes.find(t => t.name === activityTypeName);
    let bgStyle = 'background: #E4DACB;';

    // Se il tipo attività è "Nessuna" (null/vuoto), usa arancione semi-trasparente
    if (activityTypeName === 'Nessuna' || !activityTypeName) {
        bgStyle = 'background: rgba(255, 107, 43, 0.5);'; // Arancione semi-trasparente
    } else if (activityTypeObj) {
        const colorVariant = activityTypeObj.color_variant || 0;
        const pattern = activityTypeObj.pattern || 'solid';

        // Calcola colore
        let r = 255, g = 255, b = 255;
        if (colorVariant < 0) {
            r += r * colorVariant;
            g += g * colorVariant;
            b += b * colorVariant;
        }

        const color = `rgb(${Math.round(r)}, ${Math.round(g)}, ${Math.round(b)})`;
        const lightColor = `rgb(${Math.min(255, Math.round(r + 40))}, ${Math.min(255, Math.round(g + 40))}, ${Math.min(255, Math.round(b + 40))})`;

        if (pattern === 'stripes') {
            bgStyle = `background: repeating-linear-gradient(45deg, ${color}, ${color} 3px, ${lightColor} 3px, ${lightColor} 6px);`;
        } else if (pattern === 'dots') {
            bgStyle = `background-color: ${color}; background-image: radial-gradient(circle, ${lightColor} 1.5px, transparent 1.5px); background-size: 6px 6px;`;
        } else {
            bgStyle = `background: ${color};`;
        }
    }

    return `
        <div class="timeline-segment"
             style="left: ${left}%; width: ${Math.max(width, 0.5)}%; ${bgStyle} z-index: 2;"
             title="${escapeHtml(tooltipText).replace(/"/g, '&quot;')}"
             data-session-id="${parseInt(session.id)}"
             data-seconds="${parseInt(session.seconds)}"
             data-activity-type="${escapeHtml(activityTypeName)}"
             data-project-name="${escapeHtml(session.project_name || '')}">
        </div>
    `;
}

function formatDate(dateStr) {
    const date = new Date(dateStr);
    return `${date.getDate()}/${date.getMonth() + 1}/${date.getFullYear()}`;
}

window.migrateLegacyNotes = async function() {
    try {
        const count = await MigrateLegacyNotes();
        if (count > 0) {
            showNotification(`${count} note importate con successo!`, 'success');
            // Ricarica i progetti per aggiornare le note
            await loadProjects();
        } else {
            showNotification('Nessuna nota legacy trovata da importare', 'info');
        }
    } catch (error) {
        console.error('Errore migrazione note:', error);
        showNotification('Errore: ' + error, 'error');
    }
}



// === NAVIGAZIONE SEZIONI ===

window.showSection = function(section) {
    const dashboardSection = document.getElementById('dashboardSection');
    const archiveSection = document.getElementById('archiveSection');
    const settingsSection = document.getElementById('settingsSection');
    const resocontoSection = document.getElementById('resocontoSection');
    const navDashboard = document.getElementById('navDashboard');
    const navArchive = document.getElementById('navArchive');
    const navSettings = document.getElementById('navSettings');
    const navResoconto = document.getElementById('navResoconto');

    // Nascondi tutte le sezioni
    dashboardSection.style.display = 'none';
    archiveSection.style.display = 'none';
    settingsSection.style.display = 'none';
    resocontoSection.style.display = 'none';

    // Rimuovi active da tutti i link
    navDashboard.classList.remove('active');
    navArchive.classList.remove('active');
    navSettings.classList.remove('active');
    navResoconto.classList.remove('active');

    if (section === 'dashboard') {
        dashboardSection.style.display = 'block';
        navDashboard.classList.add('active');
    } else if (section === 'archive') {
        archiveSection.style.display = 'block';
        navArchive.classList.add('active');
        loadArchivedProjects();
    } else if (section === 'settings') {
        settingsSection.style.display = 'block';
        navSettings.classList.add('active');
        loadSettingsActivityTypes();
        loadIdleThreshold();
        loadMergeGap();
        loadAutostartStatus();
        loadContractedHours();
        loadWorkingDays();
        loadYearHolidays();
        loadPatronSaintDate();
        loadRecurringHolidays();
        loadTrasfertaLabel();
    } else if (section === 'resoconto') {
        resocontoSection.style.display = 'block';
        navResoconto.classList.add('active');
        initResocontoMonth();
    }
}

// Carica valore tempo di inattività dal backend
async function loadIdleThreshold() {
    try {
        const threshold = await GetIdleThreshold();
        document.getElementById('idleThresholdSetting').value = threshold;
    } catch (error) {
        console.error('Errore caricamento soglia inattività:', error);
        document.getElementById('idleThresholdSetting').value = 5; // Default
    }
}


// === MODAL CONFERMA ELIMINAZIONE ===

window.showConfirmDeleteModal = function(id, type, title, message) {
    document.getElementById('confirmDeleteId').value = id;
    document.getElementById('confirmDeleteType').value = type;
    document.getElementById('confirmDeleteTitle').textContent = title;
    document.getElementById('confirmDeleteMessage').textContent = message;
    document.getElementById('confirmDeleteModal').classList.add('show');
}

window.closeConfirmDeleteModal = function() {
    document.getElementById('confirmDeleteModal').classList.remove('show');
}

window.confirmDelete = async function() {
    const id = parseInt(document.getElementById('confirmDeleteId').value);
    const type = document.getElementById('confirmDeleteType').value;

    try {
        if (type === 'project') {
            await DeleteProject(id);
            showNotification('Progetto eliminato!', 'success');
            closeConfirmDeleteModal();
            await loadArchivedProjects();
        } else if (type === 'session') {
            await DeleteSession(id);
            showNotification('Sessione eliminata!', 'success');
            closeConfirmDeleteModal();
            await loadTimeline();
        }
    } catch (error) {
        console.error('Errore eliminazione:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.importProjectFromJSON = async function() {
    try {
        const result = await ImportProjectJSON();
        if (result) {
            showNotification(result, 'success');
            await loadProjects(); // Ricarica progetti attivi
            await loadArchivedProjects(); // Ricarica anche archiviati
        }
    } catch (error) {
        console.error('Errore importazione:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

// === GESTIONE TIPI ATTIVITÀ (IMPOSTAZIONI) ===

async function loadSettingsActivityTypes() {
    try {
        const types = await GetActivityTypes();
        activityTypes = types || [];
        displaySettingsActivityTypes(activityTypes);
    } catch (error) {
        console.error('Errore caricamento tipi attività:', error);
    }
}

function displaySettingsActivityTypes(types) {
    const container = document.getElementById('activityTypesList');
    if (!container) return;

    if (!types || types.length === 0) {
        container.innerHTML = '<p style="color: #6b7280;">Nessun tipo di attività configurato</p>';
        return;
    }

    const patternNames = { 'solid': 'Solido', 'stripes': 'Strisce', 'dots': 'Puntini' };

    let html = '';
    types.forEach((type) => {
        const colorDesc = type.color_variant > 0 ? 'chiaro' : type.color_variant < 0 ? 'scuro' : 'normale';
        const patternName = patternNames[type.pattern] || type.pattern || 'Solido';
		const escapedName = escapeHtml(type.name);
		const escapedPatternName = escapeHtml(patternName);

        html += `
            <div class="activity-type-item" draggable="true" data-id="${type.id}" data-order="${type.display_order}">
                <div style="display: flex; align-items: center; gap: 15px; flex: 1;">
                    <div class="drag-handle"></div>
                    <div style="flex: 1;">
						<strong style="font-size: 1.1em; color: #E4DACB;">${escapedName}</strong>
                        <p style="color: #9C9086; margin: 5px 0 0 0; font-size: 0.9em;">
							Colore: ${colorDesc} (${type.color_variant.toFixed(1)}) | Pattern: ${escapedPatternName}
                        </p>
                    </div>
                </div>
                <div style="display: flex; gap: 10px;">
                    <button class="btn" style="width: auto; padding: 8px 16px; margin: 0;" data-activity-action="edit" data-activity-id="${Number(type.id)}">Modifica</button>
					<button class="btn btn-danger" style="width: auto; padding: 8px 16px; margin: 0;" data-activity-action="delete" data-activity-id="${Number(type.id)}">Elimina</button>
                </div>
            </div>
        `;
    });

    container.innerHTML = html;
    container.querySelectorAll('[data-activity-action]').forEach(button => {
        button.addEventListener('click', () => {
            const type = types.find(item => Number(item.id) === Number(button.dataset.activityId));
            if (!type) return;

            if (button.dataset.activityAction === 'edit') {
                window.editActivityTypeInSettings(type.id);
            } else {
                window.deleteActivityTypeById(type.id, type.name);
            }
        });
    });
    initializeDragAndDrop();
}

// Inizializza drag and drop
function initializeDragAndDrop() {
    const items = document.querySelectorAll('.activity-type-item');
    let draggedElement = null;

    items.forEach(item => {
        item.addEventListener('dragstart', (e) => {
            draggedElement = item;
            item.classList.add('dragging');
            e.dataTransfer.effectAllowed = 'move';
        });

        item.addEventListener('dragend', () => {
            item.classList.remove('dragging');
            draggedElement = null;
            document.querySelectorAll('.activity-type-item').forEach(i => i.style.borderTop = '');
        });

        item.addEventListener('dragover', (e) => {
            e.preventDefault();
            e.dataTransfer.dropEffect = 'move';
        });

        item.addEventListener('dragenter', (e) => {
            if (item !== draggedElement) {
                item.style.borderTop = '3px solid #C99A54';
            }
        });

        item.addEventListener('dragleave', () => {
            item.style.borderTop = '';
        });

        item.addEventListener('drop', async (e) => {
            e.preventDefault();
            item.style.borderTop = '';

            if (draggedElement && draggedElement !== item) {
                const draggedId = parseInt(draggedElement.getAttribute('data-id'));
                const droppedOnId = parseInt(item.getAttribute('data-id'));

                // Riordina l'array
                const draggedIndex = activityTypes.findIndex(t => t.id === draggedId);
                const droppedIndex = activityTypes.findIndex(t => t.id === droppedOnId);

                const temp = activityTypes[draggedIndex];
                activityTypes.splice(draggedIndex, 1);
                activityTypes.splice(droppedIndex, 0, temp);

                // Aggiorna display_order
                activityTypes.forEach((type, index) => {
                    type.display_order = index + 1;
                });

                // Salva nuovo ordine
                try {
                    const updates = activityTypes.map(t => ({ id: t.id, display_order: t.display_order }));
                    await ReorderActivityTypes(updates);
                    showNotification('Ordine aggiornato!', 'success');
                    displaySettingsActivityTypes(activityTypes);
                } catch (error) {
                    console.error('Errore riordino:', error);
                    showNotification('Errore riordino', 'error');
                    loadSettingsActivityTypes();
                }
            }
        });
    });
}

window.openActivityTypeModal = function() {
    document.getElementById('activityTypeModalTitle').textContent = 'Nuovo Tipo di Attività';
    document.getElementById('activityTypeId').value = '';
    document.getElementById('activityTypeName').value = '';
    document.getElementById('activityTypeColor').value = '0';
    document.getElementById('activityTypePattern').value = 'solid';
    document.getElementById('activityTypeOrder').value = activityTypes.length + 1;
    document.getElementById('colorValue').textContent = '0.0';

    selectPattern('solid');
    updateActivityPreview();

    document.getElementById('activityTypeModal').classList.add('show');
}

window.editActivityTypeInSettings = function(id) {
    const type = activityTypes.find(t => t.id === id);
    if (!type) return;

    document.getElementById('activityTypeModalTitle').textContent = 'Modifica Tipo di Attività';
    document.getElementById('activityTypeId').value = type.id;
    document.getElementById('activityTypeName').value = type.name;
    document.getElementById('activityTypeColor').value = type.color_variant;
    document.getElementById('activityTypePattern').value = type.pattern || 'solid';
    document.getElementById('activityTypeOrder').value = type.display_order;
    document.getElementById('colorValue').textContent = type.color_variant.toFixed(1);

    selectPattern(type.pattern || 'solid');
    updateActivityPreview();

    document.getElementById('activityTypeModal').classList.add('show');
}

// Seleziona pattern
window.selectPattern = function(patternName) {
    document.querySelectorAll('.pattern-option').forEach(opt => opt.classList.remove('selected'));
    const selected = document.querySelector(`.pattern-option[data-pattern="${patternName}"]`);
    if (selected) selected.classList.add('selected');
    document.getElementById('activityTypePattern').value = patternName;
    updateActivityPreview();
}

// Aggiorna anteprima
function updateActivityPreview() {
    const name = document.getElementById('activityTypeName').value.trim() || 'Nome Attività';
    const colorVariant = parseFloat(document.getElementById('activityTypeColor').value);
    const pattern = document.getElementById('activityTypePattern').value;
    const previewElement = document.getElementById('activityPreview');
    const previewName = document.getElementById('previewName');

    if (!previewElement || !previewName) return;

    previewName.textContent = name;

    // Calcola colore
    let r = 255, g = 255, b = 255; // bianco base
    if (colorVariant < 0) {
        r += r * colorVariant;
        g += g * colorVariant;
        b += b * colorVariant;
    }

    const color = `rgb(${Math.round(r)}, ${Math.round(g)}, ${Math.round(b)})`;
    const lightColor = `rgb(${Math.min(255, Math.round(r + 40))}, ${Math.min(255, Math.round(g + 40))}, ${Math.min(255, Math.round(b + 40))})`;

    previewElement.style.backgroundColor = color;
    previewElement.style.backgroundImage = 'none';
    previewElement.style.backgroundSize = 'auto';

    if (pattern === 'stripes') {
        previewElement.style.backgroundImage = `repeating-linear-gradient(45deg, ${color}, ${color} 5px, ${lightColor} 5px, ${lightColor} 10px)`;
    } else if (pattern === 'dots') {
        previewElement.style.backgroundImage = `radial-gradient(circle, ${lightColor} 2px, transparent 2px)`;
        previewElement.style.backgroundSize = '10px 10px';
    }
}

// Event listeners per anteprima live
document.addEventListener('DOMContentLoaded', () => {
    const colorInput = document.getElementById('activityTypeColor');
    if (colorInput) {
        colorInput.addEventListener('input', () => {
            const value = parseFloat(colorInput.value);
            document.getElementById('colorValue').textContent = value.toFixed(1);
            updateActivityPreview();
        });
    }

    const nameInput = document.getElementById('activityTypeName');
    if (nameInput) {
        nameInput.addEventListener('input', updateActivityPreview);
    }
});

window.closeActivityTypeModal = function() {
    document.getElementById('activityTypeModal').classList.remove('show');
}

window.saveActivityType = async function() {
    const id = document.getElementById('activityTypeId').value;
    const name = document.getElementById('activityTypeName').value.trim();
    const colorVariant = parseFloat(document.getElementById('activityTypeColor').value);
    const pattern = document.getElementById('activityTypePattern').value;
    const displayOrder = parseInt(document.getElementById('activityTypeOrder').value);

    if (!name) {
        showNotification('Inserisci un nome', 'error');
        return;
    }

    try {
        if (id) {
            await UpdateActivityType(parseInt(id), name, colorVariant, pattern, displayOrder);
            showNotification('Tipo aggiornato!', 'success');
        } else {
            await CreateActivityType(name, colorVariant, pattern, displayOrder);
            showNotification('Tipo creato!', 'success');
        }
        closeActivityTypeModal();
        await loadSettingsActivityTypes();
        await loadActivityTypes(); // Aggiorna anche la cache globale
    } catch (error) {
        console.error('Errore salvataggio tipo attività:', error);
        showNotification('Errore: ' + (error.message || error), 'error');
    }
}

window.deleteActivityTypeById = async function(id, name) {
    try {
        const archived = await DeleteActivityType(id);
        showNotification(archived
            ? 'Tipo attività archiviato: resta nello storico ma non è più selezionabile.'
            : 'Tipo attività eliminato!', 'success');
        await loadSettingsActivityTypes();
        await loadActivityTypes();
    } catch (error) {
        console.error('Errore eliminazione:', error);
        showNotification('Errore: ' + (error.message || error), 'error');
    }
}

// === UNIONE AUTOMATICA SESSIONI ===

async function loadMergeGap() {
    try {
        document.getElementById('mergeGapSetting').value = await GetMergeGapSeconds();
    } catch (error) {
        console.error('Errore caricamento intervallo di unione:', error);
        document.getElementById('mergeGapSetting').value = 60; // Default
    }
}

window.saveMergeGap = async function() {
    const seconds = parseInt(document.getElementById('mergeGapSetting').value);
    if (Number.isNaN(seconds) || seconds < 0 || seconds > 3600) {
        showNotification('Inserisci un valore tra 0 e 3600 secondi', 'error');
        return;
    }
    try {
        await SetMergeGapSeconds(seconds);
        showNotification('Intervallo di unione salvato!', 'success');
    } catch (error) {
        console.error('Errore salvataggio intervallo di unione:', error);
        showNotification('Errore: ' + (error.message || error), 'error');
    }
}

// === IMPOSTAZIONI IDLE ===

window.saveIdleThreshold = async function() {
    const threshold = parseInt(document.getElementById('idleThresholdSetting').value);
    if (threshold && threshold > 0) {
        try {
            await SetIdleThreshold(threshold);
            showNotification('Tempo di inattività salvato!', 'success');
        } catch (error) {
            console.error('Errore salvataggio soglia:', error);
            showNotification('Errore salvataggio', 'error');
        }
    } else {
        showNotification('Inserisci un valore valido', 'error');
    }
}

// === EXPORT/IMPORT ===

window.exportData = async function() {
    try {
        const data = await ExportData();
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `prenditempo_backup_${new Date().toISOString().split('T')[0]}.json`;
        a.click();
        URL.revokeObjectURL(url);
        showNotification('Dati esportati!', 'success');
    } catch (error) {
        console.error('Errore export:', error);
        showNotification('Errore export: ' + error, 'error');
    }
}

window.importDataFromFile = async function(event) {
    const file = event.target.files[0];
    if (!file) return;
	if (file.size > 150 * 1024 * 1024) {
		showNotification('Il backup supera il limite di 150 MiB', 'error');
		event.target.value = '';
		return;
	}
	if (!window.confirm('L’importazione sostituirà tutti i dati correnti. Verrà creato automaticamente un backup preventivo. Continuare?')) {
		event.target.value = '';
		return;
	}

    try {
        const text = await file.text();
        const data = JSON.parse(text);

        await ImportData(data);
        showNotification('Dati importati con successo!', 'success');
        await loadSettingsActivityTypes();
        await loadActivityTypes();
        await loadProjects();
    } catch (error) {
        console.error('Errore import:', error);
        showNotification('Errore import: ' + error, 'error');
    }

    event.target.value = '';
}

// === AUTOSTART ===

async function loadAutostartStatus() {
    try {
        const enabled = await IsAutoStartEnabled();
        const toggle = document.getElementById('autostartToggle');
        const status = document.getElementById('autostartStatus');

        if (toggle) toggle.checked = enabled;
        if (status) status.textContent = enabled ? 'Abilitato' : 'Disabilitato';
    } catch (error) {
        console.error('Errore verifica autostart:', error);
        const status = document.getElementById('autostartStatus');
        if (status) status.textContent = 'Errore';
    }
}


window.toggleAutostart = async function() {
    const toggle = document.getElementById('autostartToggle');
    const status = document.getElementById('autostartStatus');

    try {
        if (toggle.checked) {
            await EnableAutoStart();
            if (status) status.textContent = 'Abilitato';
            showNotification('Avvio automatico abilitato!', 'success');
        } else {
            await DisableAutoStart();
            if (status) status.textContent = 'Disabilitato';
            showNotification('Avvio automatico disabilitato', 'success');
        }
    } catch (error) {
        console.error('Errore toggle autostart:', error);
        showNotification('Errore: ' + error, 'error');
        // Ripristina lo stato precedente
        toggle.checked = !toggle.checked;
        if (status) status.textContent = toggle.checked ? 'Abilitato' : 'Disabilitato';
    }
}
