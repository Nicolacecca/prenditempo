// Variabili globali
let trackingInterval = null;

// Inizializzazione
document.addEventListener('DOMContentLoaded', function() {
    loadProjects();
    loadTodayStats();
    checkTrackingStatus();

    // Aggiorna stato ogni 5 secondi
    setInterval(checkTrackingStatus, 5000);
});

// === PROGETTI ===

async function loadProjects() {
    try {
        const response = await fetch('/api/projects');
        const projects = await response.json();

        displayProjects(projects);
        populateProjectSelect(projects);
    } catch (error) {
        console.error('Errore caricamento progetti:', error);
        showNotification('Errore caricamento progetti', 'error');
    }
}

function displayProjects(projects) {
    const projectList = document.getElementById('projectList');

    if (projects.length === 0) {
        projectList.innerHTML = '<p style="color: #6b7280;">Nessun progetto. Creane uno nuovo!</p>';
        return;
    }

    projectList.innerHTML = projects.map(project => `
        <div class="project-item">
            <div>
                <h3>${project.Name}</h3>
                ${project.Description ? `<p>${project.Description}</p>` : ''}
            </div>
            <div style="display: flex; gap: 10px;">
                <button class="btn" style="width: 150px; padding: 8px 16px; margin: 0; background: #f59e0b;"
                        onclick="archiveProject(${project.ID}, '${project.Name.replace(/'/g, "\\'")}')">Chiudi Progetto</button>
            </div>
        </div>
    `).join('');
}

function populateProjectSelect(projects) {
    const select = document.getElementById('projectSelect');
    select.innerHTML = '<option value="">Seleziona progetto...</option>';

    projects.forEach(project => {
        const option = document.createElement('option');
        option.value = project.Name;
        option.textContent = project.Name;
        select.appendChild(option);
    });
}

function openCreateProjectModal() {
    document.getElementById('createProjectModal').classList.add('show');
}

function closeCreateProjectModal() {
    document.getElementById('createProjectModal').classList.remove('show');
    document.getElementById('modalProjectName').value = '';
    document.getElementById('modalProjectDesc').value = '';
}

async function createProject() {
    const name = document.getElementById('modalProjectName').value.trim();
    const description = document.getElementById('modalProjectDesc').value.trim();

    if (!name) {
        showNotification('Inserisci un nome per il progetto', 'error');
        return;
    }

    try {
        const response = await fetch('/api/projects/create', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, description })
        });

        if (response.ok) {
            showNotification('Progetto creato con successo!', 'success');
            closeCreateProjectModal();
            loadProjects();
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore creazione progetto:', error);
        showNotification('Errore creazione progetto', 'error');
    }
}

async function deleteProject(name) {
    if (!confirm(`Vuoi davvero eliminare il progetto "${name}"?`)) {
        return;
    }

    try {
        const response = await fetch('/api/projects/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name })
        });

        if (response.ok) {
            showNotification('Progetto eliminato', 'success');
            loadProjects();
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore eliminazione progetto:', error);
        showNotification('Errore eliminazione progetto', 'error');
    }
}

async function archiveProject(projectID, projectName) {
    if (!confirm(`Vuoi chiudere il progetto "${projectName}"?\n\nIl progetto verrà archiviato e verrà generato un report automatico.`)) {
        return;
    }

    try {
        const response = await fetch('/api/projects/archive', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: projectID })
        });

        if (response.ok) {
            const data = await response.json();
            showNotification('Progetto archiviato con successo!', 'success');

            // Mostra il report in una modale
            showReportModal(data.report);

            // Ricarica la lista progetti
            loadProjects();
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore archiviazione progetto:', error);
        showNotification('Errore archiviazione progetto', 'error');
    }
}

// === TRACKING ===

let isCurrentlyTracking = false;

async function toggleTracking() {
    if (isCurrentlyTracking) {
        await stopTracking();
    } else {
        await startTracking();
    }
}

async function startTracking() {
    const projectName = document.getElementById('projectSelect').value;
    const activityType = document.getElementById('activityTypeSelect').value || null;
    // Legge il tempo di inattività da localStorage (salvato nelle impostazioni)
    const idleThreshold = parseInt(localStorage.getItem('idleThreshold')) || 5;

    if (!projectName) {
        showNotification('Seleziona un progetto', 'error');
        return;
    }

    try {
        const response = await fetch('/api/tracking/start', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                project_name: projectName,
                activity_type: activityType,
                idle_threshold: idleThreshold
            })
        });

        if (response.ok) {
            showNotification(`Tracking avviato per: ${projectName}`, 'success');
            isCurrentlyTracking = true;
            updateUIForTracking(true);
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore avvio tracking:', error);
        showNotification('Errore avvio tracking', 'error');
    }
}

