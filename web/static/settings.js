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

// === GESTIONE TIPI DI ATTIVITÀ ===

let activityTypes = [];

// Carica tipi di attività
async function loadActivityTypes() {
    try {
        const response = await fetch('/api/activity-types');
        activityTypes = await response.json();
        displayActivityTypes();
    } catch (error) {
        console.error('Errore caricamento tipi attività:', error);
    }
}

// Visualizza tipi di attività
function displayActivityTypes() {
    const container = document.getElementById('activityTypesList');

    if (!activityTypes || activityTypes.length === 0) {
        container.innerHTML = '<p style="color: #999999;">Nessun tipo di attività configurato</p>';
        return;
    }

    const patternNames = {
        'solid': 'Solido',
        'stripes': 'Strisce oblique',
        'dots': 'Puntini'
    };

    let html = '<div id="activityTypesContainer" style="display: flex; flex-direction: column; gap: 10px;">';

    activityTypes.forEach((type, index) => {
        const colorDesc = type.ColorVariant > 0 ? 'chiaro' :
                         type.ColorVariant < 0 ? 'scuro' : 'normale';
        const patternName = patternNames[type.Pattern] || type.Pattern;

        html += `
            <div class="activity-type-item" draggable="true" data-id="${type.ID}" data-order="${type.DisplayOrder}"
                 style="display: flex; justify-content: space-between; align-items: center; padding: 15px; background: #1a1a1a; border-radius: 16px; border-left: 4px solid #ff6b2b; cursor: move; transition: all 0.2s; border: 1px solid #3a3a3a;">
                <div style="display: flex; align-items: center; gap: 15px; flex: 1;">
                    <div style="font-size: 1.5em; color: #999999; cursor: grab;" class="drag-handle">⠿</div>
                    <div style="flex: 1;">
                        <strong style="font-size: 1.1em; color: #ffffff;">${type.Name}</strong>
                        <p style="color: #999999; margin: 5px 0 0 0; font-size: 0.9em;">
                            Colore: ${colorDesc} (${type.ColorVariant.toFixed(1)}) | Pattern: ${patternName}
                        </p>
                    </div>
                </div>
                <div style="display: flex; gap: 10px;">
                    <button class="btn" style="width: auto; padding: 8px 16px; margin: 0; background: #ffffff; color: #1a1a1a;" onclick="editActivityType(${type.ID})">Modifica</button>
                    <button class="btn" style="width: auto; padding: 8px 16px; margin: 0; background: #dc2626; color: #ffffff;" onclick="deleteActivityType(${type.ID}, '${type.Name.replace(/'/g, "\\'")}')">Elimina</button>
                </div>
            </div>
        `;
    });

    html += '</div>';
    container.innerHTML = html;

    // Inizializza drag & drop
    initializeDragAndDrop();
}

// Apri modale per nuovo tipo
function openActivityTypeModal() {
    document.getElementById('activityTypeModalTitle').textContent = 'Nuovo Tipo di Attività';
    document.getElementById('activityTypeId').value = '';
    document.getElementById('activityTypeName').value = '';
    document.getElementById('activityTypeColor').value = '0';
    document.getElementById('activityTypePattern').value = 'solid';
    document.getElementById('activityTypeOrder').value = activityTypes.length + 1;

    // Reset visual state
    document.getElementById('colorValue').textContent = '0.0';
    document.getElementById('nameError').style.display = 'none';
    document.getElementById('saveActivityBtn').disabled = false;

    // Select default pattern
    selectPattern('solid');

    // Update preview
    updateActivityPreview();

    document.getElementById('activityTypeModal').style.display = 'flex';
}

// Modifica tipo esistente
function editActivityType(id) {
    const type = activityTypes.find(t => t.ID === id);
    if (!type) return;

    document.getElementById('activityTypeModalTitle').textContent = 'Modifica Tipo di Attività';
    document.getElementById('activityTypeId').value = type.ID;
    document.getElementById('activityTypeName').value = type.Name;
    document.getElementById('activityTypeColor').value = type.ColorVariant;
    document.getElementById('activityTypePattern').value = type.Pattern || 'solid';
    document.getElementById('activityTypeOrder').value = type.DisplayOrder;

    // Update visual state
    document.getElementById('colorValue').textContent = type.ColorVariant.toFixed(1);
    document.getElementById('nameError').style.display = 'none';
    document.getElementById('saveActivityBtn').disabled = false;

    // Select pattern
    selectPattern(type.Pattern || 'solid');

    // Update preview
    updateActivityPreview();

    document.getElementById('activityTypeModal').style.display = 'flex';
}

