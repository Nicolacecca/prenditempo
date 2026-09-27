import { SaveReportJSON, SaveReportText } from '../wailsjs/go/main/App.js';
import { showNotification } from '../notifications.js';
import { escapeHtml } from '../utils.js';

let currentReportProjectId = null;

// === REPORT ===

export function showReportModal(report) {
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
                <div style="display: flex; justify-content: space-between; padding: 12px; background: #1C1815; border-radius: 8px; margin-bottom: 8px; border: 1px solid #322B24;">
                    <span style="color: #E4DACB;"><strong>${escapeHtml(activity)}</strong></span>
                    <span style="color: #C99A54; font-weight: 600;">${hoursFormatted} ore</span>
                </div>
            `;
            }).join('');
    }

    content.innerHTML = `
        <div style="background: #1C1815; padding: 20px; border-radius: 8px; margin-bottom: 15px; border: 1px solid #322B24;">
            <h3 style="color: #C99A54; margin-bottom: 10px;">${escapeHtml(report.project_name || 'Progetto')}</h3>
            <p style="color: #9C9086;">${escapeHtml(report.project_description || 'Nessuna descrizione')}</p>
        </div>
        <div style="margin-bottom: 20px;">
            <h3 style="color: #E4DACB; margin-bottom: 10px;">Periodo</h3>
            <p style="color: #9C9086;"><strong style="color: #E4DACB;">Data inizio:</strong> ${escapeHtml(startDate)}</p>
            <p style="color: #9C9086;"><strong style="color: #E4DACB;">Data fine:</strong> ${escapeHtml(endDate)}</p>
            <p style="color: #9C9086;"><strong style="color: #E4DACB;">Chiuso il:</strong> ${escapeHtml(closedAt)}</p>
        </div>
        <div style="margin-bottom: 20px;">
            <h3 style="color: #E4DACB; margin-bottom: 10px;">Totale Ore Tracciate</h3>
            <p style="font-size: 2em; font-weight: bold; color: #C99A54;">${totalHours} ore</p>
        </div>
        <div style="margin-bottom: 20px;">
            <h3 style="color: #E4DACB; margin-bottom: 10px;">Suddivisione per Tipo di Attività</h3>
            ${activityBreakdownHTML || '<p style="color: #9C9086;">Nessun dato disponibile</p>'}
        </div>
        <div style="border-top: 1px solid #322B24; padding-top: 15px; margin-top: 10px;">
            <h3 style="color: #E4DACB; margin-bottom: 10px;">Esporta Report</h3>
            <div style="display: flex; gap: 10px;">
                <button class="btn" style="flex: 1; padding: 10px; margin: 0;" data-report-action="json">
                    Salva JSON (backup)
                </button>
                <button class="btn" style="flex: 1; padding: 10px; margin: 0; background: #10b981;" data-report-action="text">
                    Salva TXT
                </button>
                <button class="btn" style="flex: 1; padding: 10px; margin: 0;" data-report-action="print">
                    Stampa / PDF
                </button>
            </div>
        </div>
    `;

    const reportActions = {
        json: () => window.exportReportJSON(),
        text: () => window.exportReportText(),
        print: () => window.printReport(),
    };
    content.querySelectorAll('[data-report-action]').forEach(button => {
        button.addEventListener('click', () => reportActions[button.dataset.reportAction]?.());
    });

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
                h3 { color: #C99A54; margin-bottom: 10px; }
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
            <h1 style="color: #C99A54;">PrendiTempo - Report Progetto</h1>
            ${reportContent}
            <p style="margin-top: 30px; color: #6B6259; font-size: 0.9em;">
                Generato il ${new Date().toLocaleString('it-IT')}
            </p>
        </body>
        </html>
    `);
    printWindow.document.close();
    printWindow.print();
}