async function stopTracking() {
    try {
        const response = await fetch('/api/tracking/stop', {
            method: 'POST'
        });

        if (response.ok) {
            const result = await response.json();
            showNotification('Tracking fermato e dati salvati', 'success');
            isCurrentlyTracking = false;
            updateUIForTracking(false);
            loadTodayStats();
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore stop tracking:', error);
        showNotification('Errore stop tracking', 'error');
    }
}

async function checkTrackingStatus() {
    try {
        const response = await fetch('/api/tracking/status');
        const status = await response.json();

        if (status.running) {
            isCurrentlyTracking = true;
            updateUIForTracking(true);
            updateLiveStats(status.stats);
            document.getElementById('statusText').textContent = `Tracking attivo: ${status.project}`;

            // Controlla se c'è un periodo idle pendente
            checkForPendingIdle();
        } else {
            isCurrentlyTracking = false;
            updateUIForTracking(false);
            document.getElementById('statusText').textContent = 'Pronto';
        }
    } catch (error) {
        console.error('Errore check status:', error);
    }
}

function updateLiveStats(stats) {
    const liveStatsDiv = document.getElementById('liveStats');
    const currentStatsDiv = document.getElementById('currentStats');

    if (!stats || Object.keys(stats).length === 0) {
        currentStatsDiv.style.display = 'none';
        return;
    }

    currentStatsDiv.style.display = 'block';

    liveStatsDiv.innerHTML = Object.entries(stats)
        .map(([app, seconds]) => {
            const minutes = Math.floor(seconds / 60);
            return `
                <div class="stat-item">
                    <span class="stat-label">${app}</span>
                    <span class="stat-value">${minutes} min (${seconds}s)</span>
                </div>
            `;
        }).join('');
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

async function loadTodayStats() {
    try {
        const response = await fetch('/api/stats/today');
        const stats = await response.json();

        displayTodayStats(stats);
    } catch (error) {
        console.error('Errore caricamento statistiche:', error);
    }
}

function displayTodayStats(stats) {
    const statsDiv = document.getElementById('todayStats');

    if (!stats || Object.keys(stats).length === 0) {
        statsDiv.innerHTML = '<p style="color: #6b7280;">Nessuna statistica disponibile oggi</p>';
        return;
    }

    // Calcola totale
    const total = Object.values(stats).reduce((sum, sec) => sum + sec, 0);
    const totalMinutes = Math.floor(total / 60);
    const totalHours = (total / 3600).toFixed(1);

    statsDiv.innerHTML = `
        <div class="stat-item">
            <span class="stat-label"><strong>Totale Oggi</strong></span>
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

// === TIMELINE ===

// Imposta data di oggi di default
document.addEventListener('DOMContentLoaded', function() {
    setToday();
    loadTimeline();
});

function setToday() {
    const today = new Date().toISOString().split('T')[0];
    document.getElementById('timelineStartDate').value = today;
    document.getElementById('timelineEndDate').value = today;
    loadTimeline();
}

function setThisWeek() {
    const today = new Date();
    const monday = new Date(today);
    monday.setDate(today.getDate() - today.getDay() + 1); // Lunedì
    const sunday = new Date(monday);
    sunday.setDate(monday.getDate() + 6); // Domenica

    document.getElementById('timelineStartDate').value = monday.toISOString().split('T')[0];
    document.getElementById('timelineEndDate').value = sunday.toISOString().split('T')[0];
    loadTimeline();
}

async function loadTimeline() {
    const startDate = document.getElementById('timelineStartDate').value;
    const endDate = document.getElementById('timelineEndDate').value;

    if (!startDate || !endDate) {
        showNotification('Seleziona data inizio e fine', 'error');
        return;
    }

    try {
        // Carica sia le sessioni che le note
        const [sessionsResponse, notesResponse] = await Promise.all([
            fetch(`/api/stats/timeline?start=${startDate}&end=${endDate}`),
            fetch(`/api/notes/timeline?start=${startDate}&end=${endDate}`)
        ]);

        const sessions = await sessionsResponse.json();
        const notes = await notesResponse.json();

        displayTimeline(sessions, notes || [], startDate, endDate);
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
    sessions.sort((a, b) => new Date(a.Timestamp) - new Date(b.Timestamp));

    // Trova tutti i progetti unici e genera colori
    const uniqueProjects = [...new Set(sessions.map(s => s.ProjectName || 'Nessun progetto'))];
    const projectColors = generateProjectColors(uniqueProjects);

    // Determina il range temporale in base al periodo selezionato
    // Usiamo UTC per allinearci con i timestamp del database che sono in UTC
    const [startYear, startMonth, startDay] = startDate.split('-').map(Number);
    const [endYear, endMonth, endDay] = endDate.split('-').map(Number);
    const startTime = new Date(Date.UTC(startYear, startMonth - 1, startDay, 0, 0, 0));
    const endTime = new Date(Date.UTC(endYear, endMonth - 1, endDay, 23, 59, 59));
    const totalMs = endTime - startTime;

    // Calcola il numero di giorni nel periodo
    const numDays = Math.ceil(totalMs / (1000 * 60 * 60 * 24));

    // Raggruppa e aggrega sessioni per progetto e tipo di attività
    const sessionsByProject = {};
    sessions.forEach(session => {
        const projectName = session.ProjectName || 'Nessun progetto';
        if (!sessionsByProject[projectName]) {
            sessionsByProject[projectName] = [];
        }
        sessionsByProject[projectName].push(session);
    });

    // Aggrega sessioni consecutive con stesso progetto e activity_type
    const aggregateConsecutiveSessions = (sessions) => {
        if (!sessions || sessions.length === 0) return [];

        const aggregated = [];
        let currentGroup = null;

        sessions.forEach(session => {
            const activityType = session.ActivityType || 'NESSUNA';
            const sessionType = session.SessionType || 'computer';

            // Controlla se questa sessione può essere aggregata con il gruppo corrente
            if (currentGroup &&
                currentGroup.ActivityType === activityType &&
                currentGroup.SessionType === sessionType) {

                // Aggiungi al gruppo corrente
                currentGroup.Sessions.push(session);
                currentGroup.TotalSeconds += session.Seconds;
                // Aggiorna il timestamp finale
                const sessionTime = new Date(session.Timestamp);
                const sessionEndTime = new Date(sessionTime.getTime() + session.Seconds * 1000);
                if (sessionEndTime > new Date(currentGroup.EndTimestamp)) {
                    currentGroup.EndTimestamp = sessionEndTime.toISOString().replace('T', ' ').slice(0, 19);
                }
            } else {
                // Crea nuovo gruppo
                const sessionTime = new Date(session.Timestamp);
                const sessionEndTime = new Date(sessionTime.getTime() + session.Seconds * 1000);

                currentGroup = {
                    ActivityType: activityType,
                    SessionType: sessionType,
                    ProjectName: session.ProjectName,
                    ProjectID: session.ProjectID,
                    Timestamp: session.Timestamp,
                    EndTimestamp: sessionEndTime.toISOString().replace('T', ' ').slice(0, 19),
                    TotalSeconds: session.Seconds,
                    Sessions: [session], // Array di sessioni originali
                    // Manteniamo i campi per compatibilità
                    ID: session.ID,
                    AppName: session.AppName
                };
                aggregated.push(currentGroup);
            }
        });

        return aggregated;
    };

    // Applica aggregazione a ogni progetto
    Object.keys(sessionsByProject).forEach(projectName => {
        sessionsByProject[projectName] = aggregateConsecutiveSessions(sessionsByProject[projectName]);
    });

    // Raggruppa note per progetto
    const notesByProject = {};
    if (notes && notes.length > 0) {
        notes.forEach(note => {
            // Trova il nome del progetto dalla lista di progetti
            const projectName = sessions.find(s => s.ProjectID === note.ProjectID)?.ProjectName || 'Nessun progetto';
            if (!notesByProject[projectName]) {
                notesByProject[projectName] = [];
            }
            notesByProject[projectName].push(note);
        });
    }

    // Costruisci HTML
    let html = '<div style="margin-bottom: 10px; color: #6b7280; font-size: 0.9em;">';
    html += `Periodo: ${formatDate(startDate)} - ${formatDate(endDate)}`;
    html += '</div>';

    // Wrapper per timeline e marker con overflow hidden per limitare le linee verticali
    html += '<div style="position: relative; overflow: hidden;">';

    // Markers temporali adattivi in base al periodo
    html += '<div class="timeline-hour-markers">';

    if (numDays === 1) {
        // Timeline giornaliera: mostra ore ogni ora con posizionamento assoluto
        for (let hour = 0; hour <= 23; hour++) {
            // Calcola la percentuale per questa ora
            const hourMs = hour * 60 * 60 * 1000;
            const percentage = (hourMs / totalMs) * 100;
            // Sposta solo il testo per il primo marker (verso destra) - il marker rimane centrato
            const spanStyle = hour === 0 ? 'style="position: relative; left: 50%;"' : '';
            html += `<div class="timeline-hour-marker" style="left: ${percentage}%;"><span ${spanStyle}>${String(hour).padStart(2, '0')}:00</span></div>`;
        }
        // Aggiungi marker per 23:59 con testo spostato a sinistra
        html += `<div class="timeline-hour-marker" style="left: 100%;"><span style="position: relative; right: 50%;">23:59</span></div>`;
    } else if (numDays <= 7) {
        // Timeline settimanale: mostra giorni con posizionamento assoluto
        for (let d = 0; d <= numDays; d++) {
            const dayMs = d * 24 * 60 * 60 * 1000;
            const percentage = (dayMs / totalMs) * 100;
            const date = new Date(startTime.getTime() + dayMs);
            const dayName = date.toLocaleDateString('it-IT', { weekday: 'short' });
            // Sposta solo il testo per primo e ultimo marker
            const spanStyle = d === 0 ? 'style="position: relative; left: 50%;"' : (d === numDays ? 'style="position: relative; right: 50%;"' : '');
            html += `<div class="timeline-hour-marker" style="left: ${percentage}%;"><span ${spanStyle}>${dayName} ${date.getUTCDate()}/${date.getUTCMonth() + 1}</span></div>`;
        }
    } else if (numDays <= 31) {
        // Timeline mensile: mostra date ogni 3-4 giorni con posizionamento assoluto
        const step = Math.ceil(numDays / 8);
        const markers = [];
        for (let d = 0; d <= numDays; d += step) {
            markers.push(d);
        }
        markers.forEach((d, index) => {
            const dayMs = d * 24 * 60 * 60 * 1000;
            const percentage = (dayMs / totalMs) * 100;
            const date = new Date(startTime.getTime() + dayMs);
            // Sposta solo il testo per primo e ultimo marker
            const spanStyle = index === 0 ? 'style="position: relative; left: 50%;"' : (index === markers.length - 1 ? 'style="position: relative; right: 50%;"' : '');
            html += `<div class="timeline-hour-marker" style="left: ${percentage}%;"><span ${spanStyle}>${date.getUTCDate()}/${date.getUTCMonth() + 1}</span></div>`;
        });
    } else if (numDays <= 62) {
        // Timeline 32-62 giorni: mostra ogni 7 giorni (settimanale)
        for (let d = 0; d <= numDays; d += 7) {
            const dayMs = d * 24 * 60 * 60 * 1000;
            const percentage = (dayMs / totalMs) * 100;
            const date = new Date(startTime.getTime() + dayMs);
            // Sposta solo il testo per primo e ultimo marker
            const spanStyle = d === 0 ? 'style="position: relative; left: 50%;"' : (d + 7 > numDays ? 'style="position: relative; right: 50%;"' : '');
            html += `<div class="timeline-hour-marker" style="left: ${percentage}%;"><span ${spanStyle}>${date.getUTCDate()}/${date.getUTCMonth() + 1}</span></div>`;
        }
    } else {
        // Timeline oltre 2 mesi: mostra solo il primo giorno di ogni mese
        const startMonth = new Date(startTime);

        // Parti dal primo giorno del mese di inizio (o dal giorno di inizio se è il primo)
        let currentDate = new Date(Date.UTC(startMonth.getUTCFullYear(), startMonth.getUTCMonth(), 1));

        // Se il primo del mese è prima della data di inizio, vai al mese successivo
        if (currentDate < startTime) {
            currentDate = new Date(Date.UTC(startMonth.getUTCFullYear(), startMonth.getUTCMonth() + 1, 1));
        }

        let isFirst = true;
        while (currentDate <= endTime) {
            const dayMs = currentDate - startTime;
            const percentage = (dayMs / totalMs) * 100;

            // Formato: "1 Gen", "1 Feb", etc.
            const monthNames = ['Gen', 'Feb', 'Mar', 'Apr', 'Mag', 'Giu', 'Lug', 'Ago', 'Set', 'Ott', 'Nov', 'Dic'];
            const label = `1 ${monthNames[currentDate.getUTCMonth()]}`;

            // Sposta il testo per il primo marker
            const spanStyle = isFirst ? 'style="position: relative; left: 50%;"' : '';
            html += `<div class="timeline-hour-marker" style="left: ${percentage}%;"><span ${spanStyle}>${label}</span></div>`;

            isFirst = false;
            // Vai al primo del mese successivo
            currentDate = new Date(Date.UTC(currentDate.getUTCFullYear(), currentDate.getUTCMonth() + 1, 1));
        }
    }

    html += '</div>';

    // Timeline con una riga per progetto
    uniqueProjects.forEach(projectName => {
        const projectSessions = sessionsByProject[projectName] || [];
        const projectNotes = notesByProject[projectName] || [];
        const color = projectColors[projectName];

        // Label del progetto (con background per interrompere le linee dei marker)
        html += `<div style="margin-top: 15px; margin-bottom: 5px; font-weight: 600; color: #ffffff; position: relative; z-index: 200; background: #242424; padding: 5px 0;">`;
        html += `${projectName}`;
        html += `</div>`;

        // Timeline bar per questo progetto
        html += '<div class="timeline-bar">';

        // Aggiungi segmenti delle sessioni
        projectSessions.forEach(session => {
            const isOffComputer = session.SessionType === 'off-computer';
            const segment = createTimelineSegmentUnified(session, color, startTime, totalMs, isOffComputer);
            html += segment;
        });

        // Aggiungi marker per le note
        projectNotes.forEach(note => {
            const noteMarker = createNoteMarker(note, startTime, totalMs);
            html += noteMarker;
        });

        html += '</div>';
    });

    // Chiudi il wrapper delle timeline
    html += '</div>';

    // Statistiche riepilogo
    const projectStats = {};
    const activityStats = {};
    sessions.forEach(s => {
        const proj = s.ProjectName || 'Nessun progetto';
        if (!projectStats[proj]) {
            projectStats[proj] = 0;
            activityStats[proj] = {
                'RICERCA': 0,
                'PROGETTAZIONE': 0,
                'REALIZZAZIONE': 0,
                'Non specificato': 0
            };
        }
        projectStats[proj] += s.Seconds;

        // Aggiungi statistiche per tipo di attività
        const actType = s.ActivityType || 'Non specificato';
        if (!activityStats[proj][actType]) {
            activityStats[proj][actType] = 0;
        }
        activityStats[proj][actType] += s.Seconds;
    });

    html += '<div style="margin-top: 25px;">';
    html += '<h3 style="margin-bottom: 10px; color: #ffffff;">Riepilogo Tempo per Progetto</h3>';
    html += '<div class="stats-grid">';

    const totalSeconds = Object.values(projectStats).reduce((sum, sec) => sum + sec, 0);

    for (const [proj, seconds] of Object.entries(projectStats).sort((a, b) => b[1] - a[1])) {
        const hours = (seconds / 3600).toFixed(1);
        const percentage = ((seconds / totalSeconds) * 100).toFixed(1);

        // Calcola breakdown per tipo attività
        let activityBreakdown = '';
        const projActivities = activityStats[proj];
        const activityEntries = Object.entries(projActivities)
            .filter(([_, sec]) => sec > 0)
            .sort((a, b) => b[1] - a[1]);

        if (activityEntries.length > 0) {
            activityBreakdown = '<div style="margin-top: 8px; padding-left: 20px; font-size: 0.85em; color: #6b7280;">';
            activityEntries.forEach(([actType, actSeconds]) => {
                const actHours = (actSeconds / 3600).toFixed(1);
                const actPercentage = ((actSeconds / seconds) * 100).toFixed(0);
                activityBreakdown += `
                    <div style="display: flex; justify-content: space-between; align-items: center; padding: 3px 0;">
                        <span>${actType}</span>
                        <span style="color: #9ca3af;">${actHours}h (${actPercentage}%)</span>
                    </div>
                `;
            });
            activityBreakdown += '</div>';
        }

        html += `
            <div class="stat-item" style="flex-direction: column; align-items: stretch;">
                <div style="display: flex; justify-content: space-between; align-items: center;">
                    <span class="stat-label" style="display: flex; align-items: center;">
                        <strong style="color: #ffffff;">${proj}</strong>
                    </span>
                    <span class="stat-value">${hours}h (${percentage}%)</span>
                </div>
                ${activityBreakdown}
            </div>
        `;
    }
    html += '</div></div>';

    content.innerHTML = html;
}

function formatDate(dateStr) {
    const date = new Date(dateStr);
    return `${date.getDate()}/${date.getMonth() + 1}/${date.getFullYear()}`;
}

// Funzione per schiarire/scurire un colore hex
function adjustColor(color, percent) {
    // Converte hex in RGB
    const num = parseInt(color.replace('#', ''), 16);
    const r = (num >> 16) + Math.round(255 * percent);
    const g = ((num >> 8) & 0x00FF) + Math.round(255 * percent);
    const b = (num & 0x0000FF) + Math.round(255 * percent);

    // Limita i valori tra 0 e 255
    const newR = Math.max(0, Math.min(255, r));
    const newG = Math.max(0, Math.min(255, g));
    const newB = Math.max(0, Math.min(255, b));

    // Converte RGB in hex
    return '#' + ((newR << 16) | (newG << 8) | newB).toString(16).padStart(6, '0');
}

// Genera pattern CSS in base al tipo
function generatePatternStyle(color, pattern) {
    if (!pattern || pattern === 'solid') {
        return color;
    }

    switch (pattern) {
        case 'stripes':
            return `repeating-linear-gradient(45deg, ${color}, ${color} 10px, rgba(255,255,255,0.2) 10px, rgba(255,255,255,0.2) 20px)`;

        case 'dots':
            // Usa un radial-gradient per creare puntini
            return `radial-gradient(circle, rgba(255,255,255,0.3) 25%, transparent 25%),
                    radial-gradient(circle, rgba(255,255,255,0.3) 25%, transparent 25%),
                    ${color}`;

        case 'grid':
            return `linear-gradient(0deg, rgba(255,255,255,0.2) 1px, transparent 1px),
                    linear-gradient(90deg, rgba(255,255,255,0.2) 1px, transparent 1px),
                    ${color}`;

        case 'waves':
            return `repeating-linear-gradient(0deg, ${color} 0px, ${color} 10px, rgba(255,255,255,0.2) 10px, rgba(255,255,255,0.2) 20px)`;

        case 'zigzag':
            return `linear-gradient(135deg, ${color} 25%, transparent 25%),
                    linear-gradient(225deg, ${color} 25%, transparent 25%),
                    linear-gradient(45deg, ${color} 25%, transparent 25%),
                    linear-gradient(315deg, ${color} 25%, transparent 25%),
                    ${color}`;

        default:
            return color;
    }
}

// Genera background-size per pattern specifici
function getPatternBackgroundSize(pattern) {
    switch (pattern) {
        case 'dots':
            return '20px 20px, 20px 20px, 100% 100%';
        case 'grid':
            return '20px 20px';
        case 'zigzag':
            return '20px 20px';
        default:
            return 'auto';
    }
}

// Genera background-position per pattern specifici
function getPatternBackgroundPosition(pattern) {
    switch (pattern) {
        case 'dots':
            return '0 0, 10px 10px, 0 0';
        case 'zigzag':
            return '10px 0, 10px 0, 0 0, 0 0, 0 0';
        default:
            return '0 0';
    }
}

function createTimelineSegmentUnified(session, color, startTime, totalMs, isOffComputer = false) {
    // Parse timestamp della sessione
    if (!session.Timestamp) {
        console.error('Session senza timestamp!', session);
        return '';
    }

    let sessionTime;

    // Parse timestamp
    if (session.Timestamp.includes('T')) {
        // Formato ISO (es: "2025-11-18T21:15:15Z")
        // Se finisce con Z, è UTC e JavaScript lo converte automaticamente a local time
        sessionTime = new Date(session.Timestamp);
    } else {
        // Formato SQLite: "YYYY-MM-DD HH:MM:SS" (già in ora locale)
        const parts = session.Timestamp.split(' ');
        if (parts.length < 2) {
            console.error('Timestamp formato invalido:', session.Timestamp);
            return '';
        }
        const datePart = parts[0];
        const timePart = parts[1];

        // Parse come ora locale
        const [year, month, day] = datePart.split('-').map(Number);
        const [hours, minutes, seconds] = timePart.split(':').map(Number);
        sessionTime = new Date(year, month - 1, day, hours, minutes, seconds || 0);
    }

    if (isNaN(sessionTime.getTime())) {
        console.error('Data invalida:', session.Timestamp);
        return '';
    }

    // Calcola posizione relativa al periodo selezionato
    const sessionMs = sessionTime - startTime;
    const left = (sessionMs / totalMs) * 100;

    // Se è una sessione aggregata, usa TotalSeconds, altrimenti Seconds
    const totalSeconds = session.TotalSeconds || session.Seconds;
    const sessionDurationMs = totalSeconds * 1000;
    const width = (sessionDurationMs / totalMs) * 100;

    console.log('=== DEBUG TILE ===');
    console.log('Session timestamp raw:', session.Timestamp);
    console.log('Parsed sessionTime:', sessionTime.toString());
    console.log('UTC time:', sessionTime.toUTCString());
    console.log('Local time:', sessionTime.toLocaleString('it-IT'));
    console.log('getHours():', sessionTime.getHours(), 'getUTCHours():', sessionTime.getUTCHours());
    console.log('startTime:', startTime.toString(), 'UTC:', startTime.toUTCString());
    console.log('sessionMs:', sessionMs, 'totalMs:', totalMs);
    console.log('Tile left:', left.toFixed(2) + '%', 'width:', width.toFixed(2) + '%');

    // Formatta timestamp usando UTC (perché il database salva in UTC)
    const hours = sessionTime.getUTCHours();
    const minutes = sessionTime.getUTCMinutes();
    const timeStr = `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}`;
    const dateStr = `${sessionTime.getUTCDate()}/${sessionTime.getUTCMonth() + 1}`;

    // Nome progetto
    const projectName = session.ProjectName || 'Nessun progetto';

    // Modifica colore in base al tipo di attività usando i tipi configurabili
    let finalColor = color;
    const activityType = session.ActivityType || 'NESSUNA';
    let pattern = 'solid';

    // Cerca il tipo di attività nella configurazione per ottenere la variante di colore e il pattern
    const activityTypeConfig = activityTypes.find(t => t.Name === activityType);
    if (activityTypeConfig) {
        finalColor = adjustColor(color, activityTypeConfig.ColorVariant);
        pattern = activityTypeConfig.Pattern || 'solid';
    }

    // Stile per sessioni: usa pattern configurabile o strisce per off-computer
    const backgroundStyle = isOffComputer
        ? `repeating-linear-gradient(45deg, ${finalColor}, ${finalColor} 10px, rgba(255,255,255,0.3) 10px, rgba(255,255,255,0.3) 20px)`
        : generatePatternStyle(finalColor, pattern);

    const backgroundSize = isOffComputer ? 'auto' : getPatternBackgroundSize(pattern);
    const backgroundPosition = isOffComputer ? '0 0' : getPatternBackgroundPosition(pattern);

    // Costruisci lista programmi usati (se è una sessione aggregata)
    let programsList = '';
    if (session.Sessions && session.Sessions.length > 1) {
        // Conta occorrenze di ogni programma
        const appCounts = {};
        session.Sessions.forEach(s => {
            appCounts[s.AppName] = (appCounts[s.AppName] || 0) + 1;
        });
        programsList = '<br><strong>Programmi usati:</strong><br>';
        Object.entries(appCounts).forEach(([app, count]) => {
            programsList += `${app}${count > 1 ? ' (×' + count + ')' : ''}<br>`;
        });
    } else {
        programsList = `<br>${session.AppName || 'Nessuna app'}`;
    }

    // Costruisci testo per il tooltip nativo
    const tooltipText = `${activityType}\nInizio: ${dateStr} ${timeStr}\nDurata: ${Math.floor(totalSeconds / 60)} min ${totalSeconds % 60} sec\n${session.AppName ? session.AppName : 'Aggregato'}`;

    return `
        <div class="timeline-segment"
             style="left: ${left}%; width: ${Math.max(width, 0.5)}%; background: ${backgroundStyle}; background-size: ${backgroundSize}; background-position: ${backgroundPosition}; ${isOffComputer ? 'border: 2px solid rgba(0,0,0,0.2);' : ''} cursor: pointer;"
             title="${tooltipText.replace(/"/g, '&quot;')}"
             data-session-id="${session.ID}"
             data-seconds="${totalSeconds}"
             data-activity-type="${activityType}"
             data-project-name="${projectName.replace(/"/g, '&quot;')}"
             data-app-name="${session.AppName ? session.AppName.replace(/"/g, '&quot;') : 'Aggregato'}"
             data-timestamp="${dateStr} ${timeStr}"
             onclick="showSessionMenuFromElement(event, this)"
             oncontextmenu="event.preventDefault(); showSessionMenuFromElement(event, this);">
        </div>
    `;
}

function generateProjectColors(projects) {
    // Usa sempre bianco per tutte le tile dei programmi
    const projectColors = {};
    projects.forEach((project) => {
        projectColors[project] = '#ffffff';
    });

    return projectColors;
}

function createNoteMarker(note, startTime, totalMs) {
    // Parse timestamp della nota - usa lo stesso metodo delle sessioni
    // Il backend ora salva entrambi come timestamp locale in formato stringa
    let noteTime;
    if (note.Timestamp.includes('T')) {
        const utcDate = new Date(note.Timestamp);

        // Se finisce con 'Z', manteniamo i numeri senza convertire il fuso orario
        if (note.Timestamp.endsWith('Z')) {
            noteTime = new Date(
                utcDate.getUTCFullYear(),
                utcDate.getUTCMonth(),
                utcDate.getUTCDate(),
                utcDate.getUTCHours(),
                utcDate.getUTCMinutes(),
                utcDate.getUTCSeconds()
            );
        } else {
            noteTime = utcDate;
        }
    } else {
        const parts = note.Timestamp.split(' ');
        const datePart = parts[0];
        const timePart = parts[1];
        noteTime = new Date(datePart + 'T' + timePart);
    }

    // Calcola posizione relativa al periodo selezionato
    const noteMs = noteTime - startTime;
    const left = (noteMs / totalMs) * 100;

    console.log('Note marker:', {
        noteText: note.NoteText,
        timestamp: note.Timestamp,
        noteTime: noteTime.toISOString(),
        startTime: startTime.toISOString(),
        noteMs,
        totalMs,
        left: left.toFixed(2) + '%'
    });

    // Formatta timestamp
    const hours = noteTime.getHours();
    const minutes = noteTime.getMinutes();
    const timeStr = `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}`;
    const dateStr = `${noteTime.getDate()}/${noteTime.getMonth() + 1}`;

    // Tronca il testo della nota se troppo lungo
    const notePreview = note.NoteText.length > 100
        ? note.NoteText.substring(0, 100) + '...'
        : note.NoteText;

    // Costruisci testo per il tooltip nativo (stesso stile delle sessioni)
    const tooltipText = `📝 NOTA\n${dateStr} ${timeStr}\n\n${notePreview}`;

    return `
        <div class="note-marker" style="left: ${left}%;" title="${tooltipText.replace(/"/g, '&quot;')}">
            <!-- Linea verticale -->
            <div class="note-marker-line"></div>

            <!-- Icona nota sopra la timeline -->
            <div class="note-marker-icon" onclick="editNoteModal(${note.ID}, '${note.NoteText.replace(/'/g, "\\'").replace(/\n/g, '\\n')}', '${dateStr} ${timeStr}')">
                📝
            </div>
        </div>
    `;
}

// === GESTIONE IDLE TIME ===

async function checkForPendingIdle() {
    try {
        const response = await fetch('/api/tracking/idle-check');
        const data = await response.json();

        if (data.has_pending) {
            showIdleModal(data.idle_period);
        }
    } catch (error) {
        console.error('Errore check idle:', error);
    }
}

async function showIdleModal(idlePeriod) {
    const modal = document.getElementById('idleModal');
    const idleSelect = document.getElementById('idleProjectSelect');

    // Formatta durata
    const minutes = idlePeriod.minutes;
    const hours = Math.floor(minutes / 60);
    const remainingMins = minutes % 60;
    let durationText = '';
    if (hours > 0) {
        durationText = `${hours}h ${remainingMins}min`;
    } else {
        durationText = `${minutes} minuti`;
    }

    document.getElementById('idleDuration').textContent = durationText;
    document.getElementById('idlePeriod').textContent =
        `${idlePeriod.start_time} - ${idlePeriod.end_time}`;

    // Popola select con progetti
    try {
        const response = await fetch('/api/projects');
        const projects = await response.json();

        idleSelect.innerHTML = '<option value="">Seleziona progetto...</option>';
        projects.forEach(project => {
            const option = document.createElement('option');
            option.value = project.Name;
            option.textContent = project.Name;
            idleSelect.appendChild(option);
        });
    } catch (error) {
        console.error('Errore caricamento progetti:', error);
    }

    // Mostra modal
    modal.classList.add('show');
}

async function attributeIdleToProject() {
    const projectName = document.getElementById('idleProjectSelect').value;

    if (!projectName) {
        showNotification('Seleziona un progetto', 'error');
        return;
    }

    try {
        const response = await fetch('/api/tracking/attribute-idle', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                project_name: projectName,
                is_break: false
            })
        });

        if (response.ok) {
            showNotification(`Tempo idle attribuito a: ${projectName}`, 'success');
            hideIdleModal();
            loadTodayStats();
            loadTimeline();
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore attribuzione idle:', error);
        showNotification('Errore attribuzione tempo idle', 'error');
    }
}

async function attributeIdleAsBreak() {
    try {
        const response = await fetch('/api/tracking/attribute-idle', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                project_name: '',
                is_break: true
            })
        });

        if (response.ok) {
            showNotification('Tempo idle registrato come pausa', 'success');
            hideIdleModal();
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore attribuzione pausa:', error);
        showNotification('Errore attribuzione pausa', 'error');
    }
}

function hideIdleModal() {
    const modal = document.getElementById('idleModal');
    modal.classList.remove('show');
}

// === GESTIONE NOTE ===

async function openNoteModal() {
    const modal = document.getElementById('noteModal');
    const noteSelect = document.getElementById('noteProjectSelect');

    // Popola select con progetti
    try {
        const response = await fetch('/api/projects');
        const projects = await response.json();

        noteSelect.innerHTML = '<option value="">Seleziona progetto...</option>';
        projects.forEach(project => {
            const option = document.createElement('option');
            option.value = project.Name;
            option.textContent = project.Name;
            noteSelect.appendChild(option);
        });
    } catch (error) {
        console.error('Errore caricamento progetti:', error);
        showNotification('Errore caricamento progetti', 'error');
        return;
    }

    // Reset textarea
    document.getElementById('noteText').value = '';

    // Mostra modal
    modal.classList.add('show');
}

function closeNoteModal() {
    const modal = document.getElementById('noteModal');
    modal.classList.remove('show');
}

async function saveNote() {
    const projectName = document.getElementById('noteProjectSelect').value;
    const noteText = document.getElementById('noteText').value.trim();

    if (!projectName) {
        showNotification('Seleziona un progetto', 'error');
        return;
    }

    if (!noteText) {
        showNotification('Inserisci il testo della nota', 'error');
        return;
    }

    try {
        // Crea timestamp locale nello stesso formato delle sessioni
        const now = new Date();
        const year = now.getFullYear();
        const month = String(now.getMonth() + 1).padStart(2, '0');
        const day = String(now.getDate()).padStart(2, '0');
        const hours = String(now.getHours()).padStart(2, '0');
        const minutes = String(now.getMinutes()).padStart(2, '0');
        const seconds = String(now.getSeconds()).padStart(2, '0');
        const timestamp = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;

        const response = await fetch('/api/notes/create', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                project_name: projectName,
                note_text: noteText,
                timestamp: timestamp
            })
        });

        if (response.ok) {
            showNotification('Nota creata con successo!', 'success');
            closeNoteModal();
            loadTimeline(); // Ricarica timeline per mostrare la nota
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore creazione nota:', error);
        showNotification('Errore creazione nota', 'error');
    }
}

// === MODIFICA E ELIMINAZIONE NOTE ===

function editNoteModal(noteId, noteText, timestamp) {
    document.getElementById('editNoteId').value = noteId;
    document.getElementById('editNoteText').value = noteText;
    document.getElementById('editNoteTimestamp').textContent = timestamp;
    document.getElementById('editNoteModal').style.display = 'flex';
}

function closeEditNoteModal() {
    document.getElementById('editNoteModal').style.display = 'none';
}

async function updateNote() {
    const noteId = document.getElementById('editNoteId').value;
    const noteText = document.getElementById('editNoteText').value.trim();

    if (!noteText) {
        showNotification('Inserisci il testo della nota', 'error');
        return;
    }

    try {
        const response = await fetch('/api/notes/update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                id: parseInt(noteId),
                note_text: noteText
            })
        });

        if (response.ok) {
            showNotification('Nota aggiornata!', 'success');
            closeEditNoteModal();
            loadTimeline(); // Ricarica timeline per mostrare la modifica
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore aggiornamento nota:', error);
        showNotification('Errore aggiornamento nota', 'error');
    }
}

async function deleteNoteFromModal() {
    const noteId = document.getElementById('editNoteId').value;

    if (!confirm('Vuoi davvero eliminare questa nota?')) {
        return;
    }

    try {
        const response = await fetch('/api/notes/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                id: parseInt(noteId)
            })
        });

        if (response.ok) {
            showNotification('Nota eliminata!', 'success');
            closeEditNoteModal();
            loadTimeline(); // Ricarica timeline
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore eliminazione nota:', error);
        showNotification('Errore eliminazione nota', 'error');
    }
}

// Funzione per modificare il tipo di attività di una sessione
async function editActivityType(sessionID, currentActivityType, projectName) {
    // Usa i tipi di attività configurabili
    const activities = ['', ...activityTypes.map(t => t.Name)];
    let message = `Modifica tipo attività per: ${projectName}\n\nAttuale: ${currentActivityType || 'Non specificato'}\n\nSeleziona nuovo tipo:`;
    const options = activities.map((a, i) => `${i}. ${a || 'Nessuno'}`).join('\n');
    const choice = prompt(message + '\n\n' + options + `\n\nInserisci numero (0-${activities.length - 1}):`);

    if (choice === null) return; // Annullato

    const index = parseInt(choice);
    if (isNaN(index) || index < 0 || index >= activities.length) {
        showNotification('Scelta non valida', 'error');
        return;
    }

    const newActivityType = activities[index] || null;

    try {
        const response = await fetch('/api/sessions/update-activity', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                session_id: sessionID,
                activity_type: newActivityType
            })
        });

        if (response.ok) {
            showNotification('Tipo attività aggiornato\!', 'success');
            loadTimeline(); // Ricarica timeline
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore aggiornamento tipo attività:', error);
        showNotification('Errore aggiornamento', 'error');
    }
}

// Funzione wrapper che legge i dati dall'elemento
function showSessionMenuFromElement(event, element) {
    console.log('Click su tile!', element);
    console.log('Dataset:', element.dataset);

    const sessionID = parseInt(element.dataset.sessionId);
    const seconds = parseInt(element.dataset.seconds);
    const activityType = element.dataset.activityType;
    const projectName = element.dataset.projectName;

    console.log('Dati estratti:', { sessionID, seconds, activityType, projectName });

    try {
        showSessionMenu(event, sessionID, seconds, activityType, projectName);
    } catch (error) {
        console.error('Errore in showSessionMenu:', error);
    }
}

// Funzione per mostrare il menu contestuale per la sessione
function showSessionMenu(event, sessionID, seconds, activityType, projectName) {
    console.log('showSessionMenu INIZIO', { event, sessionID, seconds, activityType, projectName });
    event.stopPropagation();

    console.log('1. stopPropagation OK');

    // Rimuovi menu esistente se presente
    closeContextMenu();

    console.log('2. closeContextMenu OK');

    // Crea il menu - USA clientX/clientY per position:fixed
    const menuDiv = document.createElement('div');
    menuDiv.id = 'contextMenu';
    menuDiv.style.cssText = `position: fixed; left: ${event.clientX}px; top: ${event.clientY}px; background: white; border: 1px solid #ccc; border-radius: 4px; box-shadow: 0 4px 20px rgba(0,0,0,0.3); z-index: 999999; min-width: 200px; max-width: 300px; padding: 0;`;

    console.log('3. menuDiv creato, posizione:', event.clientX, event.clientY);
    console.log('3a. menuDiv style:', menuDiv.style.cssText);

    // Header
    const header = document.createElement('div');
    header.style.cssText = 'padding: 8px; border-bottom: 1px solid #eee; background: #f5f5f5; font-weight: bold;';
    header.textContent = projectName;
    menuDiv.appendChild(header);

    console.log('4. header aggiunto');

    // Container pulsanti
    const buttonsDiv = document.createElement('div');
    buttonsDiv.style.cssText = 'padding: 4px 0;';

    console.log('5. buttonsDiv creato');

    // Funzione helper per creare pulsanti
    const createButton = (icon, text, onClick, isDelete = false) => {
        const btn = document.createElement('button');
        btn.style.cssText = `width: 100%; padding: 8px 12px; border: none; background: none; text-align: left; cursor: pointer; ${isDelete ? 'color: #d32f2f;' : ''}`;
        btn.innerHTML = `${icon} ${text}`;
        btn.onmouseover = () => btn.style.background = isDelete ? '#ffebee' : '#f0f0f0';
        btn.onmouseout = () => btn.style.background = 'none';
        btn.onclick = () => {
            closeContextMenu();
            onClick();
        };
        return btn;
    };

    console.log('6. createButton definito');

    // Aggiungi pulsanti
    console.log('7. inizio creazione pulsanti...');
    buttonsDiv.appendChild(createButton('✏️', 'Modifica tipo attività', () => editActivityType(sessionID, activityType, projectName)));
    console.log('7a. pulsante 1 OK');
    buttonsDiv.appendChild(createButton('⏱️', 'Modifica durata', () => editSessionDuration(sessionID, seconds, projectName)));
    console.log('7b. pulsante 2 OK');
    buttonsDiv.appendChild(createButton('✂️', 'Dividi sessione', () => splitSession(sessionID, seconds, activityType, projectName)));
    console.log('7c. pulsante 3 OK');

    // Separatore
    const hr = document.createElement('hr');
    hr.style.cssText = 'margin: 4px 0; border: none; border-top: 1px solid #eee;';
    buttonsDiv.appendChild(hr);

    console.log('8. separatore aggiunto');

    // Pulsante elimina
    buttonsDiv.appendChild(createButton('🗑️', 'Elimina sessione', () => deleteSession(sessionID, projectName), true));

    console.log('9. pulsante elimina aggiunto');

    menuDiv.appendChild(buttonsDiv);
    console.log('10. buttonsDiv aggiunto a menuDiv');

    // Impedisci che i click sul menu lo chiudano
    menuDiv.onclick = (e) => {
        e.stopPropagation();
        console.log('Click sul menu bloccato');
    };

    document.body.appendChild(menuDiv);
    console.log('11. MENU AGGIUNTO AL BODY! Dovrebbe essere visibile');

    // IMPORTANTE: Ferma completamente la propagazione dell'evento per evitare che il click
    // che ha aperto il menu lo chiuda immediatamente
    event.preventDefault();

    // Chiudi il menu cliccando altrove - con timeout più lungo
    setTimeout(() => {
        console.log('12. Aggiunto listener per chiudere menu');
        document.addEventListener('click', () => {
            console.log('13. Click rilevato, chiudo menu');
            closeContextMenu();
        }, { once: true });
    }, 200);

    console.log('14. showSessionMenu FINITO');
}