// Chiudi modale
function closeActivityTypeModal() {
    document.getElementById('activityTypeModal').style.display = 'none';
}

// Salva tipo di attività
async function saveActivityType() {
    const id = document.getElementById('activityTypeId').value;
    const name = document.getElementById('activityTypeName').value.trim();
    const colorVariant = parseFloat(document.getElementById('activityTypeColor').value);
    const pattern = document.getElementById('activityTypePattern').value;
    const displayOrder = parseInt(document.getElementById('activityTypeOrder').value);

    if (!name) {
        showNotification('Inserisci un nome', 'error');
        return;
    }

    const data = {
        name: name,
        color_variant: colorVariant,
        pattern: pattern,
        display_order: displayOrder
    };

    try {
        let response;
        if (id) {
            // Modifica
            data.id = parseInt(id);
            response = await fetch('/api/activity-types/update', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
        } else {
            // Nuovo
            response = await fetch('/api/activity-types/create', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
        }

        if (response.ok) {
            showNotification(id ? 'Tipo aggiornato!' : 'Tipo creato!', 'success');
            closeActivityTypeModal();
            loadActivityTypes();
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore salvataggio tipo attività:', error);
        showNotification('Errore salvataggio', 'error');
    }
}

// Elimina tipo di attività
async function deleteActivityType(id, name) {
    if (!confirm(`Vuoi davvero eliminare il tipo "${name}"?\n\nTutte le sessioni con questo tipo verranno impostate su "Nessuna attività".`)) {
        return;
    }

    try {
        const response = await fetch('/api/activity-types/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ id: id })
        });

        if (response.ok) {
            showNotification('Tipo eliminato!', 'success');
            loadActivityTypes();
        } else {
            const error = await response.text();
            showNotification('Errore: ' + error, 'error');
        }
    } catch (error) {
        console.error('Errore eliminazione tipo attività:', error);
        showNotification('Errore eliminazione', 'error');
    }
}

// === FUNZIONI ANTEPRIMA E VALIDAZIONE ===

// Seleziona pattern
function selectPattern(patternName) {
    // Remove selected from all
    document.querySelectorAll('.pattern-option').forEach(opt => {
        opt.classList.remove('selected');
    });

    // Add selected to chosen pattern
    const selected = document.querySelector(`.pattern-option[data-pattern="${patternName}"]`);
    if (selected) {
        selected.classList.add('selected');
    }

    // Update hidden field
    document.getElementById('activityTypePattern').value = patternName;

    // Update preview
    updateActivityPreview();
}

// Aggiorna valore colore visualizzato
function updateColorValue() {
    const value = parseFloat(document.getElementById('activityTypeColor').value);
    document.getElementById('colorValue').textContent = value.toFixed(1);
}

// Valida nome attività
function validateActivityName() {
    const name = document.getElementById('activityTypeName').value.trim();
    const errorDiv = document.getElementById('nameError');
    const saveBtn = document.getElementById('saveActivityBtn');

    if (!name) {
        errorDiv.textContent = 'Il nome è obbligatorio';
        errorDiv.style.display = 'block';
        saveBtn.disabled = true;
        return false;
    }

    // Check for duplicates (excluding current if editing)
    const currentId = document.getElementById('activityTypeId').value;
    const duplicate = activityTypes.find(t =>
        t.Name.toLowerCase() === name.toLowerCase() &&
        t.ID !== parseInt(currentId)
    );

    if (duplicate) {
        errorDiv.textContent = 'Esiste già un tipo con questo nome';
        errorDiv.style.display = 'block';
        saveBtn.disabled = true;
        return false;
    }

    errorDiv.style.display = 'none';
    saveBtn.disabled = false;
    return true;
}

// Aggiorna anteprima live
function updateActivityPreview() {
    const nameInput = document.getElementById('activityTypeName');
    const colorInput = document.getElementById('activityTypeColor');
    const patternInput = document.getElementById('activityTypePattern');
    const previewElement = document.getElementById('activityPreview');
    const previewName = document.getElementById('previewName');

    // Check if elements exist
    if (!nameInput || !colorInput || !patternInput || !previewElement || !previewName) {
        console.error('Preview elements not found');
        return;
    }

    const name = nameInput.value.trim() || 'Nome Attività';
    const colorVariant = parseFloat(colorInput.value);
    const pattern = patternInput.value;

    // Update preview name
    previewName.textContent = name;

    // Calculate color (bianco e nero come nella timeline reale)
    const baseColor = { r: 255, g: 255, b: 255 }; // bianco
    let r = baseColor.r;
    let g = baseColor.g;
    let b = baseColor.b;

    if (colorVariant > 0) {
        // Lighter (resta bianco)
        // già bianco, non serve modificare
    } else if (colorVariant < 0) {
        // Darker (verso il nero)
        r += r * colorVariant;
        g += g * colorVariant;
        b += b * colorVariant;
    }

    const color = `rgb(${Math.round(r)}, ${Math.round(g)}, ${Math.round(b)})`;
    const lightColor = `rgb(${Math.min(255, Math.round(r + 40))}, ${Math.min(255, Math.round(g + 40))}, ${Math.min(255, Math.round(b + 40))})`;

    // Apply pattern to preview
    previewElement.style.backgroundColor = color;

    // Reset all background properties first
    previewElement.style.backgroundImage = 'none';
    previewElement.style.backgroundSize = 'auto';

    switch (pattern) {
        case 'solid':
            // Already reset above
            break;
        case 'stripes':
            previewElement.style.backgroundImage = `repeating-linear-gradient(45deg, ${color}, ${color} 5px, ${lightColor} 5px, ${lightColor} 10px)`;
            break;
        case 'dots':
            previewElement.style.backgroundImage = `radial-gradient(circle, ${lightColor} 2px, transparent 2px)`;
            previewElement.style.backgroundSize = '10px 10px';
            break;
    }
}

// === GESTIONE TEMPO DI INATTIVITÀ ===

function saveIdleThreshold() {
    const threshold = document.getElementById('idleThresholdSetting').value;
    if (threshold && threshold > 0) {
        localStorage.setItem('idleThreshold', threshold);
        showNotification('Tempo di inattività salvato!', 'success');
    } else {
        showNotification('Inserisci un valore valido', 'error');
    }
}

function loadIdleThreshold() {
    const saved = localStorage.getItem('idleThreshold');
    if (saved) {
        document.getElementById('idleThresholdSetting').value = saved;
    }
}

// Carica tipi al caricamento pagina
document.addEventListener('DOMContentLoaded', () => {
    loadActivityTypes();
    loadIdleThreshold();

    // Aggiungi event listener per l'input del nome
    const nameInput = document.getElementById('activityTypeName');
    if (nameInput) {
        nameInput.addEventListener('input', () => {
            updateActivityPreview();
            validateActivityName();
        });
    }

    // Aggiungi event listener per lo slider del colore
    const colorInput = document.getElementById('activityTypeColor');
    if (colorInput) {
        colorInput.addEventListener('input', () => {
            updateActivityPreview();
            updateColorValue();
        });
    }

    // Aggiungi event listener per i pattern
    document.querySelectorAll('.pattern-option').forEach(option => {
        option.addEventListener('click', () => {
            const pattern = option.getAttribute('data-pattern');
            selectPattern(pattern);
        });
    });
});

// === DRAG AND DROP PER RIORDINO ===

let draggedElement = null;
let draggedOverElement = null;

function initializeDragAndDrop() {
    const items = document.querySelectorAll('.activity-type-item');

    items.forEach(item => {
        item.addEventListener('dragstart', handleDragStart);
        item.addEventListener('dragover', handleDragOver);
        item.addEventListener('dragenter', handleDragEnter);
        item.addEventListener('dragleave', handleDragLeave);
        item.addEventListener('drop', handleDrop);
        item.addEventListener('dragend', handleDragEnd);
    });
}

function handleDragStart(e) {
    draggedElement = this;
    this.style.opacity = '0.5';
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/html', this.innerHTML);
}

function handleDragOver(e) {
    if (e.preventDefault) {
        e.preventDefault();
    }
    e.dataTransfer.dropEffect = 'move';
    return false;
}

function handleDragEnter(e) {
    if (this !== draggedElement) {
        this.style.borderTop = '3px solid #ff6b2b';
    }
}

function handleDragLeave(e) {
    this.style.borderTop = '';
}

function handleDrop(e) {
    if (e.stopPropagation) {
        e.stopPropagation();
    }

    if (draggedElement !== this) {
        // Ottieni gli ID degli elementi
        const draggedId = parseInt(draggedElement.getAttribute('data-id'));
        const droppedOnId = parseInt(this.getAttribute('data-id'));

        // Trova gli indici nell'array
        const draggedIndex = activityTypes.findIndex(t => t.ID === draggedId);
        const droppedIndex = activityTypes.findIndex(t => t.ID === droppedOnId);

        // Riordina l'array
        const temp = activityTypes[draggedIndex];
        activityTypes.splice(draggedIndex, 1);
        activityTypes.splice(droppedIndex, 0, temp);

        // Aggiorna DisplayOrder
        activityTypes.forEach((type, index) => {
            type.DisplayOrder = index + 1;
        });

        // Ridisegna la lista
        displayActivityTypes();

        // Salva il nuovo ordine sul server
        saveActivityTypesOrder();
    }

    return false;
}

function handleDragEnd(e) {
    this.style.opacity = '1';

    // Rimuovi tutti gli stili di drag
    const items = document.querySelectorAll('.activity-type-item');
    items.forEach(item => {
        item.style.borderTop = '';
    });
}

async function saveActivityTypesOrder() {
    try {
        // Prepara l'array con gli ordini aggiornati
        const updates = activityTypes.map(type => ({
            id: type.ID,
            display_order: type.DisplayOrder
        }));

        const response = await fetch('/api/activity-types/reorder', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ types: updates })
        });

        if (response.ok) {
            showNotification('Ordine aggiornato!', 'success');
        } else {
            const error = await response.text();
            showNotification('Errore aggiornamento ordine: ' + error, 'error');
            // Ricarica i dati originali in caso di errore
            loadActivityTypes();
        }
    } catch (error) {
        console.error('Errore salvataggio ordine:', error);
        showNotification('Errore salvataggio ordine', 'error');
        loadActivityTypes();
    }
}

