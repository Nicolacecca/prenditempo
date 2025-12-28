// Import Wails bindings
import { GetProjects, CreateProject, ArchiveProject, GetProjectReport, GetArchivedProjects, ReactivateProject, DeleteProject } from './wailsjs/go/main/App.js';
import { GetSessions, CreateSession, UpdateSessionDuration, UpdateSessionActivityType, DeleteSession, SplitSession } from './wailsjs/go/main/App.js';
import { GetNotes, GetAllNotes, CreateNote, UpdateNote, DeleteNote } from './wailsjs/go/main/App.js';
import { GetActivityTypes, CreateActivityType, UpdateActivityType, DeleteActivityType, ReorderActivityTypes } from './wailsjs/go/main/App.js';
import { GetTrackingState, StartTracking, StopTracking } from './wailsjs/go/main/App.js';
import { GetTodayStats, GetWeekStats, GetMonthStats } from './wailsjs/go/main/App.js';
import { CheckIdlePeriod, AttributeIdle } from './wailsjs/go/main/App.js';
import { ExportData, ImportData } from './wailsjs/go/main/App.js';
import { SaveReportJSON, SaveReportText, ImportProjectJSON } from './wailsjs/go/main/App.js';
import { IsAutoStartEnabled, EnableAutoStart, DisableAutoStart } from './wailsjs/go/main/App.js';

// Variabili globali
let currentReportProjectId = null;
let isCurrentlyTracking = false;
let activityTypes = [];
let projectsCache = [];

// Inizializzazione
document.addEventListener('DOMContentLoaded', async function() {
    await loadActivityTypes();
    await loadProjects();
    await loadTodayStats();
    await checkTrackingStatus();
    setToday();
    await loadTimeline();
    await loadAllNotes();
    await populateNotesProjectFilter();

    // Aggiorna stato ogni 5 secondi
    setInterval(checkTrackingStatus, 5000);
});

// === TIPI DI ATTIVITÀ ===