function closeContextMenu() {
    const menu = document.getElementById('contextMenu');
    if (menu) {
        menu.remove();
    }
}

// Funzione per eliminare una sessione
async function deleteSession(sessionID, projectName) {
    if (!confirm(`Vuoi davvero eliminare questa sessione di ${projectName}?`)) {
        return;
    }

    try {
        const response = await fetch('/api/sessions/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ session_id: sessionID })
        });

        if (response.ok) {
            showNotification('Sessione eliminata!', 'success');
            loadTimeline();
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore eliminazione sessione:', error);
        showNotification('Errore eliminazione', 'error');
    }
}

// Funzione per modificare la durata di una sessione
async function editSessionDuration(sessionID, currentSeconds, projectName) {
    const currentMinutes = Math.floor(currentSeconds / 60);
    const currentRemainingSeconds = currentSeconds % 60;

    const input = prompt(
        `Modifica durata per: ${projectName}\n\nDurata attuale: ${currentMinutes} min ${currentRemainingSeconds} sec\n\nInserisci nuova durata in minuti:`,
        currentMinutes
    );

    if (input === null) return;

    const newMinutes = parseInt(input);
    if (isNaN(newMinutes) || newMinutes <= 0) {
        showNotification('Durata non valida', 'error');
        return;
    }

    const newSeconds = newMinutes * 60;

    try {
        const response = await fetch('/api/sessions/update-duration', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                session_id: sessionID,
                seconds: newSeconds
            })
        });

        if (response.ok) {
            showNotification('Durata aggiornata!', 'success');
            loadTimeline();
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore aggiornamento durata:', error);
        showNotification('Errore aggiornamento', 'error');
    }
}

