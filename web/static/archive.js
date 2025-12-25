// === NOTIFICHE ===

function showNotification(message, type = 'success') {
    const notification = document.getElementById('notification');
    notification.textContent = message;
    notification.className = `notification ${type}`;
    notification.style.display = 'block';

    setTimeout(() => {
        notification.style.display = 'none';
    }, 3000);
}

// === CARICAMENTO PROGETTI ARCHIVIATI ===

async function loadArchivedProjects() {
    try {
        const response = await fetch('/api/projects/archived');
        const projects = await response.json();
        displayArchivedProjects(projects);
    } catch (error) {
        console.error('Errore caricamento progetti archiviati:', error);
        showNotification('Errore caricamento progetti archiviati', 'error');
    }
}

function displayArchivedProjects(projects) {
    const container = document.getElementById('archivedProjectsList');

    if (!projects || projects.length === 0) {
        container.innerHTML = '<p style="color: #999999;">Nessun progetto archiviato.</p>';
        return;
    }

    container.innerHTML = projects.map(project => {
        const closedDate = project.ClosedAt ? new Date(project.ClosedAt).toLocaleDateString('it-IT') : 'N/A';
        const createdDate = project.CreatedAt ? new Date(project.CreatedAt).toLocaleDateString('it-IT') : 'N/A';

        return `
            <div class="project-item">
                <div>
                    <h3>${project.Name}</h3>
                    <p>${project.Description || 'Nessuna descrizione'}</p>
                    <p style="color: #999999;"><strong style="color: #ffffff;">Creato:</strong> ${createdDate} | <strong style="color: #ffffff;">Chiuso:</strong> ${closedDate}</p>
                </div>
                <div style="display: flex; gap: 10px;">
                    <button class="btn" style="width: auto; padding: 8px 16px; margin: 0; background: #ffffff; color: #1a1a1a;"
                            onclick="viewReport(${project.ID})">Visualizza Report</button>
                    <button class="btn btn-success" style="width: auto; padding: 8px 16px; margin: 0;"
                            onclick="reactivateProject(${project.ID}, '${project.Name.replace(/'/g, "\\'")}')">Riattiva</button>
                    <button class="btn" style="width: auto; padding: 8px 16px; margin: 0; background: #dc3545; color: white;"
                            onclick="deleteArchivedProject(${project.ID}, '${project.Name.replace(/'/g, "\\'")}')">Elimina</button>
                </div>
            </div>
        `;
    }).join('');
}

// === VISUALIZZA REPORT ===

let currentReport = null; // Salva il report corrente per l'esportazione PDF

async function viewReport(projectID) {
    try {
        const response = await fetch(`/api/projects/report?id=${projectID}`);
        const report = await response.json();
        currentReport = report; // Salva il report
        showReportModal(report);
    } catch (error) {
        console.error('Errore caricamento report:', error);
        showNotification('Errore caricamento report', 'error');
    }
}

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
        <div style="background: #1a1a1a; padding: 20px; border-radius: 12px; margin-bottom: 15px; border: 1px solid #3a3a3a;">
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

// === ELIMINA PROGETTO ARCHIVIATO ===

async function deleteArchivedProject(projectID, projectName) {
    if (!confirm(`Vuoi eliminare definitivamente il progetto "${projectName}"?\n\nQuesta azione è irreversibile e cancellerà tutti i dati associati (sessioni, note, report).`)) {
        return;
    }

    try {
        const response = await fetch('/api/projects/delete-archived', {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: projectID })
        });

        if (response.ok) {
            showNotification('Progetto eliminato definitivamente!', 'success');
            loadArchivedProjects(); // Ricarica la lista
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore eliminazione progetto:', error);
        showNotification('Errore eliminazione progetto', 'error');
    }
}

// === RIATTIVA PROGETTO ===

async function reactivateProject(projectID, projectName) {
    if (!confirm(`Vuoi riattivare il progetto "${projectName}"?\n\nIl progetto tornerà visibile nella dashboard principale.`)) {
        return;
    }

    try {
        const response = await fetch('/api/projects/reactivate', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: projectID })
        });

        if (response.ok) {
            showNotification('Progetto riattivato con successo!', 'success');
            loadArchivedProjects(); // Ricarica la lista
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore riattivazione progetto:', error);
        showNotification('Errore riattivazione progetto', 'error');
    }
}

// === ESPORTAZIONE PDF ===

