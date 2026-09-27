import {
    ArchiveProject,
    CreateProject,
    GetProjects,
    UpdateProject
} from '../wailsjs/go/main/App.js';
import { showNotification } from '../notifications.js';
import { escapeHtml, validateLength } from '../utils.js';

const MAX_PROJECT_NAME_LENGTH = 100;
const MAX_PROJECT_DESC_LENGTH = 500;
let projectsCache = [];

export function getProjects() {
    return projectsCache;
}

// === PROGETTI ===

export async function loadProjects() {
    try {
        projectsCache = await GetProjects();
        displayProjects(projectsCache);
        populateProjectSelect(projectsCache);
    } catch (error) {
        console.error('Errore caricamento progetti:', error);
        showNotification('Errore caricamento progetti', 'error');
    }
}

export function displayProjects(projects) {
    const projectList = document.getElementById('projectList');

    if (!projects || projects.length === 0) {
        projectList.innerHTML = '<p style="color: #6b7280;">Nessun progetto. Creane uno nuovo!</p>';
        return;
    }

    projectList.innerHTML = projects.map(project => {
        return `
        <div class="project-item">
            <div>
                <h3>${escapeHtml(project.name)}</h3>
                ${project.description ? `<p>${escapeHtml(project.description)}</p>` : ''}
            </div>
            <div style="display: flex; gap: 10px;">
                <button class="btn" style="width: auto; padding: 8px 16px; margin: 0;"
                        data-project-action="board" data-project-id="${Number(project.id)}"
                        title="Apri la lavagna del progetto">▦ Lavagna</button>
                <button class="btn" style="width: auto; padding: 8px 16px; margin: 0;"
                        data-project-action="edit" data-project-id="${Number(project.id)}">Modifica</button>
            </div>
        </div>
    `;
    }).join('');

    projectList.querySelectorAll('[data-project-action]').forEach(button => {
        button.addEventListener('click', () => {
            const project = projects.find(item => item.id === Number(button.dataset.projectId));
            if (!project) return;
            if (button.dataset.projectAction === 'board') {
                window.openProjectBoard(project.id, project.name, project.note_text || '');
            } else {
                window.openEditProjectModal(project.id, project.name, project.description || '');
            }
        });
    });
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