// Funzione per dividere una sessione
async function splitSession(sessionID, totalSeconds, currentActivityType, projectName) {
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

    // Chiedi i tipi di attività per entrambe le parti - usa i tipi configurabili
    const activities = ['', ...activityTypes.map(t => t.Name)];
    const activityOptions = activities.map((a, i) => `${i}. ${a || 'Nessuno'}`).join('\n');

    const firstActivityChoice = prompt(
        `Tipo attività PRIMA PARTE (${firstPartMinutes} min):\n\n${activityOptions}\n\nInserisci numero (0-${activities.length - 1}):`,
        currentActivityType ? activities.indexOf(currentActivityType).toString() : '0'
    );

    if (firstActivityChoice === null) return;

    const firstActivityIndex = parseInt(firstActivityChoice);
    if (isNaN(firstActivityIndex) || firstActivityIndex < 0 || firstActivityIndex >= activities.length) {
        showNotification('Scelta non valida', 'error');
        return;
    }

    const secondActivityChoice = prompt(
        `Tipo attività SECONDA PARTE (${secondPartMinutes} min):\n\n${activityOptions}\n\nInserisci numero (0-${activities.length - 1}):`,
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
        const response = await fetch('/api/sessions/split', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                session_id: sessionID,
                first_part_seconds: firstPartSeconds,
                first_part_activity_type: firstActivityType,
                second_part_activity_type: secondActivityType
            })
        });

        if (response.ok) {
            showNotification('Sessione divisa!', 'success');
            loadTimeline();
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore divisione sessione:', error);
        showNotification('Errore divisione', 'error');
    }
}

