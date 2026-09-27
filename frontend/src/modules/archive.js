import {
    GetArchivedProjects,
    GetProjectReport,
    ReactivateProject
} from '../wailsjs/go/main/App.js';
import { showNotification } from '../notifications.js';
import { escapeHtml } from '../utils.js';
import { loadProjects } from './projects.js';
import { showReportModal } from './project-report.js';

// === GESTIONE ARCHIVIO ===

export async function loadArchivedProjects() {
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
		const escapedName = escapeHtml(project.name);
		const escapedDescription = escapeHtml(project.description || '');
		const escapedClosedAt = escapeHtml(closedAtStr);

        return `
            <div class="project-item" style="background: #1C1815; padding: 15px; border-radius: 10px; margin-bottom: 10px; display: flex; justify-content: space-between; align-items: center; border: 1px solid #322B24;">
                <div>
                    <h3 style="color: #E4DACB; font-size: 1.1em;">${escapedName}</h3>
                    ${project.description ? `<p style="color: #9C9086; font-size: 0.9em; margin-top: 5px;">${escapedDescription}</p>` : ''}
                    ${closedAtStr ? `<p style="color: #6B6259; font-size: 0.8em; margin-top: 5px;">Chiuso il: ${escapedClosedAt}</p>` : ''}
                </div>
                <div style="display: flex; gap: 10px;">
                    <button class="btn" style="padding: 8px 16px; margin: 0;" data-archive-action="board" data-project-id="${Number(project.id)}">Lavagna</button>
                    <button class="btn" style="padding: 8px 16px; margin: 0;" data-archive-action="report" data-project-id="${Number(project.id)}">Report</button>
					<button class="btn btn-success" style="padding: 8px 16px; margin: 0;" data-archive-action="reactivate" data-project-id="${Number(project.id)}">Riattiva</button>
					<button class="btn btn-danger" style="padding: 8px 16px; margin: 0;" data-archive-action="delete" data-project-id="${Number(project.id)}">Elimina</button>
                </div>
            </div>
    `;
    }).join('');

    container.querySelectorAll('[data-archive-action]').forEach(button => {
        button.addEventListener('click', () => {
            const project = projects.find(item => item.id === Number(button.dataset.projectId));
            if (!project) return;
            if (button.dataset.archiveAction === 'board') window.openProjectBoard(project.id, project.name, project.note_text || '');
            if (button.dataset.archiveAction === 'report') window.viewArchivedReport(project.id);
            if (button.dataset.archiveAction === 'reactivate') window.reactivateArchivedProject(project.id, project.name);
            if (button.dataset.archiveAction === 'delete') window.deleteArchivedProject(project.id, project.name);
        });
    });
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
    window.showConfirmDeleteModal(
        projectID,
        'project',
        `Eliminare definitivamente "${projectName}"?`,
        'Si consiglia di esportare prima un backup del progetto in formato JSON per poterlo eventualmente reimportare in futuro.'
    );
}