async function loadActivityTypes() {
    try {
        activityTypes = await GetActivityTypes();
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

// === PROGETTI ===

async function loadProjects() {
    try {
        projectsCache = await GetProjects();
        displayProjects(projectsCache);
        populateProjectSelect(projectsCache);
    } catch (error) {
        console.error('Errore caricamento progetti:', error);
        showNotification('Errore caricamento progetti', 'error');
    }
}

function displayProjects(projects) {
    const projectList = document.getElementById('projectList');

    if (!projects || projects.length === 0) {
        projectList.innerHTML = '<p style="color: #6b7280;">Nessun progetto. Creane uno nuovo!</p>';
        return;
    }

    projectList.innerHTML = projects.map(project => `
        <div class="project-item">
            <div>
                <h3>${project.name}</h3>
                ${project.description ? `<p>${project.description}</p>` : ''}
            </div>
            <div style="display: flex; gap: 10px;">
                <button class="btn" style="width: 150px; padding: 8px 16px; margin: 0; background: #f59e0b;"
                        onclick="archiveProject(${project.id}, '${project.name.replace(/'/g, "\\'")}')">Chiudi Progetto</button>
            </div>
        </div>
    `).join('');
}

function populateProjectSelect(projects) {
    const select = document.getElementById('projectSelect');
    if (!select) return;

    select.innerHTML = '<option value="">Seleziona progetto...</option>';

    projects.forEach(project => {
        const option = document.createElement('option');
        option.value = project.id;
        option.textContent = project.name;
        select.appendChild(option);
    });
}

window.openCreateProjectModal = function() {
    document.getElementById('createProjectModal').classList.add('show');
}

window.closeCreateProjectModal = function() {
    document.getElementById('createProjectModal').classList.remove('show');
    document.getElementById('modalProjectName').value = '';
    document.getElementById('modalProjectDesc').value = '';
}

window.createProject = async function() {
    const name = document.getElementById('modalProjectName').value.trim();
    const description = document.getElementById('modalProjectDesc').value.trim();

    if (!name) {
        showNotification('Inserisci un nome per il progetto', 'error');
        return;
    }

    try {
        await CreateProject(name, description);
        showNotification('Progetto creato con successo!', 'success');
        closeCreateProjectModal();
        await loadProjects();
    } catch (error) {
        console.error('Errore creazione progetto:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.archiveProject = async function(projectID, projectName) {
    try {
        await ArchiveProject(projectID);
        showNotification('Progetto archiviato con successo!', 'success');

        // Mostra report
        const report = await GetProjectReport(projectID);
        showReportModal(report);

        await loadProjects();
    } catch (error) {
        console.error('Errore archiviazione progetto:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

// === TRACKING ===

window.toggleTracking = async function() {
    if (isCurrentlyTracking) {
        await stopTracking();
    } else {
        await startTracking();
    }
}

async function startTracking() {
    const projectSelect = document.getElementById('projectSelect');
    const projectID = parseInt(projectSelect.value);
    const activityType = document.getElementById('activityTypeSelect').value || null;

    if (!projectID) {
        showNotification('Seleziona un progetto', 'error');
        return;
    }

    try {
        await StartTracking(projectID, activityType);
        const projectName = projectSelect.options[projectSelect.selectedIndex].text;
        showNotification(`Tracking avviato per: ${projectName}`, 'success');
        isCurrentlyTracking = true;
        updateUIForTracking(true);
    } catch (error) {
        console.error('Errore avvio tracking:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

async function stopTracking() {
    try {
        await StopTracking();
        showNotification('Tracking fermato e dati salvati', 'success');
        isCurrentlyTracking = false;
        updateUIForTracking(false);
        await loadTodayStats();
        await loadTimeline();
    } catch (error) {
        console.error('Errore stop tracking:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

async function checkTrackingStatus() {
    try {
        const state = await GetTrackingState();

        if (state.is_tracking) {
            isCurrentlyTracking = true;
            updateUIForTracking(true);
            document.getElementById('statusText').textContent = `Tracking attivo: ${state.project_name || 'Progetto'}`;

            // Mostra tempo trascorso
            const liveStatsDiv = document.getElementById('liveStats');
            const currentStatsDiv = document.getElementById('currentStats');
            if (state.elapsed_seconds > 0) {
                currentStatsDiv.style.display = 'block';
                const minutes = Math.floor(state.elapsed_seconds / 60);
                const seconds = state.elapsed_seconds % 60;
                liveStatsDiv.innerHTML = `
                    <div class="stat-item">
                        <span class="stat-label">Tempo trascorso</span>
                        <span class="stat-value">${minutes} min ${seconds} sec</span>
                    </div>
                `;
            }

            // Controlla se c'è un periodo idle pendente
            await checkForPendingIdle();
        } else {
            isCurrentlyTracking = false;
            updateUIForTracking(false);
            document.getElementById('statusText').textContent = 'Pronto';
        }
    } catch (error) {
        console.error('Errore check status:', error);
    }
}

// === IDLE TIME MANAGEMENT ===

async function checkForPendingIdle() {
    try {
        const idleData = await CheckIdlePeriod();
        if (idleData.has_pending) {
            showIdleModal(idleData);
        }
    } catch (error) {
        console.error('Errore check idle:', error);
    }
}

function showIdleModal(idlePeriod) {
    const modal = document.getElementById('idleModal');
    if (!modal) return;

    // Formatta durata
    const minutes = idlePeriod.minutes;
    const hours = Math.floor(minutes / 60);
    const remainingMins = minutes % 60;
    let durationText = hours > 0 ? `${hours}h ${remainingMins}min` : `${minutes} minuti`;

    document.getElementById('idleDuration').textContent = durationText;
    document.getElementById('idlePeriod').textContent = `${idlePeriod.start_time} - ${idlePeriod.end_time}`;

    // Popola select con progetti
    const idleSelect = document.getElementById('idleProjectSelect');
    idleSelect.innerHTML = '<option value="">Seleziona progetto...</option>';
    projectsCache.forEach(project => {
        const option = document.createElement('option');
        option.value = project.id;
        option.textContent = project.name;
        idleSelect.appendChild(option);
    });

    modal.classList.add('show');
}

window.attributeIdleToProject = async function() {
    const projectID = parseInt(document.getElementById('idleProjectSelect').value);

    if (!projectID) {
        showNotification('Seleziona un progetto', 'error');
        return;
    }

    try {
        await AttributeIdle(projectID, false);
        const projectName = projectsCache.find(p => p.id === projectID)?.name || 'Progetto';
        showNotification(`Tempo idle attribuito a: ${projectName}`, 'success');
        hideIdleModal();
        await loadTodayStats();
        await loadTimeline();
    } catch (error) {
        console.error('Errore attribuzione idle:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.attributeIdleAsBreak = async function() {
    try {
        await AttributeIdle(0, true);
        showNotification('Tempo idle registrato come pausa', 'success');
        hideIdleModal();
    } catch (error) {
        console.error('Errore attribuzione pausa:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

function hideIdleModal() {
    const modal = document.getElementById('idleModal');
    if (modal) modal.classList.remove('show');
}

function updateUIForTracking(isTracking) {
    const trackingBtn = document.getElementById('trackingBtn');
    const indicator = document.getElementById('statusIndicator');

    if (isTracking) {
        trackingBtn.textContent = 'Ferma Tracking';
        trackingBtn.style.background = '#dc2626';
        trackingBtn.style.color = '#ffffff';
        trackingBtn.classList.remove('btn-success');
        indicator.classList.add('active');
    } else {
        trackingBtn.textContent = 'Avvia Tracking';
        trackingBtn.style.background = '';
        trackingBtn.style.color = '';
        trackingBtn.classList.add('btn-success');
        indicator.classList.remove('active');
        document.getElementById('currentStats').style.display = 'none';
    }
}

// === STATISTICHE ===

let currentStatsPeriod = 'today';

async function loadTodayStats() {
    try {
        const stats = await GetTodayStats();
        displayStats(stats, 'Oggi');
    } catch (error) {
        console.error('Errore caricamento statistiche:', error);
    }
}

window.showTodayStats = async function() {
    currentStatsPeriod = 'today';
    updateStatsButtons();
    try {
        const stats = await GetTodayStats();
        displayStats(stats, 'Oggi');
    } catch (error) {
        console.error('Errore caricamento statistiche:', error);
    }
}

window.showWeekStats = async function() {
    currentStatsPeriod = 'week';
    updateStatsButtons();
    try {
        const stats = await GetWeekStats();
        displayStats(stats, 'Questa Settimana');
    } catch (error) {
        console.error('Errore caricamento statistiche:', error);
    }
}

window.showMonthStats = async function() {
    currentStatsPeriod = 'month';
    updateStatsButtons();
    try {
        const stats = await GetMonthStats();
        displayStats(stats, 'Questo Mese');
    } catch (error) {
        console.error('Errore caricamento statistiche:', error);
    }
}

function updateStatsButtons() {
    const todayBtn = document.getElementById('statsToday');
    const weekBtn = document.getElementById('statsWeek');
    const monthBtn = document.getElementById('statsMonth');

    [todayBtn, weekBtn, monthBtn].forEach(btn => {
        if (btn) {
            btn.style.background = '#ffffff';
            btn.style.color = '#1a1a1a';
        }
    });

    if (currentStatsPeriod === 'today' && todayBtn) {
        todayBtn.style.background = '#ff6b2b';
        todayBtn.style.color = '#ffffff';
    } else if (currentStatsPeriod === 'week' && weekBtn) {
        weekBtn.style.background = '#ff6b2b';
        weekBtn.style.color = '#ffffff';
    } else if (currentStatsPeriod === 'month' && monthBtn) {
        monthBtn.style.background = '#ff6b2b';
        monthBtn.style.color = '#ffffff';
    }
}

function displayStats(stats, periodLabel) {
    const statsDiv = document.getElementById('todayStats');

    if (!stats || Object.keys(stats).length === 0) {
        statsDiv.innerHTML = `<p style="color: #6b7280;">Nessuna statistica disponibile per: ${periodLabel}</p>`;
        return;
    }

    const total = Object.values(stats).reduce((sum, sec) => sum + sec, 0);
    const totalMinutes = Math.floor(total / 60);
    const totalHours = (total / 3600).toFixed(1);

    statsDiv.innerHTML = `
        <div class="stat-item">
            <span class="stat-label"><strong>Totale ${periodLabel}</strong></span>
            <span class="stat-value"><strong>${totalMinutes} min (${totalHours}h)</strong></span>
        </div>
        ${Object.entries(stats)
            .sort((a, b) => b[1] - a[1])
            .map(([app, seconds]) => {
                const minutes = Math.floor(seconds / 60);
                const percentage = ((seconds / total) * 100).toFixed(1);
                return `
                    <div class="stat-item">
                        <span class="stat-label">${app}</span>
                        <span class="stat-value">${minutes} min (${percentage}%)</span>
                    </div>
                `;
            }).join('')}
    `;
}

// Legacy function per backward compatibility
function displayTodayStats(stats) {
    displayStats(stats, 'Oggi');
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
        const [sessions, notes] = await Promise.all([
            GetSessions(startDate, endDate),
            GetNotes(startDate, endDate)
        ]);

        displayTimeline(sessions || [], notes || [], startDate, endDate);
    } catch (error) {
        console.error('Errore caricamento timeline:', error);
        showNotification('Errore caricamento timeline', 'error');
    }
}

function displayTimeline(sessions, notes, startDate, endDate) {
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
            html += `<div class="timeline-hour-marker" style="left: ${percentage}%;"><span ${spanStyle}>${String(hour).padStart(2, '0')}:00</span></div>`;
        }
        // Aggiungi marker per 23:59
        html += `<div class="timeline-hour-marker" style="left: 100%;"><span style="position: relative; right: 50%;">23:59</span></div>`;
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
        const projectNotes = notes ? notes.filter(n => n.project_name === projectName) : [];

        // Label del progetto (con background per interrompere le linee dei marker)
        html += `<div style="margin-top: 15px; margin-bottom: 5px; font-weight: 600; color: #ffffff; position: relative; z-index: 200; background: #242424; padding: 5px 0;">`;
        html += `${projectName}`;
        html += `</div>`;

        html += '<div class="timeline-bar">';

        projectSessions.forEach(session => {
            const segment = createTimelineSegment(session, startTime, totalMs);
            html += segment;
        });

        // Indicatori note
        projectNotes.forEach(note => {
            const noteMarker = createNoteMarker(note, startTime, totalMs);
            html += noteMarker;
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
    html += '<h3 style="margin-bottom: 10px; color: #ffffff;">Per Progetto</h3>';
    html += '<div class="stats-grid">';

    for (const [proj, seconds] of Object.entries(projectStats).sort((a, b) => b[1] - a[1])) {
        const hours = (seconds / 3600).toFixed(1);
        const percentage = ((seconds / totalSeconds) * 100).toFixed(1);

        html += `
            <div class="stat-item">
                <span class="stat-label"><strong style="color: #ffffff;">${proj}</strong></span>
                <span class="stat-value">${hours}h (${percentage}%)</span>
            </div>
        `;
    }
    html += '</div></div>';

    // Riepilogo per tipo attività
    html += '<div>';
    html += '<h3 style="margin-bottom: 10px; color: #ffffff;">Per Tipo Attività</h3>';
    html += '<div class="stats-grid">';

    for (const [activity, seconds] of Object.entries(activityStats).sort((a, b) => b[1] - a[1])) {
        const hours = (seconds / 3600).toFixed(1);
        const percentage = ((seconds / totalSeconds) * 100).toFixed(1);

        html += `
            <div class="stat-item">
                <span class="stat-label"><strong style="color: #ffffff;">${activity}</strong></span>
                <span class="stat-value">${hours}h (${percentage}%)</span>
            </div>
        `;
    }
    html += '</div></div>';

    html += '</div>';

    content.innerHTML = html;
}

function createNoteMarker(note, startTime, totalMs) {
    if (!note.timestamp) return '';

    // Parse timestamp trattandolo sempre come ora locale
    let noteTime;
    let displayHours, displayMinutes, displayDay, displayMonth;

    if (note.timestamp.includes('T')) {
        // Formato ISO: estrai i componenti direttamente dalla stringa
        const isoMatch = note.timestamp.match(/(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):?(\d{2})?/);
        if (isoMatch) {
            const [, year, month, day, hours, minutes, seconds] = isoMatch.map(Number);
            noteTime = new Date(year, month - 1, day, hours, minutes, seconds || 0);
            displayHours = hours;
            displayMinutes = minutes;
            displayDay = day;
            displayMonth = month;
        } else {
            noteTime = new Date(note.timestamp);
            displayHours = noteTime.getHours();
            displayMinutes = noteTime.getMinutes();
            displayDay = noteTime.getDate();
            displayMonth = noteTime.getMonth() + 1;
        }
    } else {
        // Formato SQLite: "2025-01-15 14:30:00"
        const parts = note.timestamp.split(' ');
        const datePart = parts[0];
        const timePart = parts[1] || '12:00:00';
        const [year, month, day] = datePart.split('-').map(Number);
        const [hours, minutes, seconds] = timePart.split(':').map(Number);
        noteTime = new Date(year, month - 1, day, hours, minutes, seconds || 0);
        displayHours = hours;
        displayMinutes = minutes;
        displayDay = day;
        displayMonth = month;
    }

    const noteMs = noteTime - startTime;
    const left = (noteMs / totalMs) * 100;

    const timeStr = `${String(displayHours).padStart(2, '0')}:${String(displayMinutes).padStart(2, '0')}`;
    const dateStr = `${displayDay}/${displayMonth}`;

    const noteText = note.note_text || '';
    // Tronca il testo della nota se troppo lungo
    const notePreview = noteText.length > 100
        ? noteText.substring(0, 100) + '...'
        : noteText;

    // Costruisci testo per il tooltip nativo
    const tooltipText = `📝 NOTA\n${dateStr} ${timeStr}\n\n${notePreview}`;

    return `
        <div class="note-marker" style="left: ${left}%;" title="${tooltipText.replace(/"/g, '&quot;')}">
            <!-- Linea verticale -->
            <div class="note-marker-line"></div>

            <!-- Icona nota sopra la timeline -->
            <div class="note-marker-icon" onclick="editNoteFromTimeline(${note.id}, '${noteText.replace(/'/g, "\\'").replace(/\n/g, '\\n')}', '${dateStr} ${timeStr}')">
                📝
            </div>
        </div>
    `;
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
    const tooltipText = `${activityTypeName}\nInizio: ${dateStr} ${timeStr}\nDurata: ${Math.floor(session.seconds / 60)} min`;

    // Trova tipo attività per colore e pattern
    const activityTypeObj = activityTypes.find(t => t.name === activityTypeName);
    let bgStyle = 'background: #ffffff;';

    if (activityTypeObj) {
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
             title="${tooltipText.replace(/"/g, '&quot;')}"
             data-session-id="${session.id}"
             onclick="showSessionMenu(event, ${session.id}, ${session.seconds}, '${activityTypeName}', '${(session.project_name || '').replace(/'/g, "\\'")}')">
        </div>
    `;
}

function formatDate(dateStr) {
    const date = new Date(dateStr);
    return `${date.getDate()}/${date.getMonth() + 1}/${date.getFullYear()}`;
}

// === NOTE ===

window.openNoteModal = async function() {
    const modal = document.getElementById('noteModal');
    const noteSelect = document.getElementById('noteProjectSelect');

    noteSelect.innerHTML = '<option value="">Seleziona progetto...</option>';
    projectsCache.forEach(project => {
        const option = document.createElement('option');
        option.value = project.id;
        option.textContent = project.name;
        noteSelect.appendChild(option);
    });

    document.getElementById('noteText').value = '';
    modal.classList.add('show');
}

window.closeNoteModal = function() {
    document.getElementById('noteModal').classList.remove('show');
}

window.saveNote = async function() {
    const projectID = parseInt(document.getElementById('noteProjectSelect').value);
    const noteText = document.getElementById('noteText').value.trim();

    if (!projectID) {
        showNotification('Seleziona un progetto', 'error');
        return;
    }

    if (!noteText) {
        showNotification('Inserisci il testo della nota', 'error');
        return;
    }

    try {
        const now = new Date();
        const timestamp = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')} ${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}`;

        await CreateNote(projectID, noteText, timestamp);
        showNotification('Nota creata con successo!', 'success');
        closeNoteModal();
        await loadTimeline();
        await loadAllNotes();
    } catch (error) {
        console.error('Errore creazione nota:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.loadAllNotes = async function() {
    try {
        const filterElement = document.getElementById('notesFilterProject');
        const searchElement = document.getElementById('notesSearchText');
        const contentElement = document.getElementById('notesListContent');

        if (!filterElement || !searchElement || !contentElement) return;

        const projectID = filterElement.value || '';
        const searchText = searchElement.value || '';

        const notes = await GetAllNotes(projectID, searchText, '50');
        displayNotesList(notes || []);
    } catch (error) {
        console.error('Errore caricamento note:', error);
    }
}

async function displayNotesList(notes) {
    const container = document.getElementById('notesListContent');

    if (!notes || notes.length === 0) {
        container.innerHTML = '<p style="color: #6b7280;">Nessuna nota trovata</p>';
        return;
    }

    let html = '<div style="display: flex; flex-direction: column; gap: 12px;">';

    notes.forEach(note => {
        const projectName = note.project_name || 'Progetto sconosciuto';
        const timestamp = new Date(note.timestamp);
        const dateStr = timestamp.toLocaleDateString('it-IT');
        const timeStr = timestamp.toLocaleTimeString('it-IT', { hour: '2-digit', minute: '2-digit' });

        html += `
            <div style="background: #1a1a1a; padding: 15px; border-radius: 8px; border-left: 4px solid #ff6b2b; border: 1px solid #3a3a3a;">
                <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 10px;">
                    <div>
                        <strong style="color: #ff6b2b; font-size: 1.05em;">${projectName}</strong>
                        <span style="color: #999999; font-size: 0.85em; margin-left: 10px;">${dateStr} ${timeStr}</span>
                    </div>
                    <div style="display: flex; gap: 8px;">
                        <button onclick="editNoteFromList(${note.id}, '${note.note_text.replace(/'/g, "\\'")}', ${note.project_id})"
                                style="padding: 5px 12px; background: #ff6b2b; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 0.85em;">
                            Modifica
                        </button>
                        <button onclick="deleteNoteFromList(${note.id})"
                                style="padding: 5px 12px; background: #ef4444; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 0.85em;">
                            Elimina
                        </button>
                    </div>
                </div>
                <p style="color: #ffffff; margin: 0; white-space: pre-wrap;">${note.note_text}</p>
            </div>
        `;
    });

    html += '</div>';
    container.innerHTML = html;
}

async function populateNotesProjectFilter() {
    try {
        const filterSelect = document.getElementById('notesFilterProject');
        if (!filterSelect) return;

        filterSelect.innerHTML = '<option value="">Tutti i progetti</option>';
        projectsCache.forEach(project => {
            filterSelect.innerHTML += `<option value="${project.id}">${project.name}</option>`;
        });
    } catch (error) {
        console.error('Errore caricamento progetti per filtro note:', error);
    }
}

window.editNoteFromList = function(noteID, noteText, projectID) {
    document.getElementById('editNoteId').value = noteID;
    document.getElementById('editNoteText').value = noteText;
    document.getElementById('editNoteModal').classList.add('show');
}

window.editNoteFromTimeline = function(noteID, noteText, timestampStr) {
    document.getElementById('editNoteId').value = noteID;
    document.getElementById('editNoteText').value = noteText;
    const timestampEl = document.getElementById('editNoteTimestamp');
    if (timestampEl) {
        timestampEl.textContent = timestampStr;
    }
    document.getElementById('editNoteModal').classList.add('show');
}

window.closeEditNoteModal = function() {
    document.getElementById('editNoteModal').classList.remove('show');
}

window.updateNote = async function() {
    const noteId = parseInt(document.getElementById('editNoteId').value);
    const noteText = document.getElementById('editNoteText').value.trim();

    if (!noteText) {
        showNotification('Inserisci il testo della nota', 'error');
        return;
    }

    try {
        await UpdateNote(noteId, noteText);
        showNotification('Nota aggiornata!', 'success');
        closeEditNoteModal();
        await loadTimeline();
        await loadAllNotes();
    } catch (error) {
        console.error('Errore aggiornamento nota:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.deleteNoteFromModal = async function() {
    const noteId = parseInt(document.getElementById('editNoteId').value);

    try {
        await DeleteNote(noteId);
        showNotification('Nota eliminata!', 'success');
        closeEditNoteModal();
        await loadTimeline();
        await loadAllNotes();
    } catch (error) {
        console.error('Errore eliminazione nota:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.deleteNoteFromList = async function(noteID) {
    try {
        await DeleteNote(noteID);
        showNotification('Nota eliminata!', 'success');
        await loadAllNotes();
    } catch (error) {
        console.error('Errore eliminazione nota:', error);
        showNotification('Errore eliminazione nota', 'error');
    }
}

// === SESSIONI ===

window.createNewSession = async function() {
    const projectRadioList = document.getElementById('projectRadioList');
    let projectsHtml = '<div style="display: flex; flex-direction: column; gap: 8px;">';
    projectsCache.forEach((project, index) => {
        const checked = index === 0 ? 'checked' : '';
        projectsHtml += `
            <label style="display: flex; justify-content: space-between; align-items: center; cursor: pointer; padding: 12px 15px; border: 1px solid #4a4a4a; border-radius: 6px; transition: all 0.2s; background: #1a1a1a;">
                <span style="color: #ffffff; font-weight: 500;">${project.name}</span>
                <input type="radio" name="projectSelect" value="${project.id}" ${checked} style="width: 18px; height: 18px; cursor: pointer; margin: 0;">
            </label>
        `;
    });
    projectsHtml += '</div>';
    projectRadioList.innerHTML = projectsHtml;

    // Popola select tipo attività
    const activitySelect = document.getElementById('sessionActivityType');
    activitySelect.innerHTML = '<option value="">Nessuno</option>';
    activityTypes.forEach(type => {
        activitySelect.innerHTML += `<option value="${type.name}">${type.name}</option>`;
    });

    // Popola ore e minuti
    const hourSelect = document.getElementById('sessionHour');
    hourSelect.innerHTML = '';
    for (let h = 0; h < 24; h++) {
        hourSelect.innerHTML += `<option value="${h}">${String(h).padStart(2, '0')}</option>`;
    }

    const minuteSelect = document.getElementById('sessionMinute');
    minuteSelect.innerHTML = '';
    for (let m = 0; m < 60; m++) {
        minuteSelect.innerHTML += `<option value="${m}">${String(m).padStart(2, '0')}</option>`;
    }

    // Imposta data/ora corrente
    const now = new Date();
    document.getElementById('sessionDate').value = now.toISOString().split('T')[0];
    hourSelect.value = now.getHours();
    minuteSelect.value = now.getMinutes();

    document.getElementById('createSessionModal').classList.add('show');
}

window.closeCreateSessionModal = function() {
    document.getElementById('createSessionModal').classList.remove('show');
}

window.saveNewSession = async function() {
    const projectRadio = document.querySelector('input[name="projectSelect"]:checked');
    if (!projectRadio) {
        showNotification('Seleziona un progetto', 'error');
        return;
    }
    const projectId = parseInt(projectRadio.value);

    const durationMinutes = parseInt(document.getElementById('sessionDuration').value);
    if (isNaN(durationMinutes) || durationMinutes <= 0) {
        showNotification('Durata non valida', 'error');
        return;
    }
    const seconds = durationMinutes * 60;

    const dateValue = document.getElementById('sessionDate').value;
    const hourValue = document.getElementById('sessionHour').value;
    const minuteValue = document.getElementById('sessionMinute').value;
    if (!dateValue) {
        showNotification('Inserisci la data', 'error');
        return;
    }

    const timestamp = `${dateValue} ${String(hourValue).padStart(2, '0')}:${String(minuteValue).padStart(2, '0')}:00`;
    const activityType = document.getElementById('sessionActivityType').value || null;

    try {
        await CreateSession('Manuale', seconds, projectId, 'computer', activityType, timestamp);
        showNotification('Nuova sessione creata!', 'success');
        closeCreateSessionModal();
        await loadTimeline();
    } catch (error) {
        console.error('Errore creazione sessione:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.showSessionMenu = function(event, sessionID, seconds, activityType, projectName) {
    event.stopPropagation();

    // Rimuovi menu esistente
    const existingMenu = document.getElementById('contextMenu');
    if (existingMenu) existingMenu.remove();

    const menuDiv = document.createElement('div');
    menuDiv.id = 'contextMenu';
    menuDiv.style.cssText = `position: fixed; left: ${event.clientX}px; top: ${event.clientY}px; background: white; border: 1px solid #ccc; border-radius: 4px; box-shadow: 0 4px 20px rgba(0,0,0,0.3); z-index: 999999; min-width: 200px; padding: 0;`;

    menuDiv.innerHTML = `
        <div style="padding: 8px; border-bottom: 1px solid #eee; background: #f5f5f5; font-weight: bold;">${projectName}</div>
        <div style="padding: 4px 0;">
            <button onclick="editActivityType(${sessionID}, '${activityType}', '${projectName}')" style="width: 100%; padding: 8px 12px; border: none; background: none; text-align: left; cursor: pointer;">Modifica tipo attività</button>
            <button onclick="editSessionDuration(${sessionID}, ${seconds}, '${projectName}')" style="width: 100%; padding: 8px 12px; border: none; background: none; text-align: left; cursor: pointer;">Modifica durata</button>
            <button onclick="splitSessionDialog(${sessionID}, ${seconds}, '${activityType}', '${projectName}')" style="width: 100%; padding: 8px 12px; border: none; background: none; text-align: left; cursor: pointer;">Dividi sessione</button>
            <hr style="margin: 4px 0; border: none; border-top: 1px solid #eee;">
            <button onclick="deleteSession(${sessionID}, '${projectName}')" style="width: 100%; padding: 8px 12px; border: none; background: none; text-align: left; cursor: pointer; color: #d32f2f;">Elimina sessione</button>
        </div>
    `;

    menuDiv.onclick = (e) => e.stopPropagation();
    document.body.appendChild(menuDiv);

    setTimeout(() => {
        document.addEventListener('click', () => {
            const menu = document.getElementById('contextMenu');
            if (menu) menu.remove();
        }, { once: true });
    }, 100);
}

// === SPLIT SESSION ===

window.splitSessionDialog = async function(sessionID, totalSeconds, currentActivityType, projectName) {
    // Rimuovi menu contestuale
    const menu = document.getElementById('contextMenu');
    if (menu) menu.remove();

    const totalMinutes = Math.floor(totalSeconds / 60);

    const input = prompt(
        `Dividi sessione: ${projectName}\n\nDurata totale: ${totalMinutes} min\n\nInserisci durata PRIMA PARTE in minuti:`,
        Math.floor(totalMinutes / 2)
    );

    if (input === null) return;

    const firstPartMinutes = parseInt(input);
    if (isNaN(firstPartMinutes) || firstPartMinutes <= 0 || firstPartMinutes >= totalMinutes) {
        showNotification('Durata non valida', 'error');
        return;
    }

    const firstPartSeconds = firstPartMinutes * 60;
    const secondPartMinutes = totalMinutes - firstPartMinutes;

    // Chiedi i tipi di attività per entrambe le parti
    const activities = ['', ...activityTypes.map(t => t.name)];
    const activityOptions = activities.map((a, i) => `${i}. ${a || 'Nessuno'}`).join('\n');

    const firstActivityChoice = prompt(
        `Tipo attività PRIMA PARTE (${firstPartMinutes} min):\n\n${activityOptions}\n\nInserisci numero:`,
        currentActivityType ? activities.indexOf(currentActivityType).toString() : '0'
    );

    if (firstActivityChoice === null) return;

    const firstActivityIndex = parseInt(firstActivityChoice);
    if (isNaN(firstActivityIndex) || firstActivityIndex < 0 || firstActivityIndex >= activities.length) {
        showNotification('Scelta non valida', 'error');
        return;
    }

    const secondActivityChoice = prompt(
        `Tipo attività SECONDA PARTE (${secondPartMinutes} min):\n\n${activityOptions}\n\nInserisci numero:`,
        '0'
    );

    if (secondActivityChoice === null) return;

    const secondActivityIndex = parseInt(secondActivityChoice);
    if (isNaN(secondActivityIndex) || secondActivityIndex < 0 || secondActivityIndex >= activities.length) {
        showNotification('Scelta non valida', 'error');
        return;
    }

    const firstActivityType = activities[firstActivityIndex] || null;
    const secondActivityType = activities[secondActivityIndex] || null;

    try {
        await SplitSession(sessionID, firstPartSeconds, firstActivityType, secondActivityType);
        showNotification('Sessione divisa!', 'success');
        await loadTimeline();
    } catch (error) {
        console.error('Errore divisione sessione:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.editActivityType = async function(sessionID, currentActivityType, projectName) {
    const activities = ['', ...activityTypes.map(t => t.name)];
    const options = activities.map((a, i) => `${i}. ${a || 'Nessuno'}`).join('\n');
    const choice = prompt(`Modifica tipo attività per: ${projectName}\n\nAttuale: ${currentActivityType || 'Non specificato'}\n\nSeleziona nuovo tipo:\n\n${options}\n\nInserisci numero:`);

    if (choice === null) return;

    const index = parseInt(choice);
    if (isNaN(index) || index < 0 || index >= activities.length) {
        showNotification('Scelta non valida', 'error');
        return;
    }

    const newActivityType = activities[index] || null;

    try {
        await UpdateSessionActivityType(sessionID, newActivityType);
        showNotification('Tipo attività aggiornato!', 'success');
        await loadTimeline();
    } catch (error) {
        console.error('Errore aggiornamento tipo attività:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.editSessionDuration = async function(sessionID, currentSeconds, projectName) {
    const currentMinutes = Math.floor(currentSeconds / 60);
    const input = prompt(`Modifica durata per: ${projectName}\n\nDurata attuale: ${currentMinutes} min\n\nInserisci nuova durata in minuti:`, currentMinutes);

    if (input === null) return;

    const newMinutes = parseInt(input);
    if (isNaN(newMinutes) || newMinutes <= 0) {
        showNotification('Durata non valida', 'error');
        return;
    }

    try {
        await UpdateSessionDuration(sessionID, newMinutes * 60);
        showNotification('Durata aggiornata!', 'success');
        await loadTimeline();
    } catch (error) {
        console.error('Errore aggiornamento durata:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.deleteSession = async function(sessionID, projectName) {
    try {
        await DeleteSession(sessionID);
        showNotification('Sessione eliminata!', 'success');
        await loadTimeline();
    } catch (error) {
        console.error('Errore eliminazione sessione:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

// === REPORT ===

function showReportModal(report) {
    const modal = document.getElementById('reportModal');
    const content = document.getElementById('reportContent');

    // Salva l'ID del progetto per l'esportazione
    currentReportProjectId = report.project_id;

    // Gestisci date in modo sicuro
    let startDate = 'N/A';
    let endDate = 'N/A';
    let closedAt = 'N/A';

    if (report.start_date && report.start_date !== '') {
        try {
            startDate = new Date(report.start_date).toLocaleDateString('it-IT');
        } catch (e) {
            startDate = report.start_date;
        }
    }

    if (report.end_date && report.end_date !== '') {
        try {
            endDate = new Date(report.end_date).toLocaleDateString('it-IT');
        } catch (e) {
            endDate = report.end_date;
        }
    }

    if (report.closed_at && report.closed_at !== '') {
        try {
            closedAt = new Date(report.closed_at).toLocaleString('it-IT');
        } catch (e) {
            closedAt = report.closed_at;
        }
    }

    // Gestisci total_hours in modo sicuro
    const totalHours = (typeof report.total_hours === 'number') ? report.total_hours.toFixed(2) : '0.00';

    let activityBreakdownHTML = '';
    if (report.activity_breakdown && Object.keys(report.activity_breakdown).length > 0) {
        activityBreakdownHTML = Object.entries(report.activity_breakdown)
            .map(([activity, hours]) => {
                const hoursFormatted = (typeof hours === 'number') ? hours.toFixed(2) : '0.00';
                return `
                <div style="display: flex; justify-content: space-between; padding: 12px; background: #1a1a1a; border-radius: 8px; margin-bottom: 8px; border: 1px solid #3a3a3a;">
                    <span style="color: #ffffff;"><strong>${activity}</strong></span>
                    <span style="color: #ff6b2b; font-weight: 600;">${hoursFormatted} ore</span>
                </div>
            `;
            }).join('');
    }

    content.innerHTML = `
        <div style="background: #1a1a1a; padding: 20px; border-radius: 8px; margin-bottom: 15px; border: 1px solid #3a3a3a;">
            <h3 style="color: #ff6b2b; margin-bottom: 10px;">${report.project_name || 'Progetto'}</h3>
            <p style="color: #999999;">${report.project_description || 'Nessuna descrizione'}</p>
        </div>
        <div style="margin-bottom: 20px;">
            <h3 style="color: #ffffff; margin-bottom: 10px;">Periodo</h3>
            <p style="color: #999999;"><strong style="color: #ffffff;">Data inizio:</strong> ${startDate}</p>
            <p style="color: #999999;"><strong style="color: #ffffff;">Data fine:</strong> ${endDate}</p>
            <p style="color: #999999;"><strong style="color: #ffffff;">Chiuso il:</strong> ${closedAt}</p>
        </div>
        <div style="margin-bottom: 20px;">
            <h3 style="color: #ffffff; margin-bottom: 10px;">Totale Ore Tracciate</h3>
            <p style="font-size: 2em; font-weight: bold; color: #ff6b2b;">${totalHours} ore</p>
        </div>
        <div style="margin-bottom: 20px;">
            <h3 style="color: #ffffff; margin-bottom: 10px;">Suddivisione per Tipo di Attività</h3>
            ${activityBreakdownHTML || '<p style="color: #999999;">Nessun dato disponibile</p>'}
        </div>
        <div style="border-top: 1px solid #3a3a3a; padding-top: 15px; margin-top: 10px;">
            <h3 style="color: #ffffff; margin-bottom: 10px;">Esporta Report</h3>
            <div style="display: flex; gap: 10px;">
                <button class="btn" style="flex: 1; padding: 10px; margin: 0;" onclick="exportReportJSON()">
                    Salva JSON (backup)
                </button>
                <button class="btn" style="flex: 1; padding: 10px; margin: 0; background: #10b981;" onclick="exportReportText()">
                    Salva TXT
                </button>
                <button class="btn" style="flex: 1; padding: 10px; margin: 0; background: #3b82f6;" onclick="printReport()">
                    Stampa / PDF
                </button>
            </div>
        </div>
    `;

    modal.classList.add('show');
}

window.closeReportModal = function() {
    document.getElementById('reportModal').classList.remove('show');
    currentReportProjectId = null;
}

// === ESPORTAZIONE REPORT ===

window.exportReportJSON = async function() {
    if (!currentReportProjectId) {
        showNotification('Errore: ID progetto non disponibile', 'error');
        return;
    }

    try {
        const filePath = await SaveReportJSON(currentReportProjectId);
        if (filePath) {
            showNotification('Report JSON salvato!', 'success');
        }
    } catch (error) {
        console.error('Errore salvataggio JSON:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.exportReportText = async function() {
    if (!currentReportProjectId) {
        showNotification('Errore: ID progetto non disponibile', 'error');
        return;
    }

    try {
        const filePath = await SaveReportText(currentReportProjectId);
        if (filePath) {
            showNotification('Report salvato!', 'success');
        }
    } catch (error) {
        console.error('Errore salvataggio:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.printReport = function() {
    // Crea una finestra di stampa con il contenuto del report
    const reportContent = document.getElementById('reportContent').innerHTML;
    const printWindow = window.open('', '_blank');
    printWindow.document.write(`
        <!DOCTYPE html>
        <html>
        <head>
            <title>Report Progetto - PrendiTempo</title>
            <style>
                body {
                    font-family: Arial, sans-serif;
                    padding: 20px;
                    background: white;
                    color: black;
                }
                h3 { color: #ff6b2b; margin-bottom: 10px; }
                p { margin: 5px 0; }
                div { margin-bottom: 15px; }
                .activity-row {
                    display: flex;
                    justify-content: space-between;
                    padding: 8px;
                    background: #f5f5f5;
                    border-radius: 4px;
                    margin-bottom: 5px;
                }
                @media print {
                    body { padding: 0; }
                }
            </style>
        </head>
        <body>
            <h1 style="color: #ff6b2b;">PrendiTempo - Report Progetto</h1>
            ${reportContent}
            <p style="margin-top: 30px; color: #666; font-size: 0.9em;">
                Generato il ${new Date().toLocaleString('it-IT')}
            </p>
        </body>
        </html>
    `);
    printWindow.document.close();
    printWindow.print();
}

// === NOTIFICHE ===

function showNotification(message, type = 'success') {
    const notification = document.getElementById('notification');
    const notificationText = document.getElementById('notificationText');

    notificationText.textContent = message;
    notification.className = `notification ${type} show`;

    setTimeout(() => {
        notification.classList.remove('show');
    }, 3000);
}

// === NAVIGAZIONE SEZIONI ===

window.showSection = function(section) {
    const dashboardSection = document.getElementById('dashboardSection');
    const archiveSection = document.getElementById('archiveSection');
    const settingsSection = document.getElementById('settingsSection');
    const navDashboard = document.getElementById('navDashboard');
    const navArchive = document.getElementById('navArchive');
    const navSettings = document.getElementById('navSettings');

    // Nascondi tutte le sezioni
    dashboardSection.style.display = 'none';
    archiveSection.style.display = 'none';
    settingsSection.style.display = 'none';

    // Rimuovi active da tutti i link
    navDashboard.classList.remove('active');
    navArchive.classList.remove('active');
    navSettings.classList.remove('active');

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
        loadAutostartStatus();
    }
}

// Carica valore tempo di inattività da localStorage
function loadIdleThreshold() {
    const saved = localStorage.getItem('idleThreshold');
    if (saved) {
        document.getElementById('idleThresholdSetting').value = saved;
    }
}

// === GESTIONE ARCHIVIO ===

async function loadArchivedProjects() {
    try {
        const projects = await GetArchivedProjects();
        displayArchivedProjects(projects || []);
    } catch (error) {
        console.error('Errore caricamento progetti archiviati:', error);
        showNotification('Errore caricamento archivio', 'error');
    }
}

function displayArchivedProjects(projects) {
    const container = document.getElementById('archivedProjectsList');

    if (!projects || projects.length === 0) {
        container.innerHTML = '<p style="color: #6b7280;">Nessun progetto archiviato</p>';
        return;
    }

    container.innerHTML = projects.map(project => {
        let closedAtStr = '';
        if (project.closed_at && project.closed_at !== '') {
            try {
                closedAtStr = new Date(project.closed_at).toLocaleDateString('it-IT');
            } catch (e) {
                closedAtStr = project.closed_at;
            }
        }

        return `
            <div class="project-item" style="background: #1a1a1a; padding: 15px; border-radius: 16px; margin-bottom: 10px; display: flex; justify-content: space-between; align-items: center; border: 1px solid #3a3a3a;">
                <div>
                    <h3 style="color: #ffffff; font-size: 1.1em;">${project.name}</h3>
                    ${project.description ? `<p style="color: #999999; font-size: 0.9em; margin-top: 5px;">${project.description}</p>` : ''}
                    ${closedAtStr ? `<p style="color: #666; font-size: 0.8em; margin-top: 5px;">Chiuso il: ${closedAtStr}</p>` : ''}
                </div>
                <div style="display: flex; gap: 10px;">
                    <button class="btn" style="padding: 8px 16px; margin: 0;" onclick="viewArchivedReport(${project.id})">Report</button>
                    <button class="btn btn-success" style="padding: 8px 16px; margin: 0;" onclick="reactivateArchivedProject(${project.id}, '${project.name.replace(/'/g, "\\'")}')">Riattiva</button>
                    <button class="btn" style="padding: 8px 16px; margin: 0; background: #ef4444; color: white;" onclick="deleteArchivedProject(${project.id}, '${project.name.replace(/'/g, "\\'")}')">Elimina</button>
                </div>
            </div>
        `;
    }).join('');
}

window.viewArchivedReport = async function(projectID) {
    try {
        const report = await GetProjectReport(projectID);
        showReportModal(report);
    } catch (error) {
        console.error('Errore caricamento report:', error);
        showNotification('Errore caricamento report', 'error');
    }
}

window.reactivateArchivedProject = async function(projectID, projectName) {
    try {
        await ReactivateProject(projectID);
        showNotification('Progetto riattivato!', 'success');
        await loadArchivedProjects();
        await loadProjects(); // Ricarica anche i progetti attivi
    } catch (error) {
        console.error('Errore riattivazione:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.deleteArchivedProject = async function(projectID, projectName) {
    const message = `Eliminare definitivamente "${projectName}"?\n\n` +
        `Si consiglia di esportare prima un backup del progetto in formato JSON ` +
        `per poterlo eventualmente reimportare in futuro.\n\n` +
        `Questa azione è irreversibile.`;

    if (!confirm(message)) return;

    try {
        await DeleteProject(projectID);
        showNotification('Progetto eliminato!', 'success');
        await loadArchivedProjects();
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

        html += `
            <div class="activity-type-item" draggable="true" data-id="${type.id}" data-order="${type.display_order}">
                <div style="display: flex; align-items: center; gap: 15px; flex: 1;">
                    <div class="drag-handle"></div>
                    <div style="flex: 1;">
                        <strong style="font-size: 1.1em; color: #ffffff;">${type.name}</strong>
                        <p style="color: #999999; margin: 5px 0 0 0; font-size: 0.9em;">
                            Colore: ${colorDesc} (${type.color_variant.toFixed(1)}) | Pattern: ${patternName}
                        </p>
                    </div>
                </div>
                <div style="display: flex; gap: 10px;">
                    <button class="btn" style="width: auto; padding: 8px 16px; margin: 0;" onclick="editActivityTypeInSettings(${type.id})">Modifica</button>
                    <button class="btn" style="width: auto; padding: 8px 16px; margin: 0; background: #ef4444; color: white;" onclick="deleteActivityTypeById(${type.id}, '${type.name.replace(/'/g, "\\'")}')">Elimina</button>
                </div>
            </div>
        `;
    });

    container.innerHTML = html;
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
                item.style.borderTop = '3px solid #ff6b2b';
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
        await DeleteActivityType(id);
        showNotification('Tipo attività eliminato!', 'success');
        await loadSettingsActivityTypes();
        await loadActivityTypes();
    } catch (error) {
        console.error('Errore eliminazione:', error);
        showNotification('Errore: ' + (error.message || error), 'error');
    }
}

// === IMPOSTAZIONI IDLE ===

window.saveIdleThreshold = function() {
    const threshold = document.getElementById('idleThresholdSetting').value;
    if (threshold && threshold > 0) {
        localStorage.setItem('idleThreshold', threshold);
        showNotification('Tempo di inattività salvato!', 'success');
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