// Funzione per creare una nuova sessione manuale
async function createNewSession() {
    // Carica lista progetti
    const projectsResponse = await fetch('/api/projects');
    const projects = await projectsResponse.json();

    // Popola radio button progetti
    const projectRadioList = document.getElementById('projectRadioList');
    let projectsHtml = '<div style="display: flex; flex-direction: column; gap: 8px;">';
    projects.forEach((project, index) => {
        const checked = index === 0 ? 'checked' : '';
        projectsHtml += `
            <label style="display: flex; justify-content: space-between; align-items: center; cursor: pointer; padding: 12px 15px; border: 1px solid #4a4a4a; border-radius: 6px; transition: all 0.2s; background: #1a1a1a;" onmouseover="this.style.background='#2a2a2a'" onmouseout="this.style.background='#1a1a1a'">
                <span style="color: #ffffff; font-weight: 500;">${project.Name}</span>
                <input type="radio" name="projectSelect" value="${project.ID}" ${checked} style="width: 18px; height: 18px; cursor: pointer; margin: 0;">
            </label>
        `;
    });
    projectsHtml += '</div>';
    projectRadioList.innerHTML = projectsHtml;

    // Popola select tipo attività
    const activitySelect = document.getElementById('sessionActivityType');
    activitySelect.innerHTML = '<option value="">Nessuno</option>';
    activityTypes.forEach(type => {
        activitySelect.innerHTML += `<option value="${type.Name}">${type.Name}</option>`;
    });

    // Popola select ore (0-23)
    const hourSelect = document.getElementById('sessionHour');
    hourSelect.innerHTML = '';
    for (let h = 0; h < 24; h++) {
        const option = document.createElement('option');
        option.value = h;
        option.textContent = String(h).padStart(2, '0');
        hourSelect.appendChild(option);
    }

    // Popola select minuti (0-59)
    const minuteSelect = document.getElementById('sessionMinute');
    minuteSelect.innerHTML = '';
    for (let m = 0; m < 60; m++) {
        const option = document.createElement('option');
        option.value = m;
        option.textContent = String(m).padStart(2, '0');
        minuteSelect.appendChild(option);
    }

    // Imposta data/ora corrente
    const now = new Date();
    const year = now.getFullYear();
    const month = String(now.getMonth() + 1).padStart(2, '0');
    const day = String(now.getDate()).padStart(2, '0');
    const hours = now.getHours();
    const minutes = now.getMinutes();

    // Imposta data e ora separatamente
    document.getElementById('sessionDate').value = `${year}-${month}-${day}`;
    hourSelect.value = hours;
    minuteSelect.value = minutes;

    // Mostra il modale
    document.getElementById('createSessionModal').style.display = 'flex';
}

