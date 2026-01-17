// Import Wails bindings
import { GetProjects, CreateProject, ArchiveProject, GetProjectReport, GetArchivedProjects, ReactivateProject, DeleteProject, UpdateProject } from './wailsjs/go/main/App.js';
import { GetSessions, CreateSession, UpdateSessionDuration, UpdateSessionActivityType, DeleteSession, SplitSession } from './wailsjs/go/main/App.js';
import { UpdateProjectNote, MigrateLegacyNotes } from './wailsjs/go/main/App.js';
import { GetActivityTypes, CreateActivityType, UpdateActivityType, DeleteActivityType, ReorderActivityTypes } from './wailsjs/go/main/App.js';
import { GetTrackingState, StartTracking, StopTracking } from './wailsjs/go/main/App.js';
import { CheckIdlePeriod, AttributeIdle } from './wailsjs/go/main/App.js';
import { ExportData, ImportData } from './wailsjs/go/main/App.js';
import { SaveReportJSON, SaveReportText, ImportProjectJSON } from './wailsjs/go/main/App.js';
import { IsAutoStartEnabled, EnableAutoStart, DisableAutoStart } from './wailsjs/go/main/App.js';
import { SetIdleThreshold, GetIdleThreshold, BringWindowToFront, RestoreNormalWindow } from './wailsjs/go/main/App.js';
import { UpdateSessionComplete, GetSessionById } from './wailsjs/go/main/App.js';
import { EventsOn } from './wailsjs/runtime/runtime.js';

// === UTILITY FUNCTIONS ===

// Escape HTML to prevent XSS
function escapeHtml(text) {
    if (text === null || text === undefined) return '';
    const div = document.createElement('div');
    div.textContent = String(text);
    return div.innerHTML;
}

