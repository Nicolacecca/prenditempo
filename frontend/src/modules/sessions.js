import {
    CreateSession,
    DeleteSession,
    GetSessionById,
    SplitSession,
    UpdateSessionActivityType,
    UpdateSessionComplete,
    UpdateSessionDuration
} from '../wailsjs/go/main/App.js';
import { showNotification } from '../notifications.js';
import { escapeHtml } from '../utils.js';

let getActivityTypes = () => [];
let getProjects = () => [];
let refreshTimeline = async () => {};

export function configureSessionModule(options) {
    getActivityTypes = options.getActivityTypes;
    getProjects = options.getProjects;
    refreshTimeline = options.loadTimeline;
}

// === SESSIONI ===

window.createNewSession = async function() {
    const projectRadioList = document.getElementById('projectRadioList');
    let projectsHtml = '<div style="display: flex; flex-direction: column; gap: 8px;">';
    getProjects().forEach((project, index) => {
        const checked = index === 0 ? 'checked' : '';
        projectsHtml += `
            <label style="display: flex; justify-content: space-between; align-items: center; cursor: pointer; padding: 12px 15px; border: 1px solid #443A2E; border-radius: 6px; transition: all 0.2s; background: #1C1815;">
                <span style="color: #E4DACB; font-weight: 500;">${escapeHtml(project.name)}</span>
                <input type="radio" name="projectSelect" value="${project.id}" ${checked} style="width: 18px; height: 18px; cursor: pointer; margin: 0;">
            </label>
        `;
    });
    projectsHtml += '</div>';
    projectRadioList.innerHTML = projectsHtml;

    // Popola select tipo attività
    const activitySelect = document.getElementById('sessionActivityType');
    activitySelect.innerHTML = '<option value="">Nessuno</option>';
    getActivityTypes().forEach(type => {
        activitySelect.innerHTML += `<option value="${escapeHtml(type.name)}">${escapeHtml(type.name)}</option>`;
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
        document.getElementById(id).onchange = updateDuration;
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
        await refreshTimeline();
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
    getActivityTypes().forEach(type => {
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
        await refreshTimeline();
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
    getActivityTypes().forEach(type => {
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
        await refreshTimeline();
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
        await refreshTimeline();
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

        getActivityTypes().forEach(type => {
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
            await refreshTimeline();
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
            await refreshTimeline();
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