function closeCreateSessionModal() {
    document.getElementById('createSessionModal').style.display = 'none';
}

async function saveNewSession() {
    // Leggi i valori dal form
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
    // Combina data e ora nel formato database (YYYY-MM-DD HH:MM:SS)
    const hours = String(hourValue).padStart(2, '0');
    const minutes = String(minuteValue).padStart(2, '0');
    const timestamp = `${dateValue} ${hours}:${minutes}:00`;

    // Tipo sessione sempre "computer" per sessioni manuali
    const sessionType = 'computer';

    const activityType = document.getElementById('sessionActivityType').value || null;

    // Invia richiesta
    try {
        const response = await fetch('/api/sessions/create', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                app_name: 'Manuale',
                seconds: seconds,
                project_id: projectId,
                session_type: sessionType,
                activity_type: activityType,
                timestamp: timestamp
            })
        });

        if (response.ok) {
            showNotification('Nuova sessione creata!', 'success');
            closeCreateSessionModal();
            loadTimeline(); // Ricarica timeline
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore creazione sessione:', error);
        showNotification('Errore creazione sessione', 'error');
    }
}

// === REPORT DI CHIUSURA ===

function showReportModal(report) {
    const modal = document.getElementById('reportModal');
    const content = document.getElementById('reportContent');

    // Formatta le date
    const startDate = report.start_date ? new Date(report.start_date).toLocaleDateString('it-IT') : 'N/A';
    const endDate = report.end_date ? new Date(report.end_date).toLocaleDateString('it-IT') : 'N/A';
    const closedAt = report.closed_at ? new Date(report.closed_at).toLocaleString('it-IT') : 'N/A';

    // Formatta la suddivisione per attività
    let activityBreakdownHTML = '';
    if (report.activity_breakdown) {
        activityBreakdownHTML = Object.entries(report.activity_breakdown)
            .map(([activity, hours]) => `
                <div style="display: flex; justify-content: space-between; padding: 12px; background: #1a1a1a; border-radius: 8px; margin-bottom: 8px; border: 1px solid #3a3a3a;">
                    <span style="color: #ffffff;"><strong>${activity}</strong></span>
                    <span style="color: #ff6b2b; font-weight: 600;">${hours.toFixed(2)} ore</span>
                </div>
            `).join('');
    }

    content.innerHTML = `
        <div style="background: #1a1a1a; padding: 20px; border-radius: 8px; margin-bottom: 15px; border: 1px solid #3a3a3a;">
            <h3 style="color: #ff6b2b; margin-bottom: 10px;">${report.project_name}</h3>
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
            <p style="font-size: 2em; font-weight: bold; color: #ff6b2b;">${report.total_hours.toFixed(2)} ore</p>
        </div>

        <div>
            <h3 style="color: #ffffff; margin-bottom: 10px;">Suddivisione per Tipo di Attività</h3>
            ${activityBreakdownHTML || '<p style="color: #999999;">Nessun dato disponibile</p>'}
        </div>
    `;

    modal.style.display = 'flex';
}