// Escape string for use in JavaScript strings (onclick handlers)
function escapeJs(text) {
    if (text === null || text === undefined) return '';
    return String(text)
        .replace(/\\/g, '\\\\')
        .replace(/'/g, "\\'")
        .replace(/"/g, '\\"')
        .replace(/\n/g, '\\n')
        .replace(/\r/g, '\\r')
        .replace(/</g, '\\x3c')
        .replace(/>/g, '\\x3e');
}

// Validate and truncate string length
function validateLength(text, maxLength, fieldName) {
    if (text && text.length > maxLength) {
        throw new Error(`${fieldName} non può superare ${maxLength} caratteri`);
    }
    return text;
}

// Constants for validation
const MAX_PROJECT_NAME_LENGTH = 100;
const MAX_PROJECT_DESC_LENGTH = 500;

// Variabili globali
let currentReportProjectId = null;
let isCurrentlyTracking = false;
let activityTypes = [];
let projectsCache = [];
let statusCheckInProgress = false; // Debounce flag for status check

// Inizializzazione
document.addEventListener('DOMContentLoaded', async function() {
    await loadActivityTypes();
    await loadProjects();
    await checkTrackingStatus();
    setToday(); // setToday già chiama loadTimeline()

    // Aggiorna stato ogni 2 secondi (ridotto per mostrare il modale idle più velocemente)
    setInterval(checkTrackingStatus, 2000);

    // Ascolta evento auto-stop per inattività
    EventsOn('tracking-auto-stopped', async (data) => {
        console.log('[EVENT] Tracking auto-stopped:', data);

        // Aggiorna stato UI
        isCurrentlyTracking = false;
        updateUIForTracking(false);
        document.getElementById('statusText').textContent = 'Pronto';

        // Mostra notifica
        const minutes = Math.floor((data.seconds || 0) / 60);
        showNotification(`Tracking fermato automaticamente (${minutes} min salvati)`, 'success');

        // Ricarica timeline
        await loadTimeline();
    });
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

    projectList.innerHTML = projects.map(project => {
        const escapedName = escapeJs(project.name);
        const escapedDesc = escapeJs(project.description || '');
        const escapedNoteText = escapeJs(project.note_text || '');
        const hasNote = project.note_text && project.note_text.trim().length > 0;
        return `
        <div class="project-item">
            <div>
                <h3>${escapeHtml(project.name)}</h3>
                ${project.description ? `<p>${escapeHtml(project.description)}</p>` : ''}
            </div>
            <div style="display: flex; gap: 10px;">
                <button class="btn" style="width: auto; padding: 8px 16px; margin: 0; background: ${hasNote ? '#10b981' : '#6b7280'};"
                        onclick="openProjectNoteModal(${project.id}, '${escapedName}', '${escapedNoteText}')"
                        title="${hasNote ? 'Modifica note' : 'Aggiungi note'}">📝 Note</button>
                <button class="btn" style="width: auto; padding: 8px 16px; margin: 0; background: #3b82f6;"
                        onclick="openEditProjectModal(${project.id}, '${escapedName}', '${escapedDesc}')">Modifica</button>
            </div>
        </div>
    `;
    }).join('');
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

    // Validate input lengths
    try {
        validateLength(name, MAX_PROJECT_NAME_LENGTH, 'Nome progetto');
        validateLength(description, MAX_PROJECT_DESC_LENGTH, 'Descrizione');
    } catch (error) {
        showNotification(error.message, 'error');
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

// === MODIFICA PROGETTO ===

window.openEditProjectModal = function(projectID, name, description) {
    document.getElementById('editProjectId').value = projectID;
    document.getElementById('editProjectName').value = name;
    document.getElementById('editProjectDesc').value = description || '';
    document.getElementById('editProjectModal').classList.add('show');
}

window.closeEditProjectModal = function() {
    document.getElementById('editProjectModal').classList.remove('show');
    document.getElementById('editProjectId').value = '';
    document.getElementById('editProjectName').value = '';
    document.getElementById('editProjectDesc').value = '';
}

window.updateProject = async function() {
    const projectID = parseInt(document.getElementById('editProjectId').value);
    const name = document.getElementById('editProjectName').value.trim();
    const description = document.getElementById('editProjectDesc').value.trim();

    if (!name) {
        showNotification('Inserisci un nome per il progetto', 'error');
        return;
    }

    // Validate input lengths
    try {
        validateLength(name, MAX_PROJECT_NAME_LENGTH, 'Nome progetto');
        validateLength(description, MAX_PROJECT_DESC_LENGTH, 'Descrizione');
    } catch (error) {
        showNotification(error.message, 'error');
        return;
    }

    try {
        await UpdateProject(projectID, name, description);
        showNotification('Progetto aggiornato con successo!', 'success');
        closeEditProjectModal();
        await loadProjects();
    } catch (error) {
        console.error('Errore aggiornamento progetto:', error);
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

window.archiveProjectFromModal = async function() {
    const projectID = parseInt(document.getElementById('editProjectId').value);
    const projectName = document.getElementById('editProjectName').value;

    closeEditProjectModal();
    await archiveProject(projectID, projectName);
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
        await loadTimeline();
    } catch (error) {
        console.error('Errore stop tracking:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

async function checkTrackingStatus() {
    // Debounce: skip if a check is already in progress
    if (statusCheckInProgress) {
        return;
    }

    statusCheckInProgress = true;
    try {
        const state = await GetTrackingState();

        if (state.is_tracking) {
            isCurrentlyTracking = true;
            updateUIForTracking(true);
            document.getElementById('statusText').textContent = `Tracking attivo: ${escapeHtml(state.project_name || 'Progetto')}`;

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

            // Controlla anche se c'è un periodo idle pendente (dopo auto-stop)
            await checkForPendingIdle();
        }
    } catch (error) {
        console.error('Errore check status:', error);
    } finally {
        statusCheckInProgress = false;
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

    // Se il modale è già visibile, non ripopolare il dropdown (evita di cancellare la selezione utente)
    if (modal.classList.contains('show')) {
        return;
    }

    // Porta la finestra in primo piano per mostrare il modal all'utente
    BringWindowToFront();

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
    // Rimuovi always on top quando l'utente ha interagito con il modal
    RestoreNormalWindow();
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
        html += `<div style="margin-top: 15px; margin-bottom: 5px; font-weight: 600; color: #ffffff; position: relative; z-index: 200; background: #242424; padding: 5px 0;">`;
        html += `${projectName}`;
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
    const activityTypeObj = activityTypes.find(t => t.name === activityTypeName);
    let bgStyle = 'background: #ffffff;';

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

    // Escape values for onclick handler
    const escapedActivityType = escapeJs(activityTypeName);
    const escapedProjectName = escapeJs(session.project_name || '');

    return `
        <div class="timeline-segment"
             style="left: ${left}%; width: ${Math.max(width, 0.5)}%; ${bgStyle} z-index: 2;"
             title="${escapeHtml(tooltipText).replace(/"/g, '&quot;')}"
             data-session-id="${parseInt(session.id)}"
             onclick="showSessionMenu(event, ${parseInt(session.id)}, ${parseInt(session.seconds)}, '${escapedActivityType}', '${escapedProjectName}')">
        </div>
    `;
}

function formatDate(dateStr) {
    const date = new Date(dateStr);
    return `${date.getDate()}/${date.getMonth() + 1}/${date.getFullYear()}`;
}

// === NOTE PROGETTO (MARKDOWN) ===

let projectNoteEditor = null;

window.openProjectNoteModal = function(projectId, projectName, noteText) {
    document.getElementById('projectNoteProjectId').value = projectId;
    document.getElementById('projectNoteProjectName').textContent = projectName;
    document.getElementById('projectNoteModal').classList.add('show');

    // Inizializza EasyMDE se non esiste già
    setTimeout(() => {
        if (projectNoteEditor) {
            projectNoteEditor.toTextArea();
            projectNoteEditor = null;
        }

        const textarea = document.getElementById('projectNoteEditor');
        textarea.value = noteText || '';

        projectNoteEditor = new EasyMDE({
            element: textarea,
            spellChecker: false,
            autosave: {
                enabled: false
            },
            toolbar: [
                'bold', 'italic', 'heading', '|',
                'quote', 'unordered-list', 'ordered-list', '|',
                'link', 'image', '|',
                'preview', 'side-by-side', 'fullscreen', '|',
                'guide'
            ],
            placeholder: 'Scrivi le note del progetto in markdown...',
            status: false,
            minHeight: '300px'
        });
    }, 100);
}

window.closeProjectNoteModal = function() {
    if (projectNoteEditor) {
        projectNoteEditor.toTextArea();
        projectNoteEditor = null;
    }
    document.getElementById('projectNoteModal').classList.remove('show');
}

window.saveProjectNote = async function() {
    const projectId = parseInt(document.getElementById('projectNoteProjectId').value);
    const noteText = projectNoteEditor ? projectNoteEditor.value() : document.getElementById('projectNoteEditor').value;

    try {
        await UpdateProjectNote(projectId, noteText);
        showNotification('Note salvate con successo!', 'success');

        // Aggiorna la cache dei progetti
        const project = projectsCache.find(p => p.id === projectId);
        if (project) {
            project.note_text = noteText;
        }

        closeProjectNoteModal();
    } catch (error) {
        console.error('Errore salvataggio note:', error);
        showNotification('Errore: ' + error, 'error');
    }
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

    // Popola ore e minuti per inizio e fine
    const hourSelects = ['sessionStartHour', 'sessionEndHour'];
    const minuteSelects = ['sessionStartMinute', 'sessionEndMinute'];

    hourSelects.forEach(id => {
        const select = document.getElementById(id);
        select.innerHTML = '';
        for (let h = 0; h < 24; h++) {
            select.innerHTML += `<option value="${h}">${String(h).padStart(2, '0')}</option>`;
        }
    });

    minuteSelects.forEach(id => {
        const select = document.getElementById(id);
        select.innerHTML = '';
        for (let m = 0; m < 60; m++) {
            select.innerHTML += `<option value="${m}">${String(m).padStart(2, '0')}</option>`;
        }
    });

    // Imposta data/ora corrente come fine, un'ora prima come inizio
    const now = new Date();
    const oneHourAgo = new Date(now.getTime() - 60 * 60 * 1000);

    document.getElementById('sessionStartDate').value = oneHourAgo.toISOString().split('T')[0];
    document.getElementById('sessionStartHour').value = oneHourAgo.getHours();
    document.getElementById('sessionStartMinute').value = oneHourAgo.getMinutes();

    document.getElementById('sessionEndDate').value = now.toISOString().split('T')[0];
    document.getElementById('sessionEndHour').value = now.getHours();
    document.getElementById('sessionEndMinute').value = now.getMinutes();

    // Aggiungi listener per calcolare la durata
    const updateDuration = () => updateNewSessionDuration();
    ['sessionStartDate', 'sessionStartHour', 'sessionStartMinute', 'sessionEndDate', 'sessionEndHour', 'sessionEndMinute'].forEach(id => {
        document.getElementById(id).addEventListener('change', updateDuration);
    });

    updateNewSessionDuration();
    document.getElementById('createSessionModal').classList.add('show');
}

function updateNewSessionDuration() {
    const startDate = document.getElementById('sessionStartDate').value;
    const startHour = parseInt(document.getElementById('sessionStartHour').value);
    const startMinute = parseInt(document.getElementById('sessionStartMinute').value);
    const endDate = document.getElementById('sessionEndDate').value;
    const endHour = parseInt(document.getElementById('sessionEndHour').value);
    const endMinute = parseInt(document.getElementById('sessionEndMinute').value);

    if (!startDate || !endDate) {
        document.getElementById('sessionCalculatedDuration').textContent = '--';
        return;
    }

    const start = new Date(`${startDate}T${String(startHour).padStart(2, '0')}:${String(startMinute).padStart(2, '0')}:00`);
    const end = new Date(`${endDate}T${String(endHour).padStart(2, '0')}:${String(endMinute).padStart(2, '0')}:00`);

    const diffSeconds = Math.floor((end - start) / 1000);

    if (diffSeconds <= 0) {
        document.getElementById('sessionCalculatedDuration').textContent = 'Non valida';
        return;
    }

    const hours = Math.floor(diffSeconds / 3600);
    const minutes = Math.floor((diffSeconds % 3600) / 60);
    document.getElementById('sessionCalculatedDuration').textContent = `${hours}h ${minutes}m`;
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

    const startDate = document.getElementById('sessionStartDate').value;
    const startHour = parseInt(document.getElementById('sessionStartHour').value);
    const startMinute = parseInt(document.getElementById('sessionStartMinute').value);
    const endDate = document.getElementById('sessionEndDate').value;
    const endHour = parseInt(document.getElementById('sessionEndHour').value);
    const endMinute = parseInt(document.getElementById('sessionEndMinute').value);

    if (!startDate || !endDate) {
        showNotification('Inserisci data di inizio e fine', 'error');
        return;
    }

    const start = new Date(`${startDate}T${String(startHour).padStart(2, '0')}:${String(startMinute).padStart(2, '0')}:00`);
    const end = new Date(`${endDate}T${String(endHour).padStart(2, '0')}:${String(endMinute).padStart(2, '0')}:00`);

    const seconds = Math.floor((end - start) / 1000);

    if (seconds <= 0) {
        showNotification('La data/ora di fine deve essere successiva all\'inizio', 'error');
        return;
    }

    const timestamp = `${startDate} ${String(startHour).padStart(2, '0')}:${String(startMinute).padStart(2, '0')}:00`;
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

    // Validate inputs
    sessionID = parseInt(sessionID);
    seconds = parseInt(seconds);

    // Rimuovi menu esistente
    const existingMenu = document.getElementById('contextMenu');
    if (existingMenu) existingMenu.remove();

    // Apri direttamente il modal unificato
    openEditSessionUnifiedModal(sessionID);
}

// === SPLIT SESSION ===

window.splitSessionDialog = async function(sessionID, totalSeconds, currentActivityType, projectName) {
    // Rimuovi menu contestuale
    const menu = document.getElementById('contextMenu');
    if (menu) menu.remove();

    const totalMinutes = Math.floor(totalSeconds / 60);

    // Imposta valori nel modale
    document.getElementById('splitSessionId').value = sessionID;
    document.getElementById('splitSessionTotalSeconds').value = totalSeconds;
    document.getElementById('splitSessionProject').textContent = projectName;
    document.getElementById('splitSessionTotal').textContent = totalMinutes;
    document.getElementById('splitSessionFirstPart').value = Math.floor(totalMinutes / 2);
    document.getElementById('splitSessionFirstPart').max = totalMinutes - 1;
    updateSplitSecondPart();

    // Popola select tipi attività
    const firstSelect = document.getElementById('splitSessionFirstType');
    const secondSelect = document.getElementById('splitSessionSecondType');
    firstSelect.innerHTML = '<option value="">Nessuno</option>';
    secondSelect.innerHTML = '<option value="">Nessuno</option>';
    activityTypes.forEach(type => {
        const firstSelected = type.name === currentActivityType ? 'selected' : '';
        firstSelect.innerHTML += `<option value="${escapeHtml(type.name)}" ${firstSelected}>${escapeHtml(type.name)}</option>`;
        secondSelect.innerHTML += `<option value="${escapeHtml(type.name)}">${escapeHtml(type.name)}</option>`;
    });

    // Mostra modale
    document.getElementById('splitSessionModal').classList.add('show');

    // Focus sull'input
    setTimeout(() => {
        document.getElementById('splitSessionFirstPart').focus();
        document.getElementById('splitSessionFirstPart').select();
    }, 100);
}

// Aggiorna calcolo seconda parte in tempo reale
function updateSplitSecondPart() {
    const totalSeconds = parseInt(document.getElementById('splitSessionTotalSeconds').value);
    const totalMinutes = Math.floor(totalSeconds / 60);
    const firstPart = parseInt(document.getElementById('splitSessionFirstPart').value) || 0;
    const secondPart = totalMinutes - firstPart;
    document.getElementById('splitSessionSecondPart').textContent = secondPart > 0 ? secondPart : '--';
}

// Event listener per aggiornamento in tempo reale
document.addEventListener('DOMContentLoaded', () => {
    const splitInput = document.getElementById('splitSessionFirstPart');
    if (splitInput) {
        splitInput.addEventListener('input', updateSplitSecondPart);
    }
});

window.closeSplitSessionModal = function() {
    document.getElementById('splitSessionModal').classList.remove('show');
}

window.confirmSplitSession = async function() {
    const sessionID = parseInt(document.getElementById('splitSessionId').value);
    const totalSeconds = parseInt(document.getElementById('splitSessionTotalSeconds').value);
    const totalMinutes = Math.floor(totalSeconds / 60);
    const firstPartMinutes = parseInt(document.getElementById('splitSessionFirstPart').value);

    if (isNaN(firstPartMinutes) || firstPartMinutes <= 0 || firstPartMinutes >= totalMinutes) {
        showNotification('Durata prima parte non valida', 'error');
        return;
    }

    const firstPartSeconds = firstPartMinutes * 60;
    const firstActivityType = document.getElementById('splitSessionFirstType').value || null;
    const secondActivityType = document.getElementById('splitSessionSecondType').value || null;

    try {
        await SplitSession(sessionID, firstPartSeconds, firstActivityType, secondActivityType);
        showNotification('Sessione divisa!', 'success');
        closeSplitSessionModal();
        await loadTimeline();
    } catch (error) {
        console.error('Errore divisione sessione:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.editActivityType = async function(sessionID, currentActivityType, projectName) {
    // Rimuovi menu contestuale
    const menu = document.getElementById('contextMenu');
    if (menu) menu.remove();

    // Popola select con tipi attività
    const select = document.getElementById('editSessionActivitySelect');
    select.innerHTML = '<option value="">Nessuno</option>';
    activityTypes.forEach(type => {
        const selected = type.name === currentActivityType ? 'selected' : '';
        select.innerHTML += `<option value="${escapeHtml(type.name)}" ${selected}>${escapeHtml(type.name)}</option>`;
    });

    // Imposta valori nel modale
    document.getElementById('editSessionActivityId').value = sessionID;
    document.getElementById('editSessionActivityProject').textContent = projectName;
    document.getElementById('editSessionActivityCurrent').textContent = currentActivityType || 'Non specificato';

    // Mostra modale
    document.getElementById('editSessionActivityModal').classList.add('show');
}

window.closeEditSessionActivityModal = function() {
    document.getElementById('editSessionActivityModal').classList.remove('show');
}

window.confirmEditSessionActivity = async function() {
    const sessionID = parseInt(document.getElementById('editSessionActivityId').value);
    const newActivityType = document.getElementById('editSessionActivitySelect').value || null;

    try {
        await UpdateSessionActivityType(sessionID, newActivityType);
        showNotification('Tipo attività aggiornato!', 'success');
        closeEditSessionActivityModal();
        await loadTimeline();
    } catch (error) {
        console.error('Errore aggiornamento tipo attività:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.editSessionDuration = async function(sessionID, currentSeconds, projectName) {
    // Rimuovi menu contestuale
    const menu = document.getElementById('contextMenu');
    if (menu) menu.remove();

    const currentMinutes = Math.floor(currentSeconds / 60);

    // Imposta valori nel modale
    document.getElementById('editSessionDurationId').value = sessionID;
    document.getElementById('editSessionDurationProject').textContent = projectName;
    document.getElementById('editSessionDurationCurrent').textContent = currentMinutes;
    document.getElementById('editSessionDurationInput').value = currentMinutes;

    // Mostra modale
    document.getElementById('editSessionDurationModal').classList.add('show');

    // Focus sull'input
    setTimeout(() => {
        document.getElementById('editSessionDurationInput').focus();
        document.getElementById('editSessionDurationInput').select();
    }, 100);
}

window.closeEditSessionDurationModal = function() {
    document.getElementById('editSessionDurationModal').classList.remove('show');
}

window.confirmEditSessionDuration = async function() {
    const sessionID = parseInt(document.getElementById('editSessionDurationId').value);
    const newMinutes = parseInt(document.getElementById('editSessionDurationInput').value);

    if (isNaN(newMinutes) || newMinutes <= 0) {
        showNotification('Durata non valida', 'error');
        return;
    }

    try {
        await UpdateSessionDuration(sessionID, newMinutes * 60);
        showNotification('Durata aggiornata!', 'success');
        closeEditSessionDurationModal();
        await loadTimeline();
    } catch (error) {
        console.error('Errore aggiornamento durata:', error);
        showNotification('Errore: ' + error, 'error');
    }
}

window.deleteSession = async function(sessionID, projectName) {
    // Rimuovi menu contestuale
    const menu = document.getElementById('contextMenu');
    if (menu) menu.remove();

    showConfirmDeleteModal(
        sessionID,
        'session',
        `Eliminare sessione di "${projectName}"?`,
        'La sessione verrà eliminata definitivamente.'
    );
}

// === MODAL UNIFICATO MODIFICA SESSIONE ===

// Variabile per memorizzare i dati della sessione corrente
let currentEditSession = null;

window.openEditSessionUnifiedModal = async function(sessionID) {
    try {
        // Carica i dati della sessione
        const session = await GetSessionById(sessionID);
        currentEditSession = session;

        // Imposta i valori nel modal
        document.getElementById('editSessionUnifiedId').value = sessionID;
        document.getElementById('editSessionUnifiedProject').textContent = session.project_name || 'Nessun progetto';

        // Calcola durata
        const minutes = Math.floor(session.seconds / 60);
        document.getElementById('editSessionUnifiedCurrentDuration').textContent = `${minutes} minuti`;

        // Parsa il timestamp
        const startTime = parseSessionTimestamp(session.timestamp);
        const endTime = new Date(startTime.getTime() + session.seconds * 1000);

        // Mostra periodo corrente
        const startStr = formatTimeHHMM(startTime);
        const endStr = formatTimeHHMM(endTime);
        document.getElementById('editSessionUnifiedCurrentPeriod').textContent = `${startStr} - ${endStr}`;

        // Popola i selettori di ore e minuti
        populateTimeSelectors();

        // Imposta data e ora di inizio
        document.getElementById('editSessionUnifiedStartDate').value = formatDateYYYYMMDD(startTime);
        document.getElementById('editSessionUnifiedStartHour').value = startTime.getHours();
        document.getElementById('editSessionUnifiedStartMinute').value = startTime.getMinutes();

        // Imposta data e ora di fine
        document.getElementById('editSessionUnifiedEndDate').value = formatDateYYYYMMDD(endTime);
        document.getElementById('editSessionUnifiedEndHour').value = endTime.getHours();
        document.getElementById('editSessionUnifiedEndMinute').value = endTime.getMinutes();

        // Imposta data e ora del punto di divisione (metà sessione)
        const splitTime = new Date(startTime.getTime() + (session.seconds / 2) * 1000);
        document.getElementById('editSessionUnifiedSplitDate').value = formatDateYYYYMMDD(splitTime);
        document.getElementById('editSessionUnifiedSplitHour').value = splitTime.getHours();
        document.getElementById('editSessionUnifiedSplitMinute').value = splitTime.getMinutes();

        // Popola select tipi attività
        const activitySelect = document.getElementById('editSessionUnifiedActivityType');
        const firstTypeSelect = document.getElementById('editSessionUnifiedFirstType');
        const secondTypeSelect = document.getElementById('editSessionUnifiedSecondType');

        activitySelect.innerHTML = '<option value="">Nessuno</option>';
        firstTypeSelect.innerHTML = '<option value="">Nessuno</option>';
        secondTypeSelect.innerHTML = '<option value="">Nessuno</option>';

        activityTypes.forEach(type => {
            const selected = type.name === session.activity_type ? 'selected' : '';
            activitySelect.innerHTML += `<option value="${escapeHtml(type.name)}" ${selected}>${escapeHtml(type.name)}</option>`;
            const firstSelected = type.name === session.activity_type ? 'selected' : '';
            firstTypeSelect.innerHTML += `<option value="${escapeHtml(type.name)}" ${firstSelected}>${escapeHtml(type.name)}</option>`;
            secondTypeSelect.innerHTML += `<option value="${escapeHtml(type.name)}">${escapeHtml(type.name)}</option>`;
        });

        // Reset modalità a "Fine"
        document.querySelector('input[name="editSessionUnifiedMode"][value="end"]').checked = true;
        toggleEditSessionMode();

        // Aggiorna calcoli durata
        updateUnifiedDurationCalc();

        // Mostra modal
        document.getElementById('editSessionUnifiedModal').classList.add('show');

    } catch (error) {
        console.error('Errore apertura modal unificato:', error);
        showNotification('Errore caricamento sessione', 'error');
    }
}

window.closeEditSessionUnifiedModal = function() {
    document.getElementById('editSessionUnifiedModal').classList.remove('show');
    currentEditSession = null;
}

window.selectRadioOption = function(label, value) {
    // Rimuovi selected da tutti i radio-option nello stesso gruppo
    const parent = label.parentElement;
    parent.querySelectorAll('.radio-option').forEach(opt => opt.classList.remove('selected'));
    // Aggiungi selected all'opzione cliccata
    label.classList.add('selected');
    // Seleziona il radio button
    const radio = label.querySelector('input[type="radio"]');
    radio.checked = true;
    // Trigger change event
    radio.dispatchEvent(new Event('change'));
}

window.toggleEditSessionMode = function() {
    const mode = document.querySelector('input[name="editSessionUnifiedMode"]:checked').value;

    if (mode === 'end') {
        document.getElementById('editSessionUnifiedEndSection').style.display = 'block';
        document.getElementById('editSessionUnifiedSplitSection').style.display = 'none';
        document.getElementById('editSessionUnifiedActivitySection').style.display = 'block';
    } else {
        document.getElementById('editSessionUnifiedEndSection').style.display = 'none';
        document.getElementById('editSessionUnifiedSplitSection').style.display = 'block';
        document.getElementById('editSessionUnifiedActivitySection').style.display = 'none';
    }
}

window.confirmEditSessionUnified = async function() {
    const sessionID = parseInt(document.getElementById('editSessionUnifiedId').value);
    const mode = document.querySelector('input[name="editSessionUnifiedMode"]:checked').value;

    // Ottieni orario di inizio
    const startDate = document.getElementById('editSessionUnifiedStartDate').value;
    const startHour = parseInt(document.getElementById('editSessionUnifiedStartHour').value);
    const startMinute = parseInt(document.getElementById('editSessionUnifiedStartMinute').value);
    const startTime = new Date(`${startDate}T${String(startHour).padStart(2, '0')}:${String(startMinute).padStart(2, '0')}:00`);

    if (mode === 'end') {
        // Modalità modifica fine
        const endDate = document.getElementById('editSessionUnifiedEndDate').value;
        const endHour = parseInt(document.getElementById('editSessionUnifiedEndHour').value);
        const endMinute = parseInt(document.getElementById('editSessionUnifiedEndMinute').value);
        const endTime = new Date(`${endDate}T${String(endHour).padStart(2, '0')}:${String(endMinute).padStart(2, '0')}:00`);

        // Calcola nuova durata
        const newSeconds = Math.floor((endTime - startTime) / 1000);
        if (newSeconds <= 0) {
            showNotification('L\'orario di fine deve essere successivo all\'orario di inizio', 'error');
            return;
        }

        const activityType = document.getElementById('editSessionUnifiedActivityType').value || null;
        const newTimestamp = startTime.getFullYear() + '-' +
            String(startTime.getMonth() + 1).padStart(2, '0') + '-' +
            String(startTime.getDate()).padStart(2, '0') + ' ' +
            String(startTime.getHours()).padStart(2, '0') + ':' +
            String(startTime.getMinutes()).padStart(2, '0') + ':00';

        try {
            await UpdateSessionComplete(sessionID, newTimestamp, newSeconds, activityType);
            showNotification('Sessione aggiornata!', 'success');
            closeEditSessionUnifiedModal();
            await loadTimeline();
        } catch (error) {
            console.error('Errore aggiornamento sessione:', error);
            showNotification('Errore: ' + error, 'error');
        }
    } else {
        // Modalità divisione
        const splitDate = document.getElementById('editSessionUnifiedSplitDate').value;
        const splitHour = parseInt(document.getElementById('editSessionUnifiedSplitHour').value);
        const splitMinute = parseInt(document.getElementById('editSessionUnifiedSplitMinute').value);
        const splitTime = new Date(`${splitDate}T${String(splitHour).padStart(2, '0')}:${String(splitMinute).padStart(2, '0')}:00`);

        // Calcola durata prima parte
        const firstPartSeconds = Math.floor((splitTime - startTime) / 1000);
        if (firstPartSeconds <= 0) {
            showNotification('Il punto di divisione deve essere dopo l\'inizio', 'error');
            return;
        }

        // Prima aggiorna il timestamp di inizio se è cambiato
        const originalStartTime = parseSessionTimestamp(currentEditSession.timestamp);
        if (startTime.getTime() !== originalStartTime.getTime()) {
            const newTimestamp = startTime.getFullYear() + '-' +
                String(startTime.getMonth() + 1).padStart(2, '0') + '-' +
                String(startTime.getDate()).padStart(2, '0') + ' ' +
                String(startTime.getHours()).padStart(2, '0') + ':' +
                String(startTime.getMinutes()).padStart(2, '0') + ':00';

            try {
                await UpdateSessionComplete(sessionID, newTimestamp, currentEditSession.seconds, currentEditSession.activity_type || null);
            } catch (error) {
                console.error('Errore aggiornamento timestamp:', error);
                showNotification('Errore aggiornamento: ' + error, 'error');
                return;
            }
        }

        const firstActivityType = document.getElementById('editSessionUnifiedFirstType').value || null;
        const secondActivityType = document.getElementById('editSessionUnifiedSecondType').value || null;

        try {
            await SplitSession(sessionID, firstPartSeconds, firstActivityType, secondActivityType);
            showNotification('Sessione divisa!', 'success');
            closeEditSessionUnifiedModal();
            await loadTimeline();
        } catch (error) {
            console.error('Errore divisione sessione:', error);
            showNotification('Errore: ' + error, 'error');
        }
    }
}

window.deleteSessionFromUnified = function() {
    const sessionID = parseInt(document.getElementById('editSessionUnifiedId').value);
    const projectName = document.getElementById('editSessionUnifiedProject').textContent;

    closeEditSessionUnifiedModal();

    showConfirmDeleteModal(
        sessionID,
        'session',
        `Eliminare sessione di "${projectName}"?`,
        'La sessione verrà eliminata definitivamente.'
    );
}

// Funzioni helper per il modal unificato
function populateTimeSelectors() {
    const hourSelects = [
        document.getElementById('editSessionUnifiedStartHour'),
        document.getElementById('editSessionUnifiedEndHour'),
        document.getElementById('editSessionUnifiedSplitHour')
    ];
    const minuteSelects = [
        document.getElementById('editSessionUnifiedStartMinute'),
        document.getElementById('editSessionUnifiedEndMinute'),
        document.getElementById('editSessionUnifiedSplitMinute')
    ];

    hourSelects.forEach(select => {
        select.innerHTML = '';
        for (let h = 0; h < 24; h++) {
            select.innerHTML += `<option value="${h}">${String(h).padStart(2, '0')}</option>`;
        }
    });

    minuteSelects.forEach(select => {
        select.innerHTML = '';
        for (let m = 0; m < 60; m++) {
            select.innerHTML += `<option value="${m}">${String(m).padStart(2, '0')}</option>`;
        }
    });

    // Aggiungi event listener per aggiornare i calcoli quando cambiano i valori
    const allInputs = [
        ...hourSelects,
        ...minuteSelects,
        document.getElementById('editSessionUnifiedStartDate'),
        document.getElementById('editSessionUnifiedEndDate'),
        document.getElementById('editSessionUnifiedSplitDate')
    ];

    allInputs.forEach(input => {
        input.addEventListener('change', updateUnifiedDurationCalc);
    });
}

function updateUnifiedDurationCalc() {
    const startDate = document.getElementById('editSessionUnifiedStartDate').value;
    const startHour = parseInt(document.getElementById('editSessionUnifiedStartHour').value);
    const startMinute = parseInt(document.getElementById('editSessionUnifiedStartMinute').value);

    if (!startDate) return;

    const startTime = new Date(`${startDate}T${String(startHour).padStart(2, '0')}:${String(startMinute).padStart(2, '0')}:00`);

    // Calcola nuova durata per modalità Fine
    const endDate = document.getElementById('editSessionUnifiedEndDate').value;
    const endHour = parseInt(document.getElementById('editSessionUnifiedEndHour').value);
    const endMinute = parseInt(document.getElementById('editSessionUnifiedEndMinute').value);

    if (endDate) {
        const endTime = new Date(`${endDate}T${String(endHour).padStart(2, '0')}:${String(endMinute).padStart(2, '0')}:00`);
        const newSeconds = Math.floor((endTime - startTime) / 1000);
        const newMinutes = Math.floor(newSeconds / 60);

        if (newSeconds > 0) {
            document.getElementById('editSessionUnifiedNewDuration').textContent = `${newMinutes} minuti`;
        } else {
            document.getElementById('editSessionUnifiedNewDuration').textContent = 'Non valido';
        }
    }

    // Calcola durate per modalità Divisione
    const splitDate = document.getElementById('editSessionUnifiedSplitDate').value;
    const splitHour = parseInt(document.getElementById('editSessionUnifiedSplitHour').value);
    const splitMinute = parseInt(document.getElementById('editSessionUnifiedSplitMinute').value);

    if (splitDate && currentEditSession) {
        const splitTime = new Date(`${splitDate}T${String(splitHour).padStart(2, '0')}:${String(splitMinute).padStart(2, '0')}:00`);
        const firstPartSeconds = Math.floor((splitTime - startTime) / 1000);
        const firstPartMinutes = Math.floor(firstPartSeconds / 60);

        // Calcola la seconda parte basandosi sull'orario di fine originale
        const originalStartTime = parseSessionTimestamp(currentEditSession.timestamp);
        const originalEndTime = new Date(originalStartTime.getTime() + currentEditSession.seconds * 1000);
        const secondPartSeconds = Math.floor((originalEndTime - splitTime) / 1000);
        const secondPartMinutes = Math.floor(secondPartSeconds / 60);

        if (firstPartSeconds > 0 && secondPartSeconds > 0) {
            document.getElementById('editSessionUnifiedFirstPartDuration').textContent = `${firstPartMinutes} min`;
            document.getElementById('editSessionUnifiedSecondPartDuration').textContent = `${secondPartMinutes} min`;
        } else {
            document.getElementById('editSessionUnifiedFirstPartDuration').textContent = 'Non valido';
            document.getElementById('editSessionUnifiedSecondPartDuration').textContent = 'Non valido';
        }
    }
}

function parseSessionTimestamp(timestamp) {
    // Parsa timestamp nel formato "2025-01-15 14:30:00" o "2025-01-15T14:30:00"
    if (timestamp.includes('T')) {
        const isoMatch = timestamp.match(/(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):?(\d{2})?/);
        if (isoMatch) {
            const [, year, month, day, hours, minutes, seconds] = isoMatch.map(Number);
            return new Date(year, month - 1, day, hours, minutes, seconds || 0);
        }
    } else {
        const parts = timestamp.split(' ');
        const datePart = parts[0];
        const timePart = parts[1] || '00:00:00';
        const [year, month, day] = datePart.split('-').map(Number);
        const [hours, minutes, seconds] = timePart.split(':').map(Number);
        return new Date(year, month - 1, day, hours, minutes, seconds || 0);
    }
    return new Date(timestamp);
}

function formatTimeHHMM(date) {
    return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
}

function formatDateYYYYMMDD(date) {
    return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
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
    showConfirmDeleteModal(
        projectID,
        'project',
        `Eliminare definitivamente "${projectName}"?`,
        'Si consiglia di esportare prima un backup del progetto in formato JSON per poterlo eventualmente reimportare in futuro.'
    );
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
