import {
    AttributeIdle,
    BringWindowToFront,
    CheckIdlePeriod,
    GetTrackingState,
    RestoreNormalWindow,
    StartTracking,
    StopTracking
} from '../wailsjs/go/main/App.js';
import { EventsOn } from '../wailsjs/runtime/runtime.js';
import { showNotification } from '../notifications.js';
import { getProjects } from './projects.js';

let isCurrentlyTracking = false;
let statusCheckInProgress = false;
let statusPollTimer = null;
let refreshTimeline = async () => {};

export async function initializeTracking(loadTimeline) {
    refreshTimeline = loadTimeline;
    await checkTrackingStatus();
    scheduleTrackingStatusCheck();

    EventsOn('tracking-auto-stopped', async data => {
        isCurrentlyTracking = false;
        updateUIForTracking(false);
        document.getElementById('statusText').textContent = 'Pronto';
        const minutes = Math.floor((data.seconds || 0) / 60);
        showNotification(`Tracking fermato automaticamente (${minutes} min salvati)`, 'success');
        await refreshTimeline();
    });

    EventsOn('idle-returned', async () => {
        await checkTrackingStatus();
    });
}

function scheduleTrackingStatusCheck() {
    if (statusPollTimer !== null) clearTimeout(statusPollTimer);
    statusPollTimer = setTimeout(async () => {
        await checkTrackingStatus();
        scheduleTrackingStatusCheck();
    }, isCurrentlyTracking ? 2000 : 15000);
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
        scheduleTrackingStatusCheck();
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
        scheduleTrackingStatusCheck();
        await refreshTimeline();
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
    getProjects().forEach(project => {
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
        const projectName = getProjects().find(p => p.id === projectID)?.name || 'Progetto';
        showNotification(`Tempo idle attribuito a: ${projectName}`, 'success');
        hideIdleModal();
        await refreshTimeline();
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
        trackingBtn.style.background = '#B1583F';
        trackingBtn.style.color = '#E4DACB';
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