function closeReportModal() {
    document.getElementById('reportModal').style.display = 'none';
}

// === TIPI DI ATTIVITÀ (caricamento per uso nelle funzioni) ===

let activityTypes = [];

// Carica tipi di attività dal server
async function loadActivityTypes() {
    try {
        const response = await fetch('/api/activity-types');
        activityTypes = await response.json();
    } catch (error) {
        console.error('Errore caricamento tipi attività:', error);
    }
}

// Carica tipi al caricamento pagina
document.addEventListener('DOMContentLoaded', () => {
    loadActivityTypes();
    loadAllNotes(); // Carica le note all'avvio
    populateNotesProjectFilter(); // Popola il filtro progetti

    // Popup di conferma alla chiusura della pagina
    window.addEventListener('beforeunload', function(e) {
        e.preventDefault();
        e.returnValue = 'Sei sicuro di voler chiudere PrendiTempo?';
        return e.returnValue;
    });
});

// === ELENCO NOTE ===

// Popola il filtro progetti per le note
async function populateNotesProjectFilter() {
    try {
        const filterSelect = document.getElementById('notesFilterProject');

        // Verifica che l'elemento esista
        if (!filterSelect) {
            console.warn('Elemento notesFilterProject non ancora disponibile');
            return;
        }

        const response = await fetch('/api/projects');
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const projects = await response.json();
        filterSelect.innerHTML = '<option value="">Tutti i progetti</option>';

        projects.forEach(project => {
            filterSelect.innerHTML += `<option value="${project.ID}">${project.Name}</option>`;
        });
    } catch (error) {
        console.error('Errore caricamento progetti per filtro note:', error);
    }
}

