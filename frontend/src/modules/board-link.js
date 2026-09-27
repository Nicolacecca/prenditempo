import { SetProjectBoardURL, OpenProjectBoardURL } from '../wailsjs/go/main/App.js';
import { showNotification } from '../notifications.js';
import { loadProjects } from './projects.js';

const EXCALIDRAW_NEW_BOARD_URL = 'https://excalidraw.com';

window.openProjectBoard = function(projectID, projectName, boardURL) {
    document.getElementById('projectBoardProjectId').value = projectID;
    document.getElementById('projectBoardProjectName').textContent = projectName || '';
    document.getElementById('projectBoardUrl').value = boardURL || '';
    document.getElementById('projectBoardModal').classList.add('show');
};

window.closeProjectBoard = function() {
    document.getElementById('projectBoardModal').classList.remove('show');
};

window.saveProjectBoardUrl = async function() {
    const projectID = parseInt(document.getElementById('projectBoardProjectId').value);
    const boardURL = document.getElementById('projectBoardUrl').value.trim();

    try {
        await SetProjectBoardURL(projectID, boardURL);
        showNotification('Lavagna collegata con successo!', 'success');
        closeProjectBoard();
        await loadProjects();
    } catch (error) {
        console.error('Errore salvataggio URL lavagna:', error);
        showNotification('Errore: ' + error, 'error');
    }
};

window.openProjectBoardUrl = async function() {
    const boardURL = document.getElementById('projectBoardUrl').value.trim();
    if (!boardURL) {
        showNotification('Inserisci prima un URL', 'error');
        return;
    }

    try {
        await OpenProjectBoardURL(boardURL);
    } catch (error) {
        console.error('Errore apertura lavagna:', error);
        showNotification('Errore: ' + error, 'error');
    }
};

window.createExcalidrawBoard = async function() {
    try {
        await OpenProjectBoardURL(EXCALIDRAW_NEW_BOARD_URL);
    } catch (error) {
        console.error('Errore apertura Excalidraw:', error);
        showNotification('Errore: ' + error, 'error');
    }
};