function exportReportToPDF() {
    if (!currentReport) {
        showNotification('Nessun report da esportare', 'error');
        return;
    }

    const { jsPDF } = window.jspdf;
    const doc = new jsPDF();

    // Formatta le date
    const startDate = currentReport.start_date ? new Date(currentReport.start_date).toLocaleDateString('it-IT') : 'N/A';
    const endDate = currentReport.end_date ? new Date(currentReport.end_date).toLocaleDateString('it-IT') : 'N/A';
    const closedAt = currentReport.closed_at ? new Date(currentReport.closed_at).toLocaleString('it-IT') : 'N/A';

    let yPos = 20;

    // Colori bianco e nero
    const colorNero = [0, 0, 0];
    const colorGrigio = [100, 100, 100];
    const colorGrigioChiaro = [150, 150, 150];

    // Titolo
    doc.setFontSize(20);
    doc.setFont(undefined, 'bold');
    doc.setTextColor(...colorNero);
    doc.text('Report di Chiusura Progetto', 20, yPos);
    yPos += 15;

    // Nome progetto
    doc.setFontSize(16);
    doc.setFont(undefined, 'bold');
    doc.setTextColor(...colorNero);
    doc.text(currentReport.project_name, 20, yPos);
    yPos += 10;

    // Descrizione
    if (currentReport.project_description) {
        doc.setFontSize(11);
        doc.setFont(undefined, 'normal');
        doc.setTextColor(...colorGrigio);
        const descLines = doc.splitTextToSize(currentReport.project_description, 170);
        doc.text(descLines, 20, yPos);
        yPos += (descLines.length * 7) + 10;
    } else {
        yPos += 5;
    }

    // Linea separatrice
    doc.setDrawColor(...colorGrigioChiaro);
    doc.line(20, yPos, 190, yPos);
    yPos += 10;

    // Sezione Periodo
    doc.setFontSize(14);
    doc.setFont(undefined, 'bold');
    doc.setTextColor(...colorNero);
    doc.text('Periodo', 20, yPos);
    yPos += 8;

    doc.setFontSize(11);
    doc.setFont(undefined, 'normal');
    doc.setTextColor(...colorGrigio);
    doc.text(`Data inizio: ${startDate}`, 20, yPos);
    yPos += 7;
    doc.text(`Data fine: ${endDate}`, 20, yPos);
    yPos += 7;
    doc.text(`Chiuso il: ${closedAt}`, 20, yPos);
    yPos += 12;

    // Linea separatrice
    doc.line(20, yPos, 190, yPos);
    yPos += 10;

    // Totale Ore
    doc.setFontSize(14);
    doc.setFont(undefined, 'bold');
    doc.setTextColor(...colorNero);
    doc.text('Totale Ore Tracciate', 20, yPos);
    yPos += 10;

    doc.setFontSize(18);
    doc.setFont(undefined, 'bold');
    doc.setTextColor(...colorNero);
    doc.text(`${currentReport.total_hours.toFixed(2)} ore`, 20, yPos);
    yPos += 15;

    // Linea separatrice
    doc.setDrawColor(...colorGrigioChiaro);
    doc.line(20, yPos, 190, yPos);
    yPos += 10;

    // Suddivisione per Tipo di Attività
    doc.setFontSize(14);
    doc.setFont(undefined, 'bold');
    doc.setTextColor(...colorNero);
    doc.text('Suddivisione per Tipo di Attivit\u00E0', 20, yPos);
    yPos += 10;

    if (currentReport.activity_breakdown && Object.keys(currentReport.activity_breakdown).length > 0) {
        doc.setFontSize(11);
        doc.setFont(undefined, 'normal');

        // Ordina per ore (dal maggiore al minore)
        const sortedActivities = Object.entries(currentReport.activity_breakdown)
            .sort((a, b) => b[1] - a[1]);

        sortedActivities.forEach(([activity, hours]) => {
            const percentage = ((hours / currentReport.total_hours) * 100).toFixed(1);

            // Nome attività
            doc.setTextColor(...colorNero);
            doc.text(activity, 25, yPos);

            // Ore e percentuale
            doc.setTextColor(...colorGrigio);
            doc.text(`${hours.toFixed(2)} ore (${percentage}%)`, 140, yPos, { align: 'right' });

            yPos += 8;

            // Se si supera la pagina, aggiungi una nuova pagina
            if (yPos > 270) {
                doc.addPage();
                yPos = 20;
            }
        });
    } else {
        doc.setTextColor(...colorGrigio);
        doc.text('Nessun dato disponibile', 25, yPos);
    }

    // Footer
    const pageCount = doc.internal.getNumberOfPages();
    for (let i = 1; i <= pageCount; i++) {
        doc.setPage(i);
        doc.setFontSize(9);
        doc.setFont(undefined, 'normal');
        doc.setTextColor(...colorGrigioChiaro);
        doc.text(`Generato il ${new Date().toLocaleString('it-IT')}`, 20, 285);
        doc.text(`Pagina ${i} di ${pageCount}`, 190, 285, { align: 'right' });
    }

    // Salva il PDF
    const fileName = `Report_${currentReport.project_name.replace(/[^a-z0-9]/gi, '_')}_${new Date().toISOString().split('T')[0]}.pdf`;
    doc.save(fileName);

    showNotification('PDF esportato con successo!', 'success');
}

// === INIZIALIZZAZIONE ===

document.addEventListener('DOMContentLoaded', () => {
    loadArchivedProjects();
});