// Carica tutte le note con filtri
async function loadAllNotes() {
    try {
        const filterElement = document.getElementById('notesFilterProject');
        const searchElement = document.getElementById('notesSearchText');
        const contentElement = document.getElementById('notesListContent');

        // Verifica che gli elementi esistano
        if (!filterElement || !searchElement || !contentElement) {
            console.warn('Elementi note non ancora disponibili');
            return;
        }

        const projectID = filterElement.value || '';
        const searchText = searchElement.value || '';

        let url = '/api/notes/all?limit=50';
        if (projectID) url += `&project_id=${projectID}`;
        if (searchText) url += `&search=${encodeURIComponent(searchText)}`;

        const response = await fetch(url);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const notes = await response.json();
        displayNotesList(notes);
    } catch (error) {
        console.error('Errore caricamento note:', error);
        const contentElement = document.getElementById('notesListContent');
        if (contentElement) {
            contentElement.innerHTML = `<p style="color: #ef4444;">Errore caricamento note: ${error.message}</p>`;
        }
    }
}

// Visualizza le note nell'elenco
async function displayNotesList(notes) {
    const container = document.getElementById('notesListContent');

    if (!notes || notes.length === 0) {
        container.innerHTML = '<p style="color: #6b7280;">Nessuna nota trovata</p>';
        return;
    }

    // Carica i nomi dei progetti
    const projectsResponse = await fetch('/api/projects');
    const projects = await projectsResponse.json();
    const projectsMap = {};
    projects.forEach(p => projectsMap[p.ID] = p.Name);

    let html = '<div style="display: flex; flex-direction: column; gap: 12px;">';

    notes.forEach(note => {
        const projectName = projectsMap[note.ProjectID] || 'Progetto sconosciuto';
        const timestamp = new Date(note.Timestamp);
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
                        <button onclick="editNoteFromList(${note.ID}, '${note.NoteText.replace(/'/g, "\\'")}', ${note.ProjectID})"
                                style="padding: 5px 12px; background: #ff6b2b; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 0.85em;">
                            Modifica
                        </button>
                        <button onclick="deleteNoteFromList(${note.ID})"
                                style="padding: 5px 12px; background: #ef4444; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 0.85em;">
                            Elimina
                        </button>
                    </div>
                </div>
                <p style="color: #ffffff; margin: 0; white-space: pre-wrap;">${note.NoteText}</p>
            </div>
        `;
    });

    html += '</div>';
    container.innerHTML = html;
}

// Modifica nota dall'elenco
function editNoteFromList(noteID, noteText, projectID) {
    // Riutilizza la funzione esistente di modifica nota
    document.getElementById('editNoteId').value = noteID;
    document.getElementById('editNoteText').value = noteText;
    document.getElementById('editNoteModal').style.display = 'flex';
}

// Elimina nota dall'elenco
async function deleteNoteFromList(noteID) {
    if (!confirm('Vuoi davvero eliminare questa nota?')) {
        return;
    }

    try {
        const response = await fetch('/api/notes/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: noteID })
        });

        if (response.ok) {
            showNotification('Nota eliminata!', 'success');
            loadAllNotes(); // Ricarica l'elenco
        } else {
            showNotification('Errore eliminazione nota', 'error');
        }
    } catch (error) {
        console.error('Errore eliminazione nota:', error);
        showNotification('Errore eliminazione nota', 'error');
    }
}