// === EXPORT/IMPORT DATI ===

// Esporta tutti i dati in formato JSON
async function exportData() {
    try {
        const response = await fetch('/api/data/export');
        if (!response.ok) {
            throw new Error('Errore durante l\'esportazione');
        }

        const data = await response.json();

        // Crea il file da scaricare
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);

        // Crea nome file con data
        const now = new Date();
        const dateStr = now.toISOString().split('T')[0]; // YYYY-MM-DD
        const filename = `prenditempo_backup_${dateStr}.json`;

        // Scarica il file
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);

        showNotification('Dati esportati con successo!', 'success');
    } catch (error) {
        console.error('Errore esportazione dati:', error);
        showNotification('Errore durante l\'esportazione dei dati', 'error');
    }
}

// Importa dati da file JSON
async function importData(input) {
    const file = input.files[0];
    if (!file) return;

    // Conferma prima di procedere
    if (!confirm('ATTENZIONE: L\'importazione sostituirà TUTTI i dati esistenti!\n\nVuoi procedere?')) {
        input.value = ''; // Reset input
        return;
    }

    const statusEl = document.getElementById('importStatus');
    statusEl.style.display = 'block';
    statusEl.style.color = '#3b82f6';
    statusEl.textContent = 'Importazione in corso...';

    try {
        const text = await file.text();
        const data = JSON.parse(text);

        // Verifica che il file abbia la struttura corretta
        if (!data.projects || !data.sessions || !data.activity_types) {
            throw new Error('File non valido: struttura dati mancante');
        }

        const response = await fetch('/api/data/import', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });

        if (!response.ok) {
            const error = await response.text();
            throw new Error(error);
        }

        statusEl.style.color = '#10b981';
        statusEl.textContent = `Importazione completata! ${data.projects.length} progetti, ${data.sessions.length} sessioni, ${data.activity_types.length} tipi di attività.`;

        showNotification('Dati importati con successo!', 'success');

        // Ricarica i tipi di attività nella pagina
        loadActivityTypes();

    } catch (error) {
        console.error('Errore importazione dati:', error);
        statusEl.style.color = '#ef4444';
        statusEl.textContent = 'Errore: ' + error.message;
        showNotification('Errore durante l\'importazione', 'error');
    }

    // Reset input per permettere di selezionare lo stesso file di nuovo
    input.value = '';
}

