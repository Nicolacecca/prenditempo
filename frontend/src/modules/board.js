import {
    ActiveSelection,
    Canvas,
    Circle,
    FabricImage,
    Group,
    IText,
    Line,
    Path,
    PencilBrush,
    Point,
    Rect,
    Textbox,
    Triangle,
    util,
} from 'fabric';
import {
    CleanupBoardAssets,
    GetBoardAsset,
    GetProjectBoard,
    ImportBoardImage,
    OpenBoardLink,
    OpenBoardVideo,
    SaveBoardPDF,
    SaveBoardPNG,
    SaveProjectBoard,
    SelectBoardFolder,
    SelectBoardVideo,
    ValidateBoardLink,
} from '../wailsjs/go/main/App.js';
import { showNotification } from '../notifications.js';
import {
    addTableColumn,
    addTableRow,
    createDefaultTableData,
    normalizeTableData,
    removeTableColumn,
    removeTableRow,
    resizeTableData,
    TABLE_MAX_COLUMNS,
    TABLE_MAX_CELL_LENGTH,
    TABLE_MAX_ROWS,
} from './board-table-data.js';
import { BOARD_ANCHORS, anchorPointsFromCorners, closestAnchorPair, nearestAnchor } from './board-connectors.js';
import { listPrefixEdits } from './board-text-lists.js';
import { restoreBoardObjectMetadata } from './board-persistence.js';
import {
    normalizeTableCellStyle,
    patchSelectedTableCellStyles,
    resizeTableCellStyles,
} from './board-table-styles.js';

const BOARD_SCHEMA_VERSION = 1;
const AUTOSAVE_DELAY_MS = 900;
const HISTORY_LIMIT = 60;
const MISSING_IMAGE_DATA_URL = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=';
const CUSTOM_OBJECT_PROPERTIES = [
    'boardKind',
    'linkType',
    'linkTarget',
    'linkLabel',
    'assetId',
    'assetName',
    'tableData',
    'tableCellStyles',
    'boardObjectId',
    'connectorFrom',
    'connectorTo',
    'connectorFromAnchor',
    'connectorToAnchor',
    'connectorLabel',
    'connectorLabelStyle',
    'videoPath',
    'videoName',
    'videoSize',
    'boardLocked',
    'boardName',
    'noteWidth',
    'noteHeight',
    'shapeLabelBaseWidth',
];

let boardCanvas = null;
let currentProject = null;
let saveTimer = null;
let spacePressed = false;
let isPanning = false;
let lastPointer = null;
let restoringHistory = false;
let undoStack = [];
let redoStack = [];
let pendingLinkType = null;
let editingTable = null;
let tableDraft = [];
let tableDraftStyles = [];
let tableSelectedCells = new Set();
let tableSelectionAnchor = null;
let boardClipboard = [];
let drawingEnabled = false;
let drawingPausedForPan = false;
let drawingEraserEnabled = false;
let snappingEnabled = false;
let minimapVisible = true;
let boardTrash = [];
let minimapMapping = null;
let connectorHelpers = [];
let draggingConnectorHandle = null;
let pendingConnectorSelectionID = null;
let activeColorTarget = null;
let boardLoadMigrationNeeded = false;
let connectorCreationMode = false;
let connectorCreationFrom = null;
let inlineTextEditor = null;

const BOARD_COLOR_CONTROLS = Object.freeze({
    text: { inputID: 'boardTextColorPicker', label: 'Colore del testo' },
    line: { inputID: 'boardLineColorPicker', label: 'Colore di linee e contorni' },
    fill: { inputID: 'boardFillColorPicker', label: 'Colore dello sfondo' },
});

function setSaveStatus(text, state = '') {
    const status = document.getElementById('boardSaveStatus');
    if (!status) return;
    status.textContent = text;
    status.dataset.state = state;
}

function canvasDocument() {
    const document = boardCanvas.toJSON(CUSTOM_OBJECT_PROPERTIES);
    document.objects = (document.objects || []).filter(object => !isConnectorHelper(object));
    walkBoardObjects(document.objects, object => {
        if (object.boardKind === 'image') {
            delete object.src;
            delete object.crossOrigin;
        }
    });
    document.prendiTempo = { trash: boardTrash };
    return JSON.stringify(document);
}

function reviveBoardObject(serialized, instance) {
    const result = restoreBoardObjectMetadata(serialized, instance, CUSTOM_OBJECT_PROPERTIES);
    if (result.migrated) boardLoadMigrationNeeded = true;
    return instance;
}

function walkBoardObjects(objects, callback) {
    if (!Array.isArray(objects)) return;
    objects.forEach(object => {
        callback(object);
        walkBoardObjects(object.objects, callback);
    });
}

async function hydrateBoardDocument(document, notifyMissing = false) {
    const hydrated = JSON.parse(JSON.stringify(document));
    let missingImages = 0;
    async function hydrateObjects(objects) {
        if (!Array.isArray(objects)) return;
        await Promise.all(objects.map(async object => {
            await hydrateObjects(object.objects);
            if (object.boardKind !== 'image') return;
            delete object.src;
            delete object.crossOrigin;
            if (!object.assetId || currentProject?.preview) return;
            try {
                const asset = await GetBoardAsset(currentProject.id, object.assetId);
                object.src = asset.data_url;
            } catch {
                missingImages += 1;
                object.src = MISSING_IMAGE_DATA_URL;
            }
        }));
    }
    await hydrateObjects(hydrated.objects);
    if (notifyMissing && missingImages > 0) {
        showNotification(`${missingImages} immagini non sono più disponibili; la lavagna è stata comunque aperta`, 'error');
    }
    return hydrated;
}

function referencedBoardAssetIDs() {
    const identifiers = new Set();
    function visit(objects) {
        objects.forEach(object => {
            if (object.boardKind === 'image' && object.assetId) identifiers.add(object.assetId);
            if (typeof object.getObjects === 'function') visit(object.getObjects());
        });
    }
    visit(boardCanvas?.getObjects() || []);
    walkBoardObjects(boardTrash.map(entry => entry.object), object => {
        if (object.boardKind === 'image' && object.assetId) identifiers.add(object.assetId);
    });
    return [...identifiers];
}

function viewportDocument() {
    return JSON.stringify({ viewportTransform: boardCanvas.viewportTransform });
}

function scheduleSave() {
    if (!boardCanvas || !currentProject || restoringHistory) return;
    clearTimeout(saveTimer);
    setSaveStatus('Modifiche non salvate', 'dirty');
    saveTimer = setTimeout(() => saveBoard(), AUTOSAVE_DELAY_MS);
}

// scheduleSave (debounce), Ctrl+S, undo/redo e la chiusura della lavagna possono
// tutti chiamare saveBoard() quasi in contemporanea. Senza serializzazione due
// richieste in volo potrebbero completarsi fuori ordine e far vincere la
// risposta di rete più lenta invece dell'ultima modifica reale. saveChain
// mette in coda le chiamate: ognuna aspetta che la precedente sia finita prima
// di leggere canvasDocument(), così l'ultima a partire manda sempre lo stato
// più recente. Il fallimento di una chiamata non blocca le successive.
let saveChain = Promise.resolve();

function saveBoard() {
    const run = saveChain.catch(() => {}).then(() => saveBoardNow());
    saveChain = run.catch(() => {});
    return run;
}

async function saveBoardNow() {
    if (!boardCanvas || !currentProject) return;
    clearTimeout(saveTimer);
    if (currentProject.preview) {
        setSaveStatus('Anteprima locale', 'saved');
        return;
    }
    setSaveStatus('Salvataggio…', 'saving');
    try {
        await SaveProjectBoard({
            project_id: currentProject.id,
            schema_version: BOARD_SCHEMA_VERSION,
            document_json: canvasDocument(),
            viewport_json: viewportDocument(),
            legacy_note_imported: true,
        });
        setSaveStatus('Salvato', 'saved');
    } catch (error) {
        console.error('Errore salvataggio lavagna:', error);
        setSaveStatus('Errore di salvataggio', 'error');
        showNotification(`Errore salvataggio lavagna: ${error}`, 'error');
        throw error;
    }
}

function captureHistory() {
    if (!boardCanvas || restoringHistory) return;
    const snapshot = canvasDocument();
    if (undoStack.at(-1) !== snapshot) {
        undoStack.push(snapshot);
        if (undoStack.length > HISTORY_LIMIT) undoStack.shift();
        redoStack = [];
    }
    updateRibbonState();
    scheduleSave();
}

async function restoreSnapshot(snapshot) {
    if (!boardCanvas || !snapshot) return;
    restoringHistory = true;
    try {
        const parsed = JSON.parse(snapshot);
        boardTrash = Array.isArray(parsed.prendiTempo?.trash) ? parsed.prendiTempo.trash.slice(-100) : [];
        const document = await hydrateBoardDocument(parsed);
        boardLoadMigrationNeeded = false;
        await boardCanvas.loadFromJSON(document, reviveBoardObject);
        stabilizeAllStickyNotes();
        stabilizeAllTextShapes();
        connectorHelpers = [];
        refreshConnectors();
        boardCanvas.requestRenderAll();
    } finally {
        restoringHistory = false;
    }
    scheduleSave();
}

async function undo() {
    if (undoStack.length < 2) return;
    const current = undoStack.pop();
    redoStack.push(current);
    await restoreSnapshot(undoStack.at(-1));
}

async function redo() {
    const snapshot = redoStack.pop();
    if (!snapshot) return;
    undoStack.push(snapshot);
    await restoreSnapshot(snapshot);
}

function viewportCenter() {
    const center = boardCanvas.getVpCenter();
    const offset = (boardCanvas.getObjects().filter(object => !isConnectorHelper(object)).length % 6) * 22;
    return new Point(center.x + offset, center.y + offset);
}

function addAndSelect(object) {
    boardCanvas.add(object);
    boardCanvas.setActiveObject(object);
    boardCanvas.requestRenderAll();
}

function addText() {
    const center = viewportCenter();
    addAndSelect(new IText('Testo', {
        left: center.x,
        top: center.y,
        fill: '#f8fafc',
        fontFamily: 'Arial',
        fontSize: 26,
        originX: 'center',
        originY: 'center',
        boardKind: 'text',
    }));
}

function addStickyNote() {
    const center = viewportCenter();
    const note = new Textbox('Nuova nota', {
        left: center.x,
        top: center.y,
        width: 240,
        height: 130,
        minWidth: 140,
        padding: 18,
        fill: '#1f2937',
        backgroundColor: '#fde68a',
        fontFamily: 'Arial',
        fontSize: 22,
        originX: 'center',
        originY: 'center',
        boardKind: 'note',
        noteWidth: 240,
        noteHeight: 130,
    });
    note.set({ height: 130 });
    addAndSelect(note);
}

function stabilizeStickyNoteDimensions(note) {
    if (note?.boardKind !== 'note') return false;
    const width = Math.max(Number(note.minWidth) || 140, Number(note.noteWidth) || Number(note.width) || 240);
    const requestedHeight = Math.max(90, Number(note.noteHeight) || Number(note.height) || 130);
    note.set({ width, scaleX: 1, scaleY: 1 });
    if (typeof note.initDimensions === 'function') note.initDimensions();
    const height = Math.max(requestedHeight, Number(note.height) || 0);
    note.set({ height, noteWidth: width, noteHeight: height });
    note.setCoords();
    return true;
}

function normalizeStickyNoteScale(note) {
    if (note?.boardKind !== 'note') return false;
    const scaleX = Math.abs(Number(note.scaleX) || 1);
    const scaleY = Math.abs(Number(note.scaleY) || 1);
    if (Math.abs(scaleX - 1) < 0.001 && Math.abs(scaleY - 1) < 0.001) return false;

    const center = note.getCenterPoint();
    const requestedWidth = Math.max(Number(note.minWidth) || 140, (Number(note.noteWidth) || Number(note.width)) * scaleX);
    const requestedHeight = Math.max(90, (Number(note.noteHeight) || Number(note.height)) * scaleY);
    note.set({ width: requestedWidth, scaleX: 1, scaleY: 1 });
    if (typeof note.initDimensions === 'function') note.initDimensions();
    const height = Math.max(requestedHeight, Number(note.height) || 0);
    note.set({ height, noteWidth: requestedWidth, noteHeight: height });
    note.setPositionByOrigin(center, 'center', 'center');
    note.setCoords();
    return true;
}

function stabilizeAllStickyNotes() {
    boardCanvas?.getObjects().forEach(object => stabilizeStickyNoteDimensions(object));
}

function createTextShape(background, boardKind, options = {}) {
    background.set({
        left: 0,
        top: 0,
        originX: 'center',
        originY: 'center',
        selectable: false,
        evented: false,
        boardKind: 'shape-background',
    });
    const labelWidth = Math.max(80, (Number(background.width) || 160) * 0.76);
    const label = new Textbox(options.shapeText || '', {
        left: 0,
        top: 0,
        width: labelWidth,
        fill: '#f8fafc',
        fontFamily: 'Arial',
        fontSize: 20,
        textAlign: 'center',
        originX: 'center',
        originY: 'center',
        selectable: false,
        evented: false,
        boardKind: 'shape-label',
    });
    return new Group([background, label], {
        originX: 'center',
        originY: 'center',
        subTargetCheck: false,
        interactive: false,
        boardKind,
        shapeLabelBaseWidth: labelWidth,
        ...options,
    });
}

function shapeLabel(object) {
    if (typeof object?.getObjects !== 'function') return null;
    return object.getObjects().find(child => child.boardKind === 'shape-label') || null;
}

function isTextShape(object) {
    return ['rectangle', 'circle', 'shape'].includes(object?.boardKind) && objectType(object) === 'group' && Boolean(shapeLabel(object));
}

function stabilizeTextShapeLabel(object) {
    if (!isTextShape(object)) return false;
    const label = shapeLabel(object);
    const scaleX = Math.max(0.05, Math.abs(Number(object.scaleX) || 1));
    const scaleY = Math.max(0.05, Math.abs(Number(object.scaleY) || 1));
    const baseWidth = Number(object.shapeLabelBaseWidth) || Number(label.width) || 120;
    label.set({
        width: baseWidth * scaleX,
        scaleX: 1 / scaleX,
        scaleY: 1 / scaleY,
    });
    label.initDimensions?.();
    object.set('shapeLabelBaseWidth', baseWidth);
    return true;
}

function stabilizeAllTextShapes() {
    boardCanvas?.getObjects().forEach(object => stabilizeTextShapeLabel(object));
}

// Sostituisce oldObject con newObject sullo stesso canvas mantenendo la sua
// posizione nello z-order. La mutazione è marcata restoringHistory così
// l'undo non la registra come azione a sé: è il chiamante a decidere se/quando
// chiamare captureHistory() per il risultato finale. Pattern che ricorreva
// identico in 4 punti (rigenerare un testo, un connettore, una tabella):
// centralizzato qui invece di riscriverlo ogni volta con piccole differenze.
function replaceObjectInPlace(oldObject, newObject, { activate = false, bringToFront = false } = {}) {
    const index = boardCanvas.getObjects().indexOf(oldObject);
    const previousRestoring = restoringHistory;
    restoringHistory = true;
    try {
        boardCanvas.remove(oldObject);
        boardCanvas.insertAt(Math.max(index, 0), newObject);
        if (bringToFront) boardCanvas.bringObjectToFront(newObject);
        if (activate) boardCanvas.setActiveObject(newObject);
    } finally {
        restoringHistory = previousRestoring;
    }
    return newObject;
}

function ensureShapeTextContainer(object) {
    if (!object || !['rectangle', 'circle', 'shape'].includes(object.boardKind)) return null;
    if (isTextShape(object)) return object;
    if (!['rect', 'circle', 'triangle'].includes(objectType(object))) return null;
    const center = object.getCenterPoint();
    const saved = {
        angle: object.angle,
        scaleX: object.scaleX,
        scaleY: object.scaleY,
        flipX: object.flipX,
        flipY: object.flipY,
        boardObjectId: object.boardObjectId,
        boardName: object.boardName,
        boardLocked: object.boardLocked,
    };
    object.set({ angle: 0, scaleX: 1, scaleY: 1 });
    const replacement = createTextShape(object, object.boardKind, {
        left: center.x,
        top: center.y,
        ...saved,
    });
    replaceObjectInPlace(object, replacement, { activate: true });
    refreshConnectors();
    boardCanvas.requestRenderAll();
    return replacement;
}

function addRectangle() {
    const center = viewportCenter();
    addAndSelect(createTextShape(new Rect({
        width: 220,
        height: 130,
        fill: 'rgba(201, 154, 84, 0.12)',
        stroke: '#C99A54',
        strokeWidth: 3,
        rx: 10,
        ry: 10,
    }), 'rectangle', { left: center.x, top: center.y }));
}

function addCircle() {
    const center = viewportCenter();
    addAndSelect(createTextShape(new Circle({
        radius: 70,
        fill: 'rgba(108, 137, 168, 0.12)',
        stroke: '#6C89A8',
        strokeWidth: 3,
    }), 'circle', { left: center.x, top: center.y }));
}

function addFrame() {
    const center = viewportCenter();
    const border = new Rect({
        left: 0,
        top: 0,
        width: 560,
        height: 340,
        fill: 'rgba(108, 137, 168, 0.035)',
        stroke: '#443A2E',
        strokeWidth: 3,
        strokeDashArray: [12, 8],
        rx: 14,
        ry: 14,
        originX: 'left',
        originY: 'top',
        selectable: false,
        evented: false,
    });
    const title = new Textbox('Nuova area', {
        left: 18,
        top: 14,
        width: 300,
        fill: '#E4DACB',
        fontFamily: 'Arial',
        fontSize: 20,
        fontWeight: 'bold',
        originX: 'left',
        originY: 'top',
    });
    const frame = new Group([border, title], {
        left: center.x,
        top: center.y,
        originX: 'center',
        originY: 'center',
        boardKind: 'frame',
        subTargetCheck: true,
        interactive: true,
    });
    addAndSelect(frame);
    boardCanvas.sendObjectToBack(frame);
}

function setDrawingMode(enabled) {
    drawingEnabled = Boolean(enabled);
    if (drawingEnabled) drawingEraserEnabled = false;
    if (boardCanvas) {
        if (drawingEnabled && !boardCanvas.freeDrawingBrush) {
            boardCanvas.freeDrawingBrush = new PencilBrush(boardCanvas);
        }
        if (drawingEnabled) {
            boardCanvas.freeDrawingBrush.color = document.getElementById('boardDrawColor')?.value || '#C99A54';
            boardCanvas.freeDrawingBrush.width = Number(document.getElementById('boardDrawWidth')?.value) || 3;
        }
        boardCanvas.isDrawingMode = drawingEnabled;
        boardCanvas.selection = !drawingEnabled && !drawingEraserEnabled;
        boardCanvas.defaultCursor = drawingEnabled || drawingEraserEnabled ? 'crosshair' : 'default';
    }
    document.querySelectorAll('[data-board-command="draw"]').forEach(button => button.classList.toggle('active', drawingEnabled));
    document.querySelectorAll('[data-board-command="erase-drawing"]').forEach(button => button.classList.toggle('active', drawingEraserEnabled));
    updateInteractionHelp();
}

function setDrawingEraserMode(enabled) {
    drawingEraserEnabled = Boolean(enabled);
    if (drawingEraserEnabled) drawingEnabled = false;
    if (boardCanvas) {
        boardCanvas.isDrawingMode = false;
        boardCanvas.selection = !drawingEraserEnabled;
        boardCanvas.defaultCursor = drawingEraserEnabled ? 'crosshair' : 'default';
    }
    document.querySelectorAll('[data-board-command="draw"]').forEach(button => button.classList.toggle('active', drawingEnabled));
    document.querySelectorAll('[data-board-command="erase-drawing"]').forEach(button => button.classList.toggle('active', drawingEraserEnabled));
    updateInteractionHelp();
}

function toggleDrawingEraserMode() {
    setDrawingEraserMode(!drawingEraserEnabled);
}

function toggleDrawingMode() {
    setDrawingMode(!drawingEnabled);
    if (drawingEnabled) boardCanvas.discardActiveObject();
    boardCanvas.requestRenderAll();
}

function addArrow() {
    const center = viewportCenter();
    const line = new Line([-90, 0, 80, 0], {
        stroke: '#f8fafc',
        strokeWidth: 4,
        originX: 'center',
        originY: 'center',
    });
    const head = new Triangle({
        left: 90,
        top: 0,
        width: 20,
        height: 24,
        fill: '#f8fafc',
        angle: 90,
        originX: 'center',
        originY: 'center',
    });
    addAndSelect(new Group([line, head], {
        left: center.x,
        top: center.y,
        originX: 'center',
        originY: 'center',
        boardKind: 'arrow',
    }));
}

function addTriangle() {
    const center = viewportCenter();
    addAndSelect(createTextShape(new Triangle({
        width: 150,
        height: 130,
        fill: 'rgba(255, 107, 43, 0.14)',
        stroke: '#ff6b2b',
        strokeWidth: 3,
    }), 'shape', { left: center.x, top: center.y }));
}

function addLine() {
    const center = viewportCenter();
    addAndSelect(new Line([-90, 0, 90, 0], {
        left: center.x,
        top: center.y,
        stroke: '#f8fafc',
        strokeWidth: 4,
        originX: 'center',
        originY: 'center',
        boardKind: 'shape',
    }));
}

function newBoardObjectId() {
    if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID();
    return `board-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function ensureBoardObjectId(object) {
    if (!object.boardObjectId) object.set('boardObjectId', newBoardObjectId());
    return object.boardObjectId;
}

function isConnectorHelper(object) {
    return object?.boardKind === 'connector-handle' || object?.boardKind === 'anchor-marker';
}

function objectAnchorPoints(object) {
    return anchorPointsFromCorners(object.getCoords());
}

function anchorPoint(object, anchorID) {
    const anchors = objectAnchorPoints(object);
    return anchors.find(anchor => anchor.id === anchorID) || anchors[0];
}

function connectorPathData(start, end) {
    const angle = Math.atan2(end.y - start.y, end.x - start.x);
    const headLength = 18;
    const wingAngle = Math.PI / 7;
    const firstWing = {
        x: end.x - headLength * Math.cos(angle - wingAngle),
        y: end.y - headLength * Math.sin(angle - wingAngle),
    };
    const secondWing = {
        x: end.x - headLength * Math.cos(angle + wingAngle),
        y: end.y - headLength * Math.sin(angle + wingAngle),
    };
    return `M ${start.x} ${start.y} L ${end.x} ${end.y} M ${firstWing.x} ${firstWing.y} L ${end.x} ${end.y} L ${secondWing.x} ${secondWing.y}`;
}

function createConnector(from, to, options = {}) {
    let fromAnchor = options.connectorFromAnchor;
    let toAnchor = options.connectorToAnchor;
    if (!BOARD_ANCHORS.includes(fromAnchor) || !BOARD_ANCHORS.includes(toAnchor)) {
        const closest = closestAnchorPair(objectAnchorPoints(from), objectAnchorPoints(to));
        fromAnchor = closest.from.id;
        toAnchor = closest.to.id;
    }
    const start = anchorPoint(from, fromAnchor);
    const end = anchorPoint(to, toAnchor);
    const savedLine = typeof options.getObjects === 'function'
        ? options.getObjects().find(child => child.boardKind === 'connector-line')
        : null;
    const savedLabel = typeof options.getObjects === 'function'
        ? options.getObjects().find(child => child.boardKind === 'connector-label')
        : null;
    const labelStyle = options.connectorLabelStyle || (savedLabel ? {
        fill: savedLabel.fill,
        fontFamily: savedLabel.fontFamily,
        fontSize: savedLabel.fontSize,
        fontWeight: savedLabel.fontWeight,
        fontStyle: savedLabel.fontStyle,
        underline: savedLabel.underline,
        backgroundColor: savedLabel.backgroundColor,
    } : {});
    const line = new Path(connectorPathData(start, end), {
        fill: '',
        stroke: savedLine?.stroke || options.stroke || '#f8fafc',
        strokeWidth: savedLine?.strokeWidth || options.strokeWidth || 3,
        strokeLineCap: 'round',
        strokeLineJoin: 'round',
        selectable: false,
        evented: false,
        boardKind: 'connector-line',
    });
    const label = new Textbox(String(options.connectorLabel ?? savedLabel?.text ?? ''), {
        left: (start.x + end.x) / 2,
        top: (start.y + end.y) / 2,
        width: Math.min(220, Math.max(100, Math.hypot(end.x - start.x, end.y - start.y) * 0.55)),
        fill: labelStyle.fill || '#f8fafc',
        fontFamily: labelStyle.fontFamily || 'Arial',
        fontSize: Number(labelStyle.fontSize) || 17,
        fontWeight: labelStyle.fontWeight || 'normal',
        fontStyle: labelStyle.fontStyle || 'normal',
        underline: Boolean(labelStyle.underline),
        textAlign: 'center',
        backgroundColor: labelStyle.backgroundColor || 'rgba(36, 36, 36, 0.88)',
        padding: 4,
        originX: 'center',
        originY: 'center',
        selectable: false,
        evented: false,
        boardKind: 'connector-label',
    });
    return new Group([line, label], {
        hasControls: false,
        lockMovementX: true,
        lockMovementY: true,
        hoverCursor: 'default',
        boardKind: 'connector',
        connectorFrom: ensureBoardObjectId(from),
        connectorTo: ensureBoardObjectId(to),
        connectorFromAnchor: fromAnchor,
        connectorToAnchor: toAnchor,
        connectorLabel: String(options.connectorLabel ?? savedLabel?.text ?? ''),
        connectorLabelStyle: labelStyle,
        boardObjectId: options.boardObjectId || newBoardObjectId(),
    });
}

function setConnectorCreationMode(enabled, from = null) {
    connectorCreationMode = Boolean(enabled);
    connectorCreationFrom = connectorCreationMode ? from : null;
    document.querySelectorAll('[data-board-command="connect"]').forEach(button => {
        button.classList.toggle('active', connectorCreationMode);
        button.setAttribute('aria-pressed', String(connectorCreationMode));
    });
    updateInteractionHelp();
}

function addConnectorBetween(from, to) {
    if (!from || !to || from === to) return false;
    const duplicate = boardCanvas.getObjects().some(object => (
        object.boardKind === 'connector'
        && (
            (object.connectorFrom === ensureBoardObjectId(from) && object.connectorTo === ensureBoardObjectId(to))
            || (object.connectorFrom === ensureBoardObjectId(to) && object.connectorTo === ensureBoardObjectId(from))
        )
    ));
    if (duplicate) {
        showNotification('Questi elementi sono già collegati', 'info');
        setConnectorCreationMode(false);
        return false;
    }
    boardCanvas.discardActiveObject();
    const connector = createConnector(from, to);
    boardCanvas.add(connector);
    boardCanvas.bringObjectToFront(connector);
    boardCanvas.setActiveObject(connector);
    boardCanvas.requestRenderAll();
    setConnectorCreationMode(false);
    showNotification('Collegamento creato. Trascina le estremità per cambiare punto di aggancio.', 'success');
    return true;
}

function chooseConnectorTarget(target) {
    if (!connectorCreationMode || !target || target.boardKind === 'connector' || isConnectorHelper(target)) return false;
    if (!connectorCreationFrom) {
        connectorCreationFrom = target;
        boardCanvas.setActiveObject(target);
        boardCanvas.requestRenderAll();
        showNotification('Primo oggetto selezionato. Ora scegli l’oggetto di destinazione.', 'info');
        updateInteractionHelp();
        return true;
    }
    if (target === connectorCreationFrom) {
        showNotification('Scegli un secondo oggetto diverso dal primo.', 'info');
        return true;
    }
    const from = connectorCreationFrom;
    window.setTimeout(() => addConnectorBetween(from, target), 0);
    return true;
}

function addConnector() {
    const selected = boardCanvas.getActiveObjects().filter(object => object.boardKind !== 'connector' && !isConnectorHelper(object));
    if (selected.length === 2) {
        addConnectorBetween(selected[0], selected[1]);
        return;
    }
    if (connectorCreationMode) {
        setConnectorCreationMode(false);
        showNotification('Creazione del collegamento annullata', 'info');
        return;
    }
    const from = selected.length === 1 ? selected[0] : null;
    setConnectorCreationMode(true, from);
    showNotification(from
        ? 'Scegli l’oggetto di destinazione.'
        : 'Scegli il primo oggetto da collegare.', 'info');
}

function refreshConnectors() {
    if (!boardCanvas) return;
    const objects = boardCanvas.getObjects();
    const byId = new Map(objects.filter(object => object.boardObjectId).map(object => [object.boardObjectId, object]));
    const connectors = objects.filter(object => object.boardKind === 'connector');
    const active = boardCanvas.getActiveObject();
    const activeConnectorID = active?.boardKind === 'connector' ? active.boardObjectId : null;
    connectors.forEach(connector => {
        const from = byId.get(connector.connectorFrom);
        const to = byId.get(connector.connectorTo);
        if (!from || !to) {
            const previousRestoring = restoringHistory;
            restoringHistory = true;
            try {
                boardCanvas.remove(connector);
            } finally {
                restoringHistory = previousRestoring;
            }
            return;
        }
        const updated = createConnector(from, to, connector);
        replaceObjectInPlace(connector, updated, {
            bringToFront: true,
            activate: activeConnectorID === updated.boardObjectId,
        });
    });
    boardCanvas.requestRenderAll();
    if (activeConnectorID) {
        const updated = boardCanvas.getObjects().find(object => object.boardObjectId === activeConnectorID);
        if (updated) showConnectorHelpers(updated);
    }
}

function removeConnectorHelpers() {
    if (!boardCanvas || !connectorHelpers.length) return;
    const previousRestoring = restoringHistory;
    restoringHistory = true;
    try {
        connectorHelpers.forEach(helper => boardCanvas.remove(helper));
        connectorHelpers = [];
    } finally {
        restoringHistory = previousRestoring;
    }
}

function showConnectorHelpers(connector) {
    if (!boardCanvas || connector?.boardKind !== 'connector') return;
    removeConnectorHelpers();
    const byId = new Map(boardCanvas.getObjects().filter(object => object.boardObjectId).map(object => [object.boardObjectId, object]));
    const ends = [
        { end: 'from', object: byId.get(connector.connectorFrom), activeAnchor: connector.connectorFromAnchor, color: '#6C89A8' },
        { end: 'to', object: byId.get(connector.connectorTo), activeAnchor: connector.connectorToAnchor, color: '#C99A54' },
    ];
    const previousRestoring = restoringHistory;
    restoringHistory = true;
    try {
        ends.forEach(({ end, object, activeAnchor, color }) => {
            if (!object) return;
            objectAnchorPoints(object).forEach(anchor => {
                const marker = new Circle({
                    left: anchor.x, top: anchor.y, radius: anchor.id === activeAnchor ? 6 : 4,
                    originX: 'center', originY: 'center', fill: anchor.id === activeAnchor ? color : '#1C1815',
                    stroke: color, strokeWidth: 2, selectable: false, evented: false,
                    excludeFromExport: true, boardKind: 'anchor-marker', connectorOwnerId: connector.boardObjectId,
                    connectorEnd: end, connectorAnchor: anchor.id,
                });
                connectorHelpers.push(marker);
                boardCanvas.add(marker);
            });
            const selectedAnchor = anchorPoint(object, activeAnchor);
            const handle = new Circle({
                left: selectedAnchor.x, top: selectedAnchor.y, radius: 9,
                originX: 'center', originY: 'center', fill: color, stroke: '#ffffff', strokeWidth: 2,
                hasControls: false, hasBorders: false, hoverCursor: 'move',
                excludeFromExport: true, boardKind: 'connector-handle', connectorOwnerId: connector.boardObjectId,
                connectorEnd: end,
            });
            connectorHelpers.push(handle);
            boardCanvas.add(handle);
        });
    } finally {
        restoringHistory = previousRestoring;
    }
    boardCanvas.requestRenderAll();
}

function moveConnectorHandle(handle) {
    pendingConnectorSelectionID = handle.connectorOwnerId;
    const connector = boardCanvas.getObjects().find(object => object.boardKind === 'connector' && object.boardObjectId === handle.connectorOwnerId);
    if (!connector) return;
    const targetID = handle.connectorEnd === 'from' ? connector.connectorFrom : connector.connectorTo;
    const target = boardCanvas.getObjects().find(object => object.boardObjectId === targetID);
    if (!target) return;
    const nearest = nearestAnchor(objectAnchorPoints(target), handle.getCenterPoint());
    if (!nearest) return;
    const property = handle.connectorEnd === 'from' ? 'connectorFromAnchor' : 'connectorToAnchor';
    connector.set(property, nearest.id);
    const from = boardCanvas.getObjects().find(object => object.boardObjectId === connector.connectorFrom);
    const to = boardCanvas.getObjects().find(object => object.boardObjectId === connector.connectorTo);
    if (!from || !to) return;
    const updated = createConnector(from, to, connector);
    replaceObjectInPlace(connector, updated, { bringToFront: true });
    handle.set({ left: nearest.x, top: nearest.y }).setCoords();
    connectorHelpers.filter(helper => helper.boardKind === 'anchor-marker' && helper.connectorEnd === handle.connectorEnd).forEach(marker => {
        const selected = marker.connectorAnchor === nearest.id;
        marker.set({
            radius: selected ? 6 : 4,
            fill: selected ? handle.fill : '#1C1815',
        });
    });
    boardCanvas.requestRenderAll();
}

function finishConnectorHandle(handleOrConnectorID) {
    const connectorID = typeof handleOrConnectorID === 'string' ? handleOrConnectorID : handleOrConnectorID?.connectorOwnerId;
    const connector = boardCanvas.getObjects().find(object => object.boardKind === 'connector' && object.boardObjectId === connectorID);
    if (!connector) return;
    boardCanvas.setActiveObject(connector);
    showConnectorHelpers(connector);
    pendingConnectorSelectionID = null;
    captureHistory();
}

function handleConnectorSelection() {
    const active = boardCanvas?.getActiveObject();
    if (pendingConnectorSelectionID && active?.boardKind !== 'connector-handle' && active?.boardObjectId !== pendingConnectorSelectionID) {
        const connectorID = pendingConnectorSelectionID;
        window.setTimeout(() => finishConnectorHandle(connectorID), 35);
        return;
    }
    if (active?.boardKind === 'connector') showConnectorHelpers(active);
    else if (!isConnectorHelper(active)) removeConnectorHelpers();
}

async function copySelection() {
    const selected = boardCanvas.getActiveObjects().filter(object => object.boardKind !== 'connector' && !isConnectorHelper(object));
    if (!selected.length) return false;
    boardClipboard = await Promise.all(selected.map(object => object.clone(CUSTOM_OBJECT_PROPERTIES)));
    updateRibbonState();
    return true;
}

async function cutSelection() {
    if (await copySelection()) deleteSelection();
}

async function pasteClipboard() {
    if (!boardClipboard.length) return;
    const clones = await Promise.all(boardClipboard.map(object => object.clone(CUSTOM_OBJECT_PROPERTIES)));
    restoringHistory = true;
    try {
        boardCanvas.discardActiveObject();
        clones.forEach(clone => {
            clone.set({
                left: (clone.left || 0) + 28,
                top: (clone.top || 0) + 28,
                boardObjectId: newBoardObjectId(),
            });
            clone.setCoords();
            boardCanvas.add(clone);
        });
        if (clones.length === 1) boardCanvas.setActiveObject(clones[0]);
        else boardCanvas.setActiveObject(new ActiveSelection(clones, { canvas: boardCanvas }));
    } finally {
        restoringHistory = false;
    }
    boardCanvas.requestRenderAll();
    boardClipboard = await Promise.all(clones.map(object => object.clone(CUSTOM_OBJECT_PROPERTIES)));
    captureHistory();
}

async function duplicateSelection() {
    if (await copySelection()) await pasteClipboard();
}

function changeSelectionLayer(direction) {
    const selected = boardCanvas.getActiveObjects().filter(object => object.boardKind !== 'connector' && !isConnectorHelper(object));
    if (!selected.length) return;
    selected.forEach(object => {
        if (direction === 'forward') boardCanvas.bringObjectForward(object);
        else boardCanvas.sendObjectBackwards(object);
    });
    refreshConnectors();
    boardCanvas.requestRenderAll();
    captureHistory();
}

function toggleSelectionLock() {
    const selected = boardCanvas.getActiveObjects().filter(object => object.boardKind !== 'connector' && !isConnectorHelper(object));
    if (!selected.length) {
        showNotification('Seleziona almeno un elemento da bloccare o sbloccare', 'info');
        return;
    }
    const shouldLock = selected.some(object => !object.boardLocked);
    selected.forEach(object => object.set({
        boardLocked: shouldLock,
        lockMovementX: shouldLock,
        lockMovementY: shouldLock,
        lockScalingX: shouldLock,
        lockScalingY: shouldLock,
        lockRotation: shouldLock,
        hasControls: !shouldLock,
    }));
    boardCanvas.discardActiveObject();
    boardCanvas.requestRenderAll();
    captureHistory();
    showNotification(shouldLock ? 'Elementi bloccati' : 'Elementi sbloccati', 'success');
}

function groupSelection() {
    const selected = boardCanvas.getActiveObjects().filter(object => object.boardKind !== 'connector' && !isConnectorHelper(object));
    if (selected.length < 2) {
        showNotification('Seleziona almeno due elementi con Shift+click', 'info');
        return;
    }
    if (selected.some(object => object.boardLocked)) {
        showNotification('Sblocca gli elementi prima di raggrupparli', 'info');
        return;
    }
    const selectedIDs = new Set(selected.map(object => object.boardObjectId).filter(Boolean));
    const hasConnections = boardCanvas.getObjects().some(object => (
        object.boardKind === 'connector'
        && (selectedIDs.has(object.connectorFrom) || selectedIDs.has(object.connectorTo))
    ));
    if (hasConnections) {
        showNotification('Rimuovi prima le frecce collegate agli elementi da raggruppare', 'info');
        return;
    }
    restoringHistory = true;
    try {
        boardCanvas.discardActiveObject();
        selected.forEach(object => boardCanvas.remove(object));
        const group = new Group(selected, {
            boardKind: 'group',
            originX: 'center',
            originY: 'center',
        });
        boardCanvas.add(group);
        boardCanvas.setActiveObject(group);
    } finally {
        restoringHistory = false;
    }
    boardCanvas.requestRenderAll();
    captureHistory();
}

function ungroupSelection() {
    const group = boardCanvas.getActiveObject();
    if (!group || group.boardKind !== 'group') {
        showNotification('Seleziona un gruppo da separare', 'info');
        return;
    }
    restoringHistory = true;
    let children;
    try {
        boardCanvas.discardActiveObject();
        boardCanvas.remove(group);
        children = group.removeAll();
        children.forEach(object => boardCanvas.add(object));
        if (children.length === 1) boardCanvas.setActiveObject(children[0]);
        else boardCanvas.setActiveObject(new ActiveSelection(children, { canvas: boardCanvas }));
        refreshConnectors();
    } finally {
        restoringHistory = false;
    }
    boardCanvas.requestRenderAll();
    captureHistory();
}

function alignSelection(mode) {
    const selected = boardCanvas.getActiveObjects().filter(object => object.boardKind !== 'connector' && !isConnectorHelper(object));
    if (selected.length < 2) {
        showNotification('Seleziona almeno due elementi con Shift+click', 'info');
        return;
    }
    if (mode === 'distribute-x' || mode === 'distribute-y') {
        if (selected.length < 3) {
            showNotification('Seleziona almeno tre elementi da distribuire', 'info');
            return;
        }
        const horizontal = mode === 'distribute-x';
        const ordered = [...selected].sort((a, b) => {
            const first = a.getCenterPoint();
            const second = b.getCenterPoint();
            return horizontal ? first.x - second.x : first.y - second.y;
        });
        const first = ordered[0].getCenterPoint();
        const last = ordered.at(-1).getCenterPoint();
        const spacing = (horizontal ? last.x - first.x : last.y - first.y) / (ordered.length - 1);
        ordered.slice(1, -1).forEach((object, index) => {
            const center = object.getCenterPoint();
            object.setPositionByOrigin(horizontal
                ? new Point(first.x + spacing * (index + 1), center.y)
                : new Point(center.x, first.y + spacing * (index + 1)), 'center', 'center');
            object.setCoords();
        });
    } else {
        const anchor = selected[0].getBoundingRect();
        const anchorCenter = selected[0].getCenterPoint();
        selected.slice(1).forEach(object => {
            const rect = object.getBoundingRect();
            const center = object.getCenterPoint();
            if (mode === 'left') object.set({ left: (object.left || 0) + anchor.left - rect.left });
            else if (mode === 'right') object.set({ left: (object.left || 0) + anchor.left + anchor.width - rect.left - rect.width });
            else if (mode === 'top') object.set({ top: (object.top || 0) + anchor.top - rect.top });
            else if (mode === 'bottom') object.set({ top: (object.top || 0) + anchor.top + anchor.height - rect.top - rect.height });
            else if (mode === 'center-x') object.set({ left: (object.left || 0) + anchorCenter.x - center.x });
            else if (mode === 'center-y') object.set({ top: (object.top || 0) + anchorCenter.y - center.y });
            object.setCoords();
        });
    }
    const activeSelection = boardCanvas.getActiveObject();
    if (objectType(activeSelection) === 'activeselection') {
        boardCanvas.discardActiveObject();
        selected.forEach(object => object.setCoords());
        boardCanvas.setActiveObject(new ActiveSelection(selected, { canvas: boardCanvas }));
    }
    refreshConnectors();
    boardCanvas.requestRenderAll();
    captureHistory();
}

function toggleSnapping() {
    snappingEnabled = !snappingEnabled;
    document.querySelectorAll('[data-board-command="snap"]').forEach(button => button.classList.toggle('active', snappingEnabled));
    showNotification(snappingEnabled ? 'Aggancio alla griglia attivato' : 'Aggancio alla griglia disattivato', 'info');
}

const BOARD_KIND_LABELS = {
    text: 'Testo',
    note: 'Nota',
    'legacy-note': 'Nota importata',
    shape: 'Forma',
    rectangle: 'Rettangolo',
    circle: 'Cerchio',
    arrow: 'Freccia',
    connector: 'Connettore',
    table: 'Tabella',
    frame: 'Cornice',
    image: 'Immagine',
    video: 'Video',
    link: 'Collegamento',
    group: 'Gruppo',
    drawing: 'Disegno',
    rect: 'Rettangolo',
    textbox: 'Testo',
};

function objectKindLabel(object) {
    return BOARD_KIND_LABELS[object?.boardKind] || BOARD_KIND_LABELS[object?.type?.toLowerCase()] || 'Oggetto';
}

function objectDisplayName(object) {
    if (object?.boardName) return object.boardName;
    if (typeof object?.text === 'string' && object.text.trim()) return object.text.trim().replace(/\s+/g, ' ').slice(0, 42);
    if (object?.linkLabel) return object.linkLabel;
    if (object?.assetName) return object.assetName;
    if (object?.videoName) return object.videoName;
    return objectKindLabel(object);
}

function updateInteractionHelp() {
    const help = document.getElementById('boardHelp');
    if (!help) return;
    const active = boardCanvas?.getActiveObject?.();
    if (connectorCreationMode) {
        help.textContent = connectorCreationFrom
            ? 'Collegamento: scegli l’oggetto di destinazione · Esc: annulla'
            : 'Collegamento: scegli il primo oggetto · Esc: annulla';
    } else if (drawingEnabled) {
        help.textContent = 'Matita attiva: trascina per disegnare · Spazio+trascina: sposta · Esc: termina';
    } else if (drawingEraserEnabled) {
        help.textContent = 'Gomma attiva: clicca un tratto per eliminarlo · Esc: termina';
    } else if (active?.boardKind === 'connector') {
        help.textContent = 'Doppio click: scrivi un’etichetta · Trascina le estremità verso gli 8 agganci · Canc: elimina';
    } else if (active?.boardKind === 'table') {
        help.textContent = 'Doppio click: modifica la tabella · Trascina: sposta · Canc: elimina';
    } else if (active) {
        help.textContent = 'Doppio click: apri o modifica · Shift+click: selezione multipla · Canc: elimina';
    } else {
        help.textContent = 'Rotellina: zoom · Spazio+trascina: sposta · Shift+click: selezione multipla';
    }
}

function updateStatusBar() {
    if (!boardCanvas) return;
    const selectedCount = boardCanvas.getActiveObjects().length;
    const objectCount = boardCanvas.getObjects().filter(object => object.boardKind !== 'connector' && !isConnectorHelper(object)).length;
    const zoom = Math.round(boardCanvas.getZoom() * 100);
    const selection = document.getElementById('boardSelectionStatus');
    const count = document.getElementById('boardObjectCount');
    const slider = document.getElementById('boardZoomSlider');
    const zoomValue = document.getElementById('boardZoomValue');
    if (selection) selection.textContent = selectedCount ? `${selectedCount} ${selectedCount === 1 ? 'elemento selezionato' : 'elementi selezionati'}` : 'Nessuna selezione';
    if (count) count.textContent = `${objectCount} ${objectCount === 1 ? 'oggetto' : 'oggetti'}`;
    if (slider) slider.value = String(Math.max(8, Math.min(800, zoom)));
    if (zoomValue) zoomValue.textContent = `${zoom}%`;
    updateInteractionHelp();
}

function renderMinimap() {
    const minimap = document.getElementById('boardMinimap');
    const canvas = document.getElementById('boardMinimapCanvas');
    if (!boardCanvas || !minimap || !canvas || !minimapVisible || minimap.hidden) return;
    const context = canvas.getContext('2d');
    const inverse = util.invertTransform(boardCanvas.viewportTransform);
    const viewTopLeft = util.transformPoint(new Point(0, 0), inverse);
    const viewBottomRight = util.transformPoint(new Point(boardCanvas.width, boardCanvas.height), inverse);
    const objectRects = boardCanvas.getObjects().filter(object => object.visible !== false && !isConnectorHelper(object)).map(object => ({ object, rect: object.getBoundingRect() }));
    const points = [
        { left: viewTopLeft.x, top: viewTopLeft.y, right: viewBottomRight.x, bottom: viewBottomRight.y },
        ...objectRects.map(({ rect }) => ({ left: rect.left, top: rect.top, right: rect.left + rect.width, bottom: rect.top + rect.height })),
    ];
    const bounds = points.reduce((acc, rect) => ({
        left: Math.min(acc.left, rect.left), top: Math.min(acc.top, rect.top),
        right: Math.max(acc.right, rect.right), bottom: Math.max(acc.bottom, rect.bottom),
    }), { left: Infinity, top: Infinity, right: -Infinity, bottom: -Infinity });
    const padding = 9;
    const width = Math.max(bounds.right - bounds.left, 1);
    const height = Math.max(bounds.bottom - bounds.top, 1);
    const scale = Math.min((canvas.width - padding * 2) / width, (canvas.height - padding * 2) / height);
    const offsetX = (canvas.width - width * scale) / 2 - bounds.left * scale;
    const offsetY = (canvas.height - height * scale) / 2 - bounds.top * scale;
    const mapRect = rect => ({ x: rect.left * scale + offsetX, y: rect.top * scale + offsetY, width: (rect.right - rect.left) * scale, height: (rect.bottom - rect.top) * scale });

    context.clearRect(0, 0, canvas.width, canvas.height);
    context.fillStyle = '#1C1815';
    context.fillRect(0, 0, canvas.width, canvas.height);
    objectRects.forEach(({ object, rect }) => {
        const mapped = mapRect({ left: rect.left, top: rect.top, right: rect.left + rect.width, bottom: rect.top + rect.height });
        context.fillStyle = object.boardKind === 'image' ? '#60a5fa' : object.boardKind === 'note' ? '#facc15' : object.boardKind === 'connector' ? '#9ca3af' : '#d1d5db';
        context.globalAlpha = object.boardKind === 'connector' ? 0.65 : 0.82;
        context.fillRect(mapped.x, mapped.y, Math.max(mapped.width, 2), Math.max(mapped.height, 2));
    });
    context.globalAlpha = 1;
    const viewportRect = mapRect({ left: viewTopLeft.x, top: viewTopLeft.y, right: viewBottomRight.x, bottom: viewBottomRight.y });
    context.strokeStyle = '#C99A54';
    context.lineWidth = 2;
    context.strokeRect(viewportRect.x, viewportRect.y, viewportRect.width, viewportRect.height);
    minimapMapping = { scale, offsetX, offsetY };
}

function setZoom(zoom) {
    if (!boardCanvas) return;
    const constrained = Math.max(0.08, Math.min(Number(zoom), 8));
    boardCanvas.zoomToPoint(new Point(boardCanvas.width / 2, boardCanvas.height / 2), constrained);
    boardCanvas.requestRenderAll();
    updateStatusBar();
    scheduleSave();
}

function toggleMinimap() {
    minimapVisible = !minimapVisible;
    const minimap = document.getElementById('boardMinimap');
    if (minimap) minimap.hidden = !minimapVisible;
    document.querySelectorAll('[data-board-command="toggle-minimap"]').forEach(button => button.classList.toggle('active', minimapVisible));
    if (minimapVisible) renderMinimap();
}

function renderObjectPanel() {
    const container = document.getElementById('boardObjectList');
    if (!container || !boardCanvas) return;
    const search = (document.getElementById('boardObjectSearch')?.value || '').trim().toLocaleLowerCase('it');
    const active = new Set(boardCanvas.getActiveObjects());
    const objects = [...boardCanvas.getObjects()].reverse().filter(object => !isConnectorHelper(object));
    container.replaceChildren();
    objects.filter(object => objectDisplayName(object).toLocaleLowerCase('it').includes(search)).forEach(object => {
        const row = document.createElement('div');
        row.className = `board-object-row${active.has(object) ? ' selected' : ''}`;
        const main = document.createElement('button');
        main.type = 'button';
        main.className = 'board-object-main';
        main.textContent = objectDisplayName(object);
        main.title = `${objectKindLabel(object)} · doppio clic per rinominare`;
        main.addEventListener('click', event => {
            if (object.visible === false) return;
            if (connectorCreationMode) {
                chooseConnectorTarget(object);
                return;
            }
            const modifier = event.shiftKey || event.ctrlKey || event.metaKey;
            if (modifier) {
                const current = boardCanvas.getActiveObjects().filter(item => item !== object);
                const objects = active.has(object) ? current : [...current, object];
                if (objects.length > 1) boardCanvas.setActiveObject(new ActiveSelection(objects, { canvas: boardCanvas }));
                else if (objects.length === 1) boardCanvas.setActiveObject(objects[0]);
                else boardCanvas.discardActiveObject();
            } else boardCanvas.setActiveObject(object);
            boardCanvas.requestRenderAll();
        });
        main.addEventListener('dblclick', () => {
            const name = window.prompt('Nome dell’oggetto', objectDisplayName(object));
            if (name === null) return;
            object.boardName = name.trim().slice(0, 80);
            captureHistory();
            renderObjectPanel();
        });
        const actions = document.createElement('div');
        actions.className = 'board-object-actions';
        const buttons = [
            ['rename', '✎', 'Rinomina'],
            ['visibility', object.visible === false ? '○' : '●', object.visible === false ? 'Mostra' : 'Nascondi'],
            ['lock', object.boardLocked ? '■' : '□', object.boardLocked ? 'Sblocca' : 'Blocca'],
            ['up', '↑', 'Porta avanti'],
            ['down', '↓', 'Porta indietro'],
        ];
        buttons.forEach(([action, text, title]) => {
            const button = document.createElement('button');
            button.type = 'button';
            button.textContent = text;
            button.title = title;
            button.setAttribute('aria-label', `${title}: ${objectDisplayName(object)}`);
            button.addEventListener('click', () => handleObjectPanelAction(action, object));
            actions.appendChild(button);
        });
        row.append(main, actions);
        container.appendChild(row);
    });
    if (!container.children.length) {
        const empty = document.createElement('p');
        empty.className = 'board-side-empty';
        empty.textContent = search ? 'Nessun oggetto corrisponde alla ricerca.' : 'La lavagna non contiene ancora oggetti.';
        container.appendChild(empty);
    }
}

function handleObjectPanelAction(action, object) {
    if (!boardCanvas) return;
    if (action === 'rename') {
        const name = window.prompt('Nome dell’oggetto', objectDisplayName(object));
        if (name === null) return;
        object.boardName = name.trim().slice(0, 80);
    } else if (action === 'visibility') {
        object.set('visible', object.visible === false);
        if (object.visible === false) boardCanvas.discardActiveObject();
    } else if (action === 'lock') {
        boardCanvas.setActiveObject(object);
        toggleSelectionLock();
        return;
    } else if (action === 'up') boardCanvas.bringObjectForward(object);
    else if (action === 'down') boardCanvas.sendObjectBackwards(object);
    object.setCoords();
    boardCanvas.requestRenderAll();
    captureHistory();
    renderObjectPanel();
}

function renderTrashPanel() {
    const container = document.getElementById('boardTrashList');
    if (!container) return;
    container.replaceChildren();
    [...boardTrash].reverse().forEach(entry => {
        const row = document.createElement('div');
        row.className = 'board-object-row';
        const label = document.createElement('span');
        label.className = 'board-object-main';
        label.textContent = entry.name || 'Oggetto eliminato';
        const actions = document.createElement('div');
        actions.className = 'board-object-actions';
        const restore = document.createElement('button');
        restore.type = 'button';
        restore.textContent = 'Ripristina';
        restore.addEventListener('click', () => restoreTrashEntry(entry.id));
        actions.appendChild(restore);
        row.append(label, actions);
        container.appendChild(row);
    });
    if (!boardTrash.length) {
        const empty = document.createElement('p');
        empty.className = 'board-side-empty';
        empty.textContent = 'Il cestino è vuoto.';
        container.appendChild(empty);
    }
}

function callBoardBackend(method, ...args) {
    const handler = window.go?.main?.App?.[method];
    if (typeof handler !== 'function') return Promise.reject(new Error(`funzione ${method} non disponibile`));
    return handler(...args);
}

function formatVersionDate(value) {
    if (!value) return '';
    const normalized = String(value).includes('T') ? String(value) : `${String(value).replace(' ', 'T')}Z`;
    const date = new Date(normalized);
    return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString('it-IT', { dateStyle: 'short', timeStyle: 'short' });
}

function renderBoardVersions(versions) {
    const container = document.getElementById('boardVersionList');
    if (!container) return;
    container.replaceChildren();
    versions.forEach(version => {
        const row = document.createElement('div');
        row.className = 'board-version-row';
        const label = document.createElement('div');
        label.textContent = version.name;
        const meta = document.createElement('span');
        meta.className = 'board-version-meta';
        meta.textContent = `${version.is_automatic ? 'Automatica' : 'Nominata'} · ${formatVersionDate(version.created_at)}`;
        label.appendChild(meta);
        const actions = document.createElement('div');
        actions.className = 'board-object-actions';
        const restore = document.createElement('button');
        restore.type = 'button';
        restore.textContent = 'Ripristina';
        restore.addEventListener('click', () => restoreBoardVersion(version));
        const remove = document.createElement('button');
        remove.type = 'button';
        remove.textContent = '×';
        remove.title = 'Elimina versione';
        remove.addEventListener('click', () => deleteBoardVersion(version));
        actions.append(restore, remove);
        row.append(label, actions);
        container.appendChild(row);
    });
    if (!versions.length) {
        const empty = document.createElement('p');
        empty.className = 'board-side-empty';
        empty.textContent = 'Non sono ancora presenti versioni salvate.';
        container.appendChild(empty);
    }
}

async function loadBoardVersions() {
    if (!currentProject || currentProject.preview) {
        renderBoardVersions([]);
        return;
    }
    const container = document.getElementById('boardVersionList');
    if (container) container.textContent = 'Caricamento…';
    try {
        renderBoardVersions(await callBoardBackend('ListBoardVersions', currentProject.id));
    } catch (error) {
        if (container) container.textContent = `Impossibile caricare le versioni: ${error}`;
    }
}

async function createNamedBoardVersion() {
    const input = document.getElementById('boardVersionName');
    const name = input?.value.trim() || '';
    if (!name) {
        showNotification('Inserisci un nome per la versione', 'info');
        input?.focus();
        return;
    }
    if (currentProject?.preview) {
        showNotification('Le versioni persistenti sono disponibili nell’app installata', 'info');
        return;
    }
    try {
        await saveBoard();
        await callBoardBackend('CreateBoardVersion', currentProject.id, name);
        input.value = '';
        await loadBoardVersions();
        showNotification('Versione della lavagna salvata', 'success');
    } catch (error) {
        showNotification(`Impossibile creare la versione: ${error}`, 'error');
    }
}

async function restoreBoardVersion(version) {
    if (!window.confirm(`Ripristinare “${version.name}”? Lo stato attuale verrà conservato nella cronologia.`)) return;
    try {
        const restored = await callBoardBackend('RestoreBoardVersion', currentProject.id, version.id);
        await initializeCanvas(restored, '');
        await loadBoardVersions();
        showNotification('Versione ripristinata', 'success');
    } catch (error) {
        showNotification(`Impossibile ripristinare la versione: ${error}`, 'error');
    }
}

async function deleteBoardVersion(version) {
    if (!window.confirm(`Eliminare definitivamente la versione “${version.name}”?`)) return;
    try {
        await callBoardBackend('DeleteBoardVersion', currentProject.id, version.id);
        await loadBoardVersions();
        showNotification('Versione eliminata', 'success');
    } catch (error) {
        showNotification(`Impossibile eliminare la versione: ${error}`, 'error');
    }
}

// Ripristina un oggetto dal cestino sul canvas senza registrare history né
// ridisegnare il pannello: usata sia per il ripristino di una singola riga
// (che poi fa lei stessa il resto) sia da restoreAllTrash, che deve poter
// ripristinare N oggetti pagando una sola captureHistory()/renderTrashPanel()
// invece di una per oggetto (captureHistory serializza l'intero canvas: farlo
// N volte per un ripristino multiplo è O(n) sprecato).
async function restoreTrashEntryQuiet(entryID) {
    const index = boardTrash.findIndex(entry => entry.id === entryID);
    if (index < 0) return null;
    const [entry] = boardTrash.splice(index, 1);
    const hydrated = await hydrateBoardDocument({ objects: [entry.object] });
    const [object] = await util.enlivenObjects(hydrated.objects);
    if (!object) return null;
    object.set({ left: (object.left || 0) + 18, top: (object.top || 0) + 18, visible: true });
    object.setCoords();
    boardCanvas.add(object);
    boardCanvas.setActiveObject(object);
    return object;
}

async function restoreTrashEntry(entryID) {
    const object = await restoreTrashEntryQuiet(entryID);
    if (!object) return;
    boardCanvas.requestRenderAll();
    captureHistory();
    renderTrashPanel();
}

async function restoreAllTrash() {
    const identifiers = boardTrash.map(entry => entry.id);
    let restoredAny = false;
    for (const identifier of identifiers) {
        const object = await restoreTrashEntryQuiet(identifier);
        if (object) restoredAny = true;
    }
    if (!restoredAny) return;
    boardCanvas.requestRenderAll();
    captureHistory();
    renderTrashPanel();
}

function emptyTrash() {
    if (!boardTrash.length) return;
    if (!window.confirm('Svuotare definitivamente il cestino della lavagna?')) return;
    boardTrash = [];
    captureHistory();
    renderTrashPanel();
}

function setBoardSideTab(name) {
    document.querySelectorAll('[data-board-side-tab]').forEach(tab => tab.setAttribute('aria-selected', String(tab.dataset.boardSideTab === name)));
    document.querySelectorAll('[data-board-side-panel]').forEach(panel => { panel.hidden = panel.dataset.boardSidePanel !== name; });
    if (name === 'objects') renderObjectPanel();
    if (name === 'trash') renderTrashPanel();
    if (name === 'versions') loadBoardVersions();
}

function toggleSidePanel() {
    const panel = document.getElementById('boardSidePanel');
    const workspace = document.querySelector('.board-workspace');
    if (!panel || !workspace) return;
    const opening = panel.hidden;
    panel.hidden = !opening;
    workspace.classList.toggle('side-panel-open', opening);
    document.querySelectorAll('[data-board-command="toggle-layers"]').forEach(button => button.classList.toggle('active', opening));
    if (opening) renderObjectPanel();
    requestAnimationFrame(resizeCanvas);
}

function closeInlineTextEditor(commit = true) {
    if (!inlineTextEditor) return;
    const { element, object, originalValue } = inlineTextEditor;
    const value = commit ? element.value.slice(0, 1000) : originalValue;
    inlineTextEditor = null;
    element.remove();
    if (object?.boardKind === 'connector') {
        object.set('connectorLabel', value.replace(/\r?\n/g, ' '));
        refreshConnectors();
        const updated = boardCanvas.getObjects().find(item => item.boardObjectId === object.boardObjectId);
        if (updated) boardCanvas.setActiveObject(updated);
    } else {
        const label = shapeLabel(object);
        if (label) {
            label.set('text', value);
            label.initDimensions?.();
            object.setCoords();
        }
    }
    boardCanvas.requestRenderAll();
    if (commit && value !== originalValue) captureHistory();
}

function beginInlineTextEdit(requestedObject, initialCharacter = '') {
    closeInlineTextEditor(true);
    let object = requestedObject;
    if (!object) return false;
    if (object.boardKind !== 'connector') object = ensureShapeTextContainer(object);
    if (!object) return false;
    const currentValue = object.boardKind === 'connector'
        ? String(object.connectorLabel || '')
        : String(shapeLabel(object)?.text || '');
    const wrapper = document.getElementById('projectBoardCanvasWrap');
    const point = util.transformPoint(object.getCenterPoint(), boardCanvas.viewportTransform);
    const bounds = object.getBoundingRect();
    const zoom = boardCanvas.getZoom();
    const width = object.boardKind === 'connector'
        ? Math.max(150, Math.min(320, bounds.width * zoom * 0.6))
        : Math.max(140, Math.min(440, bounds.width * zoom - 24));
    const height = object.boardKind === 'connector'
        ? 42
        : Math.max(58, Math.min(260, bounds.height * zoom - 20));
    const editor = document.createElement('textarea');
    editor.className = 'board-inline-text-editor';
    editor.value = initialCharacter || currentValue;
    editor.rows = object.boardKind === 'connector' ? 1 : 3;
    editor.setAttribute('aria-label', object.boardKind === 'connector' ? 'Etichetta del collegamento' : 'Testo della forma');
    editor.style.left = `${Math.max(8, Math.min(wrapper.clientWidth - width - 8, point.x - width / 2))}px`;
    editor.style.top = `${Math.max(8, Math.min(wrapper.clientHeight - height - 34, point.y - height / 2))}px`;
    editor.style.width = `${width}px`;
    editor.style.height = `${height}px`;
    const textSource = object.boardKind === 'connector'
        ? object.getObjects?.().find(child => child.boardKind === 'connector-label')
        : shapeLabel(object);
    if (textSource) {
        editor.style.color = String(textSource.fill || '#ffffff');
        editor.style.fontFamily = textSource.fontFamily || 'Arial';
        editor.style.fontSize = `${Math.max(12, Number(textSource.fontSize) * zoom || 17)}px`;
        editor.style.fontWeight = textSource.fontWeight || 'normal';
        editor.style.fontStyle = textSource.fontStyle || 'normal';
        editor.style.textDecoration = textSource.underline ? 'underline' : 'none';
    }
    wrapper.appendChild(editor);
    inlineTextEditor = { element: editor, object, originalValue: currentValue };
    removeConnectorHelpers();
    editor.addEventListener('keydown', event => {
        event.stopPropagation();
        if (event.key === 'Escape') {
            event.preventDefault();
            closeInlineTextEditor(false);
        } else if (event.key === 'Enter' && (object.boardKind === 'connector' || event.ctrlKey || event.metaKey)) {
            event.preventDefault();
            closeInlineTextEditor(true);
        }
    });
    editor.addEventListener('blur', () => closeInlineTextEditor(true), { once: true });
    editor.focus();
    if (!initialCharacter && currentValue) editor.select();
    return true;
}

function editSelectedObject() {
    const object = boardCanvas?.getActiveObject();
    if (!object) return;
    if (object.boardKind === 'table') openTableEditor(object);
    else if (object.boardKind === 'link' || object.boardKind === 'video') openLinkObject(object);
    else if (object.boardKind === 'connector' || ['rectangle', 'circle', 'shape'].includes(object.boardKind)) beginInlineTextEdit(object);
    else if (typeof object.enterEditing === 'function') {
        object.enterEditing();
        object.selectAll?.();
        boardCanvas.requestRenderAll();
    } else showNotification('Questo oggetto si modifica tramite i comandi di formato', 'info');
}

function updateContextualTab() {
    const active = boardCanvas?.getActiveObject?.();
    const tab = document.querySelector('[data-board-context-tab]');
    const title = document.getElementById('boardContextTitle');
    const hint = document.getElementById('boardContextHint');
    if (!tab) return;
    const hasSelection = Boolean(active);
    tab.hidden = !hasSelection;
    if (!hasSelection) return;
    const label = objectKindLabel(active);
    tab.textContent = `Formato ${label}`;
    if (title) title.textContent = label;
    if (hint) hint.textContent = active.boardKind === 'table'
        ? 'Usa Modifica per cambiare struttura, contenuto e formato delle singole celle.'
        : active.boardKind === 'connector'
            ? 'Doppio click o Modifica per aggiungere un’etichetta al collegamento.'
            : ['rectangle', 'circle', 'shape'].includes(active.boardKind)
                ? 'Doppio click o inizia a digitare per scrivere nella forma.'
        : active.boardKind === 'link' || active.boardKind === 'video'
            ? 'Apri la risorsa oppure disponi e blocca la relativa scheda.'
            : 'Colore, testo e spessore sono disponibili nella scheda Home.';
}

function setBoardRibbonTab(tabName, focusTab = false) {
    const tab = document.querySelector(`[data-board-tab="${tabName}"]`);
    const panel = document.querySelector(`[data-board-panel="${tabName}"]`);
    if (!tab || tab.hidden || !panel) return;
    document.querySelectorAll('.board-shape-gallery').forEach(gallery => { gallery.hidden = true; });
    closeColorGallery();
    document.querySelectorAll('[data-board-tab]').forEach(item => item.setAttribute('aria-selected', String(item === tab)));
    document.querySelectorAll('[data-board-panel]').forEach(item => {
        const active = item === panel;
        item.hidden = !active;
        item.classList.toggle('active', active);
    });
    if (focusTab) tab.focus();
    requestAnimationFrame(resizeCanvas);
}

function updateRibbonState() {
    const selected = boardCanvas?.getActiveObjects?.() || [];
    const active = boardCanvas?.getActiveObject?.();
    const selectedWithoutConnectors = selected.filter(object => object.boardKind !== 'connector' && !isConnectorHelper(object));
    const hasSelection = selected.length > 0;
    const setDisabled = (command, disabled) => {
        document.querySelectorAll(`[data-board-command="${command}"]`).forEach(button => {
            button.disabled = disabled;
            button.setAttribute('aria-disabled', String(disabled));
        });
    };

    ['copy', 'cut', 'duplicate', 'delete', 'lock', 'forward', 'backward'].forEach(command => setDisabled(command, !hasSelection));
    const hasTextSelection = selected.some(object => selectionContains(object, isTextObject));
    const hasListTextSelection = selected.some(object => selectionContains(object, isListTextObject));
    document.querySelectorAll('[data-board-text-style]').forEach(button => {
        button.disabled = !hasTextSelection;
        button.setAttribute('aria-disabled', String(!hasTextSelection));
    });
    document.querySelectorAll('[data-board-list-command]').forEach(button => {
        button.disabled = !hasListTextSelection;
        button.setAttribute('aria-disabled', String(!hasListTextSelection));
    });
    setDisabled('paste', boardClipboard.length === 0);
    setDisabled('undo', undoStack.length < 2);
    setDisabled('redo', redoStack.length === 0);
    setDisabled('group', selectedWithoutConnectors.length < 2);
    setDisabled('ungroup', active?.boardKind !== 'group');
    setDisabled('align-x', selectedWithoutConnectors.length < 2);
    setDisabled('align-y', selectedWithoutConnectors.length < 2);
    setDisabled('align-menu', selectedWithoutConnectors.length < 2);
    setDisabled('connect', false);
    setDisabled('open-link', !selected.some(object => object.boardKind === 'link' || object.boardKind === 'video'));
    setDisabled('edit-object', !active || !(['table', 'link', 'video', 'connector', 'rectangle', 'circle', 'shape'].includes(active.boardKind) || typeof active.enterEditing === 'function'));

    const contextTab = document.querySelector('[data-board-context-tab]');
    updateContextualTab();
    if (!hasSelection && contextTab?.getAttribute('aria-selected') === 'true') setBoardRibbonTab('home');
    updateStatusBar();
    renderObjectPanel();
    renderMinimap();
}

function toggleBoardRibbon() {
    const ribbon = document.querySelector('.board-ribbon');
    const toggle = document.querySelector('[data-board-ribbon-toggle]');
    if (!ribbon || !toggle) return;
    const collapsed = ribbon.classList.toggle('collapsed');
    toggle.textContent = collapsed ? '⌄' : '⌃';
    toggle.title = collapsed ? 'Espandi barra comandi' : 'Comprimi barra comandi';
    toggle.setAttribute('aria-label', toggle.title);
    toggle.setAttribute('aria-expanded', String(!collapsed));
    requestAnimationFrame(resizeCanvas);
}

function closeColorGallery() {
    const gallery = document.getElementById('boardColorGallery');
    if (gallery) gallery.hidden = true;
    document.querySelectorAll('[data-board-color-target]').forEach(button => button.setAttribute('aria-expanded', 'false'));
    activeColorTarget = null;
}

function setBoardColorValue(target, value) {
    const control = BOARD_COLOR_CONTROLS[target];
    if (!control || !/^#[0-9a-f]{6}$/i.test(value)) return;
    const input = document.getElementById(control.inputID);
    const preview = document.querySelector(`[data-board-color-preview="${target}"]`);
    if (input) input.value = value;
    if (preview) preview.style.setProperty('--board-selected-color', value);
    if (activeColorTarget === target) {
        document.querySelectorAll('[data-board-color-value]').forEach(button => {
            button.classList.toggle('active', button.dataset.boardColorValue.toLowerCase() === value.toLowerCase());
        });
    }
}

function openColorGallery(target, anchorButton = null) {
    const control = BOARD_COLOR_CONTROLS[target];
    const gallery = document.getElementById('boardColorGallery');
    if (!control || !gallery) return;
    const willOpen = gallery.hidden || activeColorTarget !== target;
    closeColorGallery();
    if (!willOpen) return;
    activeColorTarget = target;
    gallery.hidden = false;
    document.getElementById('boardColorGalleryTitle').textContent = control.label;
    document.querySelector(`[data-board-color-target="${target}"]`)?.setAttribute('aria-expanded', 'true');
    const anchor = anchorButton || document.querySelector(`[data-board-color-target="${target}"]`);
    const ribbon = gallery.closest('.board-ribbon');
    if (anchor && ribbon) {
        const anchorRect = anchor.getBoundingClientRect();
        const ribbonRect = ribbon.getBoundingClientRect();
        const preferredLeft = anchorRect.left - ribbonRect.left;
        const below = anchorRect.bottom - ribbonRect.top + 5;
        const above = anchorRect.top - ribbonRect.top - gallery.offsetHeight - 5;
        gallery.style.left = `${Math.max(8, Math.min(ribbon.clientWidth - gallery.offsetWidth - 8, preferredLeft))}px`;
        gallery.style.top = `${anchorRect.bottom + gallery.offsetHeight + 8 <= window.innerHeight || above < 8 ? below : above}px`;
    }
    const value = document.getElementById(control.inputID)?.value || '';
    document.querySelectorAll('[data-board-color-value]').forEach(button => {
        button.classList.toggle('active', button.dataset.boardColorValue.toLowerCase() === value.toLowerCase());
    });
}

function objectType(object) {
    return String(object?.type || '').toLowerCase();
}

function visitBoardObject(object, callback, rootKind = object?.boardKind) {
    callback(object, rootKind);
    if (typeof object?.forEachObject === 'function') {
        object.forEachObject(child => visitBoardObject(child, callback, rootKind));
    }
}

function selectionContains(object, predicate) {
    let found = false;
    visitBoardObject(object, child => {
        if (predicate(child)) found = true;
    });
    return found;
}

function isTextObject(object) {
    return typeof object?.text === 'string';
}

function isListTextObject(object) {
    return isTextObject(object) && ['text', 'note', 'legacy-note'].includes(object.boardKind);
}

function supportsLineStyle(object) {
    return object?.boardKind === 'connector'
        || ['line', 'path'].includes(objectType(object))
        || (object?.stroke !== undefined && object.stroke !== null && object.stroke !== '');
}

function supportsFillStyle(object, rootKind) {
    if (object?.boardKind === 'note') return true;
    if (rootKind === 'arrow') return false;
    return ['rect', 'circle', 'triangle', 'polygon'].includes(objectType(object));
}

function syncTableStyleMetadata(table) {
    if (table?.boardKind !== 'table' || typeof table.getObjects !== 'function') return;
    const data = normalizeTableData(table.tableData);
    const children = table.getObjects();
    const styles = resizeTableCellStyles(table.tableCellStyles, data.length, data[0].length);
    data.forEach((row, rowIndex) => row.forEach((cell, columnIndex) => {
        const flatIndex = (rowIndex * row.length + columnIndex) * 2;
        const background = children[flatIndex];
        const text = children[flatIndex + 1];
        styles[rowIndex][columnIndex] = normalizeTableCellStyle({
            ...styles[rowIndex][columnIndex],
            fill: background?.fill,
            borderColor: background?.stroke,
            borderWidth: background?.strokeWidth,
            textColor: text?.fill,
            fontFamily: text?.fontFamily,
            fontSize: text?.fontSize,
            fontWeight: text?.fontWeight,
            fontStyle: text?.fontStyle,
            underline: text?.underline,
            horizontalAlign: text?.textAlign,
        }, rowIndex);
    }));
    table.set('tableCellStyles', styles);
}

function syncConnectorLabelMetadata(connector) {
    if (connector?.boardKind !== 'connector' || typeof connector.getObjects !== 'function') return;
    const label = connector.getObjects().find(child => child.boardKind === 'connector-label');
    if (!label) return;
    connector.set({
        connectorLabel: label.text || '',
        connectorLabelStyle: {
            fill: label.fill,
            fontFamily: label.fontFamily,
            fontSize: label.fontSize,
            fontWeight: label.fontWeight,
            fontStyle: label.fontStyle,
            underline: label.underline,
            backgroundColor: label.backgroundColor,
        },
    });
}

function hexToRGBA(hex, transparencyPercent) {
    const normalized = String(hex || '').trim().replace('#', '');
    if (!/^[0-9a-f]{6}$/i.test(normalized)) return 'rgba(0, 0, 0, 0)';
    const red = parseInt(normalized.slice(0, 2), 16);
    const green = parseInt(normalized.slice(2, 4), 16);
    const blue = parseInt(normalized.slice(4, 6), 16);
    const alpha = Math.max(0, Math.min(1, 1 - Number(transparencyPercent || 0) / 100));
    return `rgba(${red}, ${green}, ${blue}, ${alpha})`;
}

function applyTextValues(object, style) {
    if (!isTextObject(object)) return;
    const values = {};
    if (style.color !== undefined) values.fill = style.color;
    if (style.fontFamily !== undefined) values.fontFamily = style.fontFamily;
    if (style.fontSize !== undefined) values.fontSize = style.fontSize;
    const hasTextSelection = object.isEditing && object.selectionEnd > object.selectionStart && typeof object.setSelectionStyles === 'function';
    if (hasTextSelection) object.setSelectionStyles(values, object.selectionStart, object.selectionEnd);
    else object.set(values);
}

function applySelectedTextStyle(styleOverride = null, notifyMissing = true) {
    const selected = boardCanvas.getActiveObjects().filter(object => !isConnectorHelper(object));
    const textObjects = selected.filter(object => selectionContains(object, isTextObject));
    if (!textObjects.length) {
        if (notifyMissing) showNotification('Seleziona un testo o un oggetto che contiene testo', 'info');
        return;
    }
    const style = styleOverride || {
        color: document.getElementById('boardTextColorPicker').value,
        fontFamily: document.getElementById('boardFontFamily').value,
        fontSize: Number(document.getElementById('boardFontSize').value),
    };
    textObjects.forEach(object => visitBoardObject(object, child => applyTextValues(child, style)));
    textObjects.forEach(object => {
        syncTableStyleMetadata(object);
        syncConnectorLabelMetadata(object);
    });
    if (textObjects.some(object => object.boardKind === 'connector')) refreshConnectors();
    boardCanvas.requestRenderAll();
    captureHistory();
}

function textStyleAtCursor(object) {
    if (!isTextObject(object)) return {};
    if (object.isEditing && typeof object.getSelectionStyles === 'function') {
        const style = object.getSelectionStyles(object.selectionStart, Math.max(object.selectionEnd, object.selectionStart + 1))[0];
        if (style) return { ...object, ...style };
    }
    return object;
}

function applyTextStyleCommand(command) {
    const selected = boardCanvas.getActiveObjects().filter(object => !isConnectorHelper(object));
    const targets = [];
    selected.forEach(object => visitBoardObject(object, child => {
        if (isTextObject(child)) targets.push(child);
    }));
    if (!targets.length) {
        showNotification('Seleziona un testo da formattare', 'info');
        return;
    }
    targets.forEach(object => {
        const current = textStyleAtCursor(object);
        const baseSize = Number(object.fontSize) || Number(document.getElementById('boardFontSize').value) || 24;
        let values;
        if (command === 'normal') values = { fontWeight: 'normal', fontStyle: 'normal', underline: false, deltaY: 0, fontSize: baseSize };
        else if (command === 'bold') values = { fontWeight: String(current.fontWeight) === 'bold' ? 'normal' : 'bold' };
        else if (command === 'italic') values = { fontStyle: current.fontStyle === 'italic' ? 'normal' : 'italic' };
        else if (command === 'underline') values = { underline: !current.underline };
        else if (command === 'subscript') values = Number(current.deltaY) > 0
            ? { deltaY: 0, fontSize: baseSize }
            : { deltaY: baseSize * 0.3, fontSize: baseSize * 0.65 };
        else if (command === 'superscript') values = Number(current.deltaY) < 0
            ? { deltaY: 0, fontSize: baseSize }
            : { deltaY: -baseSize * 0.3, fontSize: baseSize * 0.65 };
        if (!values) return;
        const hasTextSelection = object.isEditing && object.selectionEnd > object.selectionStart && typeof object.setSelectionStyles === 'function';
        if (hasTextSelection) object.setSelectionStyles(values, object.selectionStart, object.selectionEnd);
        else object.set(values);
    });
    boardCanvas.requestRenderAll();
    captureHistory();
}

function applyTextListCommand(command) {
    const selected = boardCanvas.getActiveObjects().filter(object => !isConnectorHelper(object));
    const targets = [];
    selected.forEach(object => visitBoardObject(object, child => {
        if (isListTextObject(child)) targets.push(child);
    }));
    if (!targets.length) {
        showNotification('Seleziona un testo o una nota per creare un elenco', 'info');
        return;
    }

    targets.forEach(object => {
        const edits = listPrefixEdits(object.text, {
            mode: command,
            start: object.selectionStart,
            end: object.selectionEnd,
            editing: Boolean(object.isEditing),
        }).sort((first, second) => second.start - first.start);
        edits.forEach(edit => {
            if (edit.removeLength > 0 && typeof object.removeChars === 'function') {
                object.removeChars(edit.start, edit.start + edit.removeLength);
            }
            if (edit.insert && typeof object.insertChars === 'function') {
                object.insertChars(edit.insert, undefined, edit.start);
            }
        });
        object.setCoords();
    });
    selected.forEach(object => {
        syncTableStyleMetadata(object);
        syncConnectorLabelMetadata(object);
    });
    if (selected.some(object => object.boardKind === 'connector')) refreshConnectors();
    boardCanvas.requestRenderAll();
    captureHistory();
}

function applySelectedObjectStyle(styleOverride = null, notifyMissing = true) {
    const selected = boardCanvas.getActiveObjects().filter(object => !isConnectorHelper(object));
    if (!selected.length) {
        if (notifyMissing) showNotification('Seleziona almeno una forma o un oggetto', 'info');
        return;
    }
    const allStyles = styleOverride === null;
    const style = styleOverride || {};
    const lineColor = style.lineColor ?? document.getElementById('boardLineColorPicker').value;
    const fillColor = style.fillColor ?? hexToRGBA(
        document.getElementById('boardFillColorPicker').value,
        Number(document.getElementById('boardFillOpacity').value),
    );
    const strokeWidth = style.strokeWidth ?? Number(document.getElementById('boardStrokeWidth').value);
    const applyLineColor = allStyles || style.lineColor !== undefined;
    const applyStrokeWidth = allStyles || style.strokeWidth !== undefined;
    const applyFill = allStyles || style.fillColor !== undefined;
    let changed = false;
    selected.forEach(object => visitBoardObject(object, (child, rootKind) => {
        if (supportsLineStyle(child)) {
            const values = {};
            if (applyLineColor) values.stroke = lineColor;
            if (applyStrokeWidth) values.strokeWidth = strokeWidth;
            if (Object.keys(values).length) {
                child.set(values);
                changed = true;
            }
        }
        if (applyLineColor && rootKind === 'arrow' && objectType(child) === 'triangle') {
            child.set({ fill: lineColor });
            changed = true;
        } else if (applyFill && child.boardKind === 'note') {
            child.set({ backgroundColor: fillColor });
            changed = true;
        } else if (applyFill && supportsFillStyle(child, rootKind)) {
            child.set({ fill: fillColor });
            changed = true;
        }
    }));
    if (!changed) {
        if (notifyMissing) showNotification('L’oggetto selezionato non dispone di contorno o sfondo modificabile', 'info');
        return;
    }
    selected.forEach(object => syncTableStyleMetadata(object));
    refreshConnectors();
    boardCanvas.requestRenderAll();
    captureHistory();
}

function applyCurrentColor(target) {
    if (target === 'text') {
        applySelectedTextStyle({ color: document.getElementById('boardTextColorPicker').value }, false);
    } else if (target === 'line') {
        applySelectedObjectStyle({ lineColor: document.getElementById('boardLineColorPicker').value }, false);
    } else if (target === 'fill') {
        applySelectedObjectStyle({
            fillColor: hexToRGBA(
                document.getElementById('boardFillColorPicker').value,
                Number(document.getElementById('boardFillOpacity').value),
            ),
        }, false);
    }
}

function boardContentBounds() {
    const objects = boardCanvas.getObjects().filter(object => !isConnectorHelper(object));
    if (!objects.length) return null;
    return objects.reduce((bounds, object) => {
        const rect = object.getBoundingRect();
        return {
            left: Math.min(bounds.left, rect.left),
            top: Math.min(bounds.top, rect.top),
            right: Math.max(bounds.right, rect.left + rect.width),
            bottom: Math.max(bounds.bottom, rect.top + rect.height),
        };
    }, { left: Infinity, top: Infinity, right: -Infinity, bottom: -Infinity });
}

function safeExportFileName(value) {
    return String(value || 'lavagna').replace(/[<>:"/\\|?*\u0000-\u001f]/g, '-').slice(0, 80) || 'lavagna';
}

function renderWholeBoard(format, requestedMultiplier, backgroundColor = '') {
    const bounds = boardContentBounds();
    if (!bounds) return null;
    const padding = 50;
    const viewport = [...boardCanvas.viewportTransform];
    const previousBackground = boardCanvas.backgroundColor;
    boardCanvas.discardActiveObject();
    boardCanvas.setViewportTransform([1, 0, 0, 1, 0, 0]);
    if (backgroundColor) boardCanvas.backgroundColor = backgroundColor;
    try {
        const width = bounds.right - bounds.left + padding * 2;
        const height = bounds.bottom - bounds.top + padding * 2;
        const pixelLimitMultiplier = Math.sqrt(25_000_000 / Math.max(width * height, 1));
        return boardCanvas.toDataURL({
            format,
            quality: 0.92,
            multiplier: Math.max(0.25, Math.min(requestedMultiplier, pixelLimitMultiplier)),
            left: bounds.left - padding,
            top: bounds.top - padding,
            width,
            height,
        });
    } finally {
        boardCanvas.backgroundColor = previousBackground;
        boardCanvas.setViewportTransform(viewport);
        boardCanvas.requestRenderAll();
    }
}

async function exportBoardPNG() {
    const dataURL = renderWholeBoard('png', 2);
    if (!dataURL) {
        showNotification('La lavagna è vuota', 'info');
        return;
    }
    const name = `${safeExportFileName(currentProject?.name)}-lavagna.png`;
    if (currentProject?.preview) {
        const link = document.createElement('a');
        link.href = dataURL;
        link.download = name;
        link.click();
        showNotification('Anteprima PNG generata', 'success');
        return;
    }
    try {
        const path = await SaveBoardPNG(dataURL, name);
        if (path) showNotification('Lavagna esportata in PNG', 'success');
    } catch (error) {
        showNotification(`Impossibile esportare il PNG: ${error}`, 'error');
    }
}

async function exportBoardPDF() {
    if (currentProject?.preview) {
        showNotification('L’esportazione PDF è disponibile nell’app installata', 'info');
        return;
    }
    const dataURL = renderWholeBoard('jpeg', 1.5, '#1C1815');
    if (!dataURL) {
        showNotification('La lavagna è vuota', 'info');
        return;
    }
    try {
        const name = `${safeExportFileName(currentProject?.name)}-lavagna.pdf`;
        const path = await SaveBoardPDF(dataURL, name);
        if (path) showNotification('Lavagna esportata in PDF', 'success');
    } catch (error) {
        showNotification(`Impossibile esportare il PDF: ${error}`, 'error');
    }
}

function selectAllObjects() {
    const objects = boardCanvas.getObjects().filter(object => object.boardKind !== 'connector' && !isConnectorHelper(object));
    if (!objects.length) return;
    boardCanvas.discardActiveObject();
    if (objects.length === 1) boardCanvas.setActiveObject(objects[0]);
    else boardCanvas.setActiveObject(new ActiveSelection(objects, { canvas: boardCanvas }));
    boardCanvas.requestRenderAll();
}

function hideContextMenu() {
    const menu = document.getElementById('boardContextMenu');
    if (menu) menu.hidden = true;
}

function showContextMenu(x, y, target) {
    if (target) boardCanvas.setActiveObject(target);
    else boardCanvas.discardActiveObject();
    const menu = document.getElementById('boardContextMenu');
    const wrapper = document.getElementById('projectBoardCanvasWrap');
    menu.hidden = false;
    const left = Math.min(x, Math.max(wrapper.clientWidth - menu.offsetWidth - 8, 8));
    const top = Math.min(y, Math.max(wrapper.clientHeight - menu.offsetHeight - 8, 8));
    menu.style.left = `${Math.max(left, 8)}px`;
    menu.style.top = `${Math.max(top, 8)}px`;
    menu.querySelector('button')?.focus();
    boardCanvas.requestRenderAll();
}

function handleContextAction(action) {
    hideContextMenu();
    const actions = {
        duplicate: duplicateSelection,
        lock: toggleSelectionLock,
        forward: () => changeSelectionLayer('forward'),
        backward: () => changeSelectionLayer('backward'),
        delete: deleteSelection,
    };
    if (actions[action]) Promise.resolve(actions[action]()).catch(error => console.error('Errore menu lavagna:', error));
}

function createTableGroup(value, options = {}) {
    const tableData = normalizeTableData(value);
    const tableCellStyles = resizeTableCellStyles(options.tableCellStyles, tableData.length, tableData[0].length);
    const cellWidth = 180;
    const cellHeight = 52;
    const cells = [];
    tableData.forEach((row, rowIndex) => {
        row.forEach((cell, columnIndex) => {
            const style = normalizeTableCellStyle(tableCellStyles[rowIndex][columnIndex], rowIndex);
            cells.push(new Rect({
                left: columnIndex * cellWidth,
                top: rowIndex * cellHeight,
                width: cellWidth,
                height: cellHeight,
                fill: style.fill,
                stroke: style.borderColor,
                strokeWidth: style.borderWidth,
                originX: 'left',
                originY: 'top',
                selectable: false,
                evented: false,
                boardKind: 'table-cell-background',
            }));
            const originY = { top: 'top', middle: 'center', bottom: 'bottom' }[style.verticalAlign];
            const textTop = rowIndex * cellHeight + ({ top: 7, middle: cellHeight / 2, bottom: cellHeight - 7 }[style.verticalAlign]);
            cells.push(new Textbox(cell, {
                left: columnIndex * cellWidth + 10,
                top: textTop,
                width: cellWidth - 20,
                fill: style.textColor,
                fontFamily: style.fontFamily,
                fontSize: style.fontSize,
                fontWeight: style.fontWeight,
                fontStyle: style.fontStyle,
                underline: style.underline,
                textAlign: style.horizontalAlign,
                originX: 'left',
                originY,
                selectable: false,
                evented: false,
                boardKind: 'table-cell-text',
            }));
        });
    });
    return new Group(cells, {
        ...options,
        originX: 'center',
        originY: 'center',
        hoverCursor: 'pointer',
        boardKind: 'table',
        tableData,
        tableCellStyles,
    });
}

function tableCellKey(rowIndex, columnIndex) {
    return `${rowIndex}:${columnIndex}`;
}

function updateTableFormatToolbar() {
    const firstKey = [...tableSelectedCells][0] || '0:0';
    const [rowIndex, columnIndex] = firstKey.split(':').map(Number);
    const style = normalizeTableCellStyle(tableDraftStyles[rowIndex]?.[columnIndex], rowIndex);
    document.getElementById('boardTableFill').value = /^#[0-9a-f]{6}$/i.test(style.fill) ? style.fill : '#252a34';
    document.getElementById('boardTableTextColor').value = /^#[0-9a-f]{6}$/i.test(style.textColor) ? style.textColor : '#f3f4f6';
    document.getElementById('boardTableFontFamily').value = style.fontFamily;
    document.getElementById('boardTableFontSize').value = String(style.fontSize);
    document.querySelectorAll('[data-board-table-format]').forEach(button => {
        const action = button.dataset.boardTableFormat;
        const active = (action === 'bold' && style.fontWeight === 'bold')
            || (action === 'italic' && style.fontStyle === 'italic')
            || (action === 'underline' && style.underline)
            || (action === `align-${style.horizontalAlign}`)
            || (action === `align-${style.verticalAlign}`);
        button.classList.toggle('active', active);
        button.setAttribute('aria-pressed', String(active));
    });
}

function updateTableCellSelectionClasses() {
    document.querySelectorAll('#boardTableEditor td').forEach(cell => {
        cell.classList.toggle('selected', tableSelectedCells.has(tableCellKey(Number(cell.dataset.row), Number(cell.dataset.column))));
    });
    updateTableFormatToolbar();
}

function selectTableCell(rowIndex, columnIndex, event) {
    const key = tableCellKey(rowIndex, columnIndex);
    if (event.shiftKey && tableSelectionAnchor) {
        const [anchorRow, anchorColumn] = tableSelectionAnchor.split(':').map(Number);
        tableSelectedCells = new Set();
        for (let row = Math.min(anchorRow, rowIndex); row <= Math.max(anchorRow, rowIndex); row += 1) {
            for (let column = Math.min(anchorColumn, columnIndex); column <= Math.max(anchorColumn, columnIndex); column += 1) {
                tableSelectedCells.add(tableCellKey(row, column));
            }
        }
    } else if (event.ctrlKey || event.metaKey) {
        if (tableSelectedCells.has(key) && tableSelectedCells.size > 1) tableSelectedCells.delete(key);
        else tableSelectedCells.add(key);
        tableSelectionAnchor = key;
    } else {
        tableSelectedCells = new Set([key]);
        tableSelectionAnchor = key;
    }
    updateTableCellSelectionClasses();
}

function applyTableStylePatch(patch) {
    if (!tableSelectedCells.size) tableSelectedCells.add('0:0');
    tableDraftStyles = patchSelectedTableCellStyles(tableDraftStyles, tableSelectedCells, patch);
    renderTableEditor();
}

function handleTableFormatAction(action) {
    const firstKey = [...tableSelectedCells][0] || '0:0';
    const [rowIndex, columnIndex] = firstKey.split(':').map(Number);
    const current = normalizeTableCellStyle(tableDraftStyles[rowIndex]?.[columnIndex], rowIndex);
    const patches = {
        bold: { fontWeight: current.fontWeight === 'bold' ? 'normal' : 'bold' },
        italic: { fontStyle: current.fontStyle === 'italic' ? 'normal' : 'italic' },
        underline: { underline: !current.underline },
        'align-left': { horizontalAlign: 'left' },
        'align-center': { horizontalAlign: 'center' },
        'align-right': { horizontalAlign: 'right' },
        'align-top': { verticalAlign: 'top' },
        'align-middle': { verticalAlign: 'middle' },
        'align-bottom': { verticalAlign: 'bottom' },
    };
    if (patches[action]) applyTableStylePatch(patches[action]);
}

function renderTableEditor() {
    const container = document.getElementById('boardTableEditor');
    container.replaceChildren();
    const table = document.createElement('table');
    const body = document.createElement('tbody');
    tableDraft.forEach((row, rowIndex) => {
        const rowElement = document.createElement('tr');
        row.forEach((cell, columnIndex) => {
            const cellElement = document.createElement('td');
            const key = tableCellKey(rowIndex, columnIndex);
            const style = normalizeTableCellStyle(tableDraftStyles[rowIndex]?.[columnIndex], rowIndex);
            cellElement.dataset.row = rowIndex;
            cellElement.dataset.column = columnIndex;
            cellElement.classList.toggle('selected', tableSelectedCells.has(key));
            const input = document.createElement('textarea');
            input.value = cell;
            input.maxLength = TABLE_MAX_CELL_LENGTH;
            input.dataset.row = rowIndex;
            input.dataset.column = columnIndex;
            input.setAttribute('aria-label', `Riga ${rowIndex + 1}, colonna ${columnIndex + 1}`);
            input.wrap = style.wrap ? 'soft' : 'off';
            input.style.backgroundColor = style.fill;
            input.style.color = style.textColor;
            input.style.fontFamily = style.fontFamily;
            input.style.fontSize = `${style.fontSize}px`;
            input.style.fontWeight = style.fontWeight;
            input.style.fontStyle = style.fontStyle;
            input.style.textDecoration = style.underline ? 'underline' : 'none';
            input.style.textAlign = style.horizontalAlign;
            input.style.paddingTop = { top: '6px', middle: '16px', bottom: '28px' }[style.verticalAlign];
            cellElement.addEventListener('mousedown', event => {
                selectTableCell(rowIndex, columnIndex, event);
                // Un click reale dell'utente su una cella non porta il focus sulla
                // sua textarea (verificato: la cella risulta selezionata ma
                // document.activeElement resta <body>, quindi digitare non scrive
                // nulla) — probabilmente il canvas fabric.js sottostante intercetta
                // il mousedown. Il focus esplicito qui ripristina il comportamento
                // atteso senza dover disattivare gli handler del canvas.
                if (document.activeElement !== input) input.focus();
            });
            input.addEventListener('focus', event => {
                if (!tableSelectedCells.has(key)) selectTableCell(rowIndex, columnIndex, event);
            });
            input.addEventListener('input', () => {
                tableDraft[rowIndex][columnIndex] = input.value;
            });
            cellElement.appendChild(input);
            rowElement.appendChild(cellElement);
        });
        body.appendChild(rowElement);
    });
    table.appendChild(body);
    container.appendChild(table);
    const rows = tableDraft.length;
    const columns = tableDraft[0].length;
    document.getElementById('boardTableRows').value = String(rows);
    document.getElementById('boardTableColumns').value = String(columns);
    document.querySelector('[data-board-table-action="remove-row"]').disabled = rows <= 1;
    document.querySelector('[data-board-table-action="remove-column"]').disabled = columns <= 1;
    document.querySelector('[data-board-table-action="add-row"]').disabled = rows >= TABLE_MAX_ROWS;
    document.querySelector('[data-board-table-action="add-column"]').disabled = columns >= TABLE_MAX_COLUMNS;
    updateTableFormatToolbar();
}

function openTableEditor(table = null) {
    editingTable = table?.boardKind === 'table' ? table : null;
    if (editingTable) syncTableStyleMetadata(editingTable);
    tableDraft = normalizeTableData(editingTable?.tableData || createDefaultTableData());
    tableDraftStyles = resizeTableCellStyles(editingTable?.tableCellStyles, tableDraft.length, tableDraft[0].length);
    tableSelectedCells = new Set(['0:0']);
    tableSelectionAnchor = '0:0';
    document.getElementById('boardTableDialog').hidden = false;
    renderTableEditor();
    requestAnimationFrame(() => document.querySelector('#boardTableEditor textarea')?.focus());
}

function closeTableEditor() {
    document.getElementById('boardTableDialog').hidden = true;
    editingTable = null;
    tableDraft = [];
    tableDraftStyles = [];
    tableSelectedCells = new Set();
    tableSelectionAnchor = null;
}

function applyTableEditor() {
    if (!tableDraft.length) return;
    if (!editingTable) {
        const center = viewportCenter();
        addAndSelect(createTableGroup(tableDraft, { left: center.x, top: center.y, tableCellStyles: tableDraftStyles }));
        closeTableEditor();
        return;
    }
    const previous = editingTable;
    const replacement = createTableGroup(tableDraft, {
        left: previous.left,
        top: previous.top,
        angle: previous.angle,
        scaleX: previous.scaleX,
        scaleY: previous.scaleY,
        flipX: previous.flipX,
        flipY: previous.flipY,
        boardObjectId: previous.boardObjectId,
        boardName: previous.boardName,
        boardLocked: previous.boardLocked,
        tableCellStyles: tableDraftStyles,
    });
    replaceObjectInPlace(previous, replacement, { activate: true });
    boardCanvas.requestRenderAll();
    closeTableEditor();
    captureHistory();
}

function handleTableAction(action) {
    if (action === 'cancel') {
        closeTableEditor();
        return;
    }
    if (action === 'confirm') {
        applyTableEditor();
        return;
    }
    const operations = {
        'add-row': addTableRow,
        'remove-row': removeTableRow,
        'add-column': addTableColumn,
        'remove-column': removeTableColumn,
    };
    if (operations[action]) {
        tableDraft = operations[action](tableDraft);
        tableDraftStyles = resizeTableCellStyles(tableDraftStyles, tableDraft.length, tableDraft[0].length);
        tableSelectedCells = new Set([...tableSelectedCells].filter(key => {
            const [row, column] = key.split(':').map(Number);
            return row < tableDraft.length && column < tableDraft[0].length;
        }));
        if (!tableSelectedCells.size) tableSelectedCells.add('0:0');
        renderTableEditor();
    }
}

async function addImage() {
    if (currentProject?.preview) {
        showNotification('La selezione immagini è disponibile nell’app installata', 'info');
        return;
    }
    try {
        const asset = await ImportBoardImage(currentProject.id);
        if (!asset) return;
        const image = await FabricImage.fromURL(asset.data_url);
        const center = viewportCenter();
        const maximumWidth = 720;
        const maximumHeight = 520;
        const scale = Math.min(
            maximumWidth / Math.max(image.width, 1),
            maximumHeight / Math.max(image.height, 1),
            1,
        );
        image.set({
            left: center.x,
            top: center.y,
            originX: 'center',
            originY: 'center',
            scaleX: scale,
            scaleY: scale,
            boardKind: 'image',
            assetId: asset.id,
            assetName: asset.file_name,
        });
        addAndSelect(image);
    } catch (error) {
        console.error('Errore inserimento immagine:', error);
        showNotification(`Impossibile inserire l’immagine: ${error}`, 'error');
    }
}

function formatFileSize(bytes) {
    const value = Number(bytes) || 0;
    if (value < 1024) return `${value} B`;
    if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`;
    if (value < 1024 * 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MiB`;
    return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GiB`;
}

function addVideoCard(video) {
    const center = viewportCenter();
    const background = new Rect({
        left: 0,
        top: 0,
        width: 410,
        height: 126,
        rx: 14,
        ry: 14,
        fill: '#252a34',
        stroke: '#db2777',
        strokeWidth: 3,
        originX: 'left',
        originY: 'top',
    });
    const play = new Circle({
        left: 22,
        top: 36,
        radius: 25,
        fill: '#db2777',
        originX: 'left',
        originY: 'top',
        selectable: false,
        evented: false,
    });
    const icon = new IText('▶', {
        left: 39,
        top: 48,
        fill: '#ffffff',
        fontFamily: 'Arial',
        fontSize: 22,
        originX: 'left',
        originY: 'top',
        selectable: false,
        evented: false,
    });
    const name = new Textbox(video.file_name, {
        left: 92,
        top: 20,
        width: 290,
        fill: '#fdf2f8',
        fontFamily: 'Arial',
        fontSize: 20,
        fontWeight: 'bold',
        originX: 'left',
        originY: 'top',
        selectable: false,
        evented: false,
    });
    const path = new Textbox(video.path, {
        left: 92,
        top: 58,
        width: 290,
        fill: '#f9a8d4',
        fontFamily: 'Arial',
        fontSize: 12,
        originX: 'left',
        originY: 'top',
        selectable: false,
        evented: false,
    });
    const size = new IText(formatFileSize(video.size), {
        left: 92,
        top: 96,
        fill: '#9ca3af',
        fontFamily: 'Arial',
        fontSize: 12,
        originX: 'left',
        originY: 'top',
        selectable: false,
        evented: false,
    });
    addAndSelect(new Group([background, play, icon, name, path, size], {
        left: center.x,
        top: center.y,
        originX: 'center',
        originY: 'center',
        hoverCursor: 'pointer',
        boardKind: 'video',
        videoPath: video.path,
        videoName: video.file_name,
        videoSize: video.size,
    }));
}

async function addVideoLink() {
    if (currentProject?.preview) {
        showNotification('La selezione video è disponibile nell’app installata', 'info');
        return;
    }
    try {
        const video = await SelectBoardVideo();
        if (video) addVideoCard(video);
    } catch (error) {
        showNotification(`Impossibile collegare il video: ${error}`, 'error');
    }
}

async function openVideoObject(object) {
    try {
        await OpenBoardVideo(object.videoPath);
    } catch (error) {
        showNotification(`Impossibile aprire il video: ${error}`, 'error');
    }
}

function defaultLinkLabel(linkType, target) {
    if (linkType === 'web') {
        try {
            return new URL(target).hostname;
        } catch {
            return 'Link Internet';
        }
    }
    const parts = target.replace(/[\\/]+$/, '').split(/[\\/]/).filter(Boolean);
    return parts.at(-1) || 'Cartella';
}

function addLinkCard(linkType, target, requestedLabel = '') {
    const center = viewportCenter();
    const label = requestedLabel.trim() || defaultLinkLabel(linkType, target);
    const icon = linkType === 'web' ? '↗' : '⌂';
    const color = linkType === 'web' ? '#2563eb' : '#d97706';
    const background = new Rect({
        left: 0,
        top: 0,
        width: 360,
        height: 112,
        rx: 14,
        ry: 14,
        fill: '#252a34',
        stroke: color,
        strokeWidth: 3,
        originX: 'left',
        originY: 'top',
    });
    const iconText = new IText(icon, {
        left: 20,
        top: 25,
        fill: color,
        fontFamily: 'Arial',
        fontSize: 34,
        originX: 'left',
        originY: 'top',
        selectable: false,
    });
    const labelText = new Textbox(label, {
        left: 72,
        top: 19,
        width: 265,
        fill: '#f8fafc',
        fontFamily: 'Arial',
        fontSize: 21,
        fontWeight: 'bold',
        originX: 'left',
        originY: 'top',
        selectable: false,
    });
    const targetText = new Textbox(target, {
        left: 72,
        top: 58,
        width: 265,
        fill: '#a5b4fc',
        fontFamily: 'Arial',
        fontSize: 13,
        originX: 'left',
        originY: 'top',
        selectable: false,
    });
    addAndSelect(new Group([background, iconText, labelText, targetText], {
        left: center.x,
        top: center.y,
        originX: 'center',
        originY: 'center',
        hoverCursor: 'pointer',
        boardKind: 'link',
        linkType,
        linkTarget: target,
        linkLabel: label,
    }));
}

async function openLinkObject(object = boardCanvas?.getActiveObject()) {
    if (object?.boardKind === 'video') {
        await openVideoObject(object);
        return;
    }
    if (!object || object.boardKind !== 'link') {
        showNotification('Seleziona prima un collegamento o un video', 'info');
        return;
    }
    try {
        await OpenBoardLink(object.linkType, object.linkTarget);
    } catch (error) {
        console.error('Errore apertura collegamento:', error);
        showNotification(`Impossibile aprire il collegamento: ${error}`, 'error');
    }
}

function closeLinkDialog() {
    const dialog = document.getElementById('boardLinkDialog');
    dialog.hidden = true;
    pendingLinkType = null;
    document.getElementById('boardLinkTarget').value = '';
    document.getElementById('boardLinkLabel').value = '';
}

function showLinkDialog(linkType) {
    pendingLinkType = linkType;
    const dialog = document.getElementById('boardLinkDialog');
    const target = document.getElementById('boardLinkTarget');
    document.getElementById('boardLinkDialogTitle').textContent = linkType === 'web'
        ? 'Aggiungi link Internet'
        : 'Collega cartella su server o NAS';
    target.placeholder = linkType === 'web'
        ? 'https://esempio.it/pagina'
        : '\\\\server oppure \\\\server\\condivisione\\cartella';
    target.type = linkType === 'web' ? 'url' : 'text';
    dialog.hidden = false;
    target.focus();
}

async function confirmLinkDialog() {
    if (!pendingLinkType) return;
    const targetInput = document.getElementById('boardLinkTarget');
    const label = document.getElementById('boardLinkLabel').value.slice(0, 120);
    try {
        let target;
        if (currentProject?.preview) {
            target = targetInput.value.trim();
            if (pendingLinkType === 'web') {
                if (!target.includes('://')) target = `https://${target}`;
                const parsed = new URL(target);
                if (!['http:', 'https:'].includes(parsed.protocol) || parsed.username || parsed.password) throw new Error('URL HTTP/HTTPS non valido');
                target = parsed.toString();
            } else if (!target.startsWith('\\\\')) {
                throw new Error('percorso UNC non valido');
            }
        } else {
            target = await ValidateBoardLink(pendingLinkType, targetInput.value);
        }
        addLinkCard(pendingLinkType, target, label);
        closeLinkDialog();
    } catch (error) {
        showNotification(`Collegamento non valido: ${error}`, 'error');
        targetInput.focus();
    }
}

async function addLocalFolderLink() {
    try {
        const target = await SelectBoardFolder();
        if (!target) return;
        addLinkCard(target.startsWith('\\\\') ? 'network' : 'folder', target);
    } catch (error) {
        showNotification(`Impossibile selezionare la cartella: ${error}`, 'error');
    }
}

function deleteSelection() {
    const active = boardCanvas?.getActiveObjects() || [];
    if (!active.length) return;
    active.filter(object => object.boardKind !== 'connector' && !isConnectorHelper(object)).forEach(object => {
        const serialized = object.toObject(CUSTOM_OBJECT_PROPERTIES);
        walkBoardObjects([serialized], child => {
            if (child.boardKind === 'image') {
                delete child.src;
                delete child.crossOrigin;
            }
        });
        boardTrash.push({
            id: `trash-${Date.now()}-${Math.random().toString(16).slice(2)}`,
            name: objectDisplayName(object),
            deletedAt: new Date().toISOString(),
            object: serialized,
        });
    });
    if (boardTrash.length > 100) boardTrash = boardTrash.slice(-100);
    restoringHistory = true;
    try {
        boardCanvas.discardActiveObject();
        active.forEach(object => boardCanvas.remove(object));
        refreshConnectors();
    } finally {
        restoringHistory = false;
    }
    boardCanvas.requestRenderAll();
    captureHistory();
}

function fitContent() {
    const objects = boardCanvas.getObjects().filter(object => !isConnectorHelper(object));
    if (!objects.length) {
        boardCanvas.setViewportTransform([1, 0, 0, 1, 0, 0]);
    } else {
        const bounds = objects.reduce((acc, object) => {
            const rect = object.getBoundingRect();
            return {
                left: Math.min(acc.left, rect.left),
                top: Math.min(acc.top, rect.top),
                right: Math.max(acc.right, rect.left + rect.width),
                bottom: Math.max(acc.bottom, rect.top + rect.height),
            };
        }, { left: Infinity, top: Infinity, right: -Infinity, bottom: -Infinity });
        const width = Math.max(bounds.right - bounds.left, 1);
        const height = Math.max(bounds.bottom - bounds.top, 1);
        const zoom = Math.min(boardCanvas.width / (width + 160), boardCanvas.height / (height + 160), 2);
        boardCanvas.setViewportTransform([
            zoom, 0, 0, zoom,
            boardCanvas.width / 2 - ((bounds.left + bounds.right) / 2) * zoom,
            boardCanvas.height / 2 - ((bounds.top + bounds.bottom) / 2) * zoom,
        ]);
    }
    boardCanvas.requestRenderAll();
    scheduleSave();
}

function changeZoom(delta) {
    if (!boardCanvas) return;
    setZoom(Math.round((boardCanvas.getZoom() + delta) * 10) / 10);
}

function showHomeGroup(selector) {
    setBoardRibbonTab('home', true);
    requestAnimationFrame(() => document.querySelector(selector)?.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'nearest' }));
}

function resizeCanvas() {
    if (!boardCanvas) return;
    const wrapper = document.getElementById('projectBoardCanvasWrap');
    boardCanvas.setDimensions({ width: wrapper.clientWidth, height: wrapper.clientHeight });
    boardCanvas.requestRenderAll();
}

function installCanvasEvents() {
    boardCanvas.upperCanvasEl.addEventListener('contextmenu', event => event.preventDefault());
    boardCanvas.on('object:added', event => { if (!isConnectorHelper(event.target)) captureHistory(); });
    boardCanvas.on('object:modified', event => {
        if (isConnectorHelper(event.target)) return;
        if (normalizeStickyNoteScale(event.target)) refreshConnectors();
        stabilizeTextShapeLabel(event.target);
        captureHistory();
    });
    boardCanvas.on('object:removed', event => { if (!isConnectorHelper(event.target)) captureHistory(); });
    boardCanvas.on('text:changed', event => {
        if (event.target?.boardKind === 'note') {
            const center = event.target.getCenterPoint();
            stabilizeStickyNoteDimensions(event.target);
            event.target.setPositionByOrigin(center, 'center', 'center');
            refreshConnectors();
            boardCanvas.requestRenderAll();
        }
        scheduleSave();
    });
    boardCanvas.on('text:editing:exited', captureHistory);
    boardCanvas.on('path:created', event => {
        event.path?.set('boardKind', 'drawing');
        captureHistory();
    });
    boardCanvas.on('object:moving', event => {
        if (event.target?.boardKind === 'connector-handle') {
            moveConnectorHandle(event.target);
            return;
        }
        if (snappingEnabled && event.target?.boardKind !== 'connector') {
            event.target.set({
                left: Math.round((event.target.left || 0) / 20) * 20,
                top: Math.round((event.target.top || 0) / 20) * 20,
            });
            event.target.setCoords();
        }
        refreshConnectors();
    });
    boardCanvas.on('object:scaling', event => {
        stabilizeTextShapeLabel(event.target);
        refreshConnectors();
    });
    boardCanvas.on('object:rotating', refreshConnectors);
    boardCanvas.on('selection:created', () => { updateRibbonState(); handleConnectorSelection(); });
    boardCanvas.on('selection:updated', () => { updateRibbonState(); handleConnectorSelection(); });
    boardCanvas.on('selection:cleared', () => { updateRibbonState(); handleConnectorSelection(); });
    boardCanvas.on('after:render', () => {
        updateStatusBar();
        renderMinimap();
    });

    boardCanvas.on('mouse:wheel', event => {
        const original = event.e;
        let zoom = boardCanvas.getZoom() * Math.pow(0.999, original.deltaY);
        zoom = Math.max(0.08, Math.min(zoom, 8));
        boardCanvas.zoomToPoint(new Point(original.offsetX, original.offsetY), zoom);
        original.preventDefault();
        original.stopPropagation();
        scheduleSave();
    });

    boardCanvas.on('mouse:down', event => {
        const original = event.e;
        if (event.target?.boardKind === 'connector-handle') {
            draggingConnectorHandle = event.target;
            return;
        }
        if (original.button === 2) {
            showContextMenu(original.offsetX, original.offsetY, event.target);
            original.preventDefault();
            return;
        }
        if (drawingEraserEnabled) {
            if (event.target?.boardKind === 'drawing') {
                boardCanvas.remove(event.target);
                boardCanvas.requestRenderAll();
            } else showNotification('La gomma elimina soltanto i tratti creati con la Matita.', 'info');
            original.preventDefault();
            return;
        }
        if (connectorCreationMode && original.button === 0 && chooseConnectorTarget(event.target)) {
            original.preventDefault();
            return;
        }
        hideContextMenu();
        if (original.button !== 1 && !spacePressed) return;
        isPanning = true;
        lastPointer = { x: original.clientX, y: original.clientY };
        boardCanvas.selection = false;
        boardCanvas.defaultCursor = 'grabbing';
        original.preventDefault();
    });

    boardCanvas.on('mouse:move', event => {
        if (!isPanning) return;
        const original = event.e;
        const transform = boardCanvas.viewportTransform;
        transform[4] += original.clientX - lastPointer.x;
        transform[5] += original.clientY - lastPointer.y;
        lastPointer = { x: original.clientX, y: original.clientY };
        boardCanvas.requestRenderAll();
    });

    boardCanvas.on('mouse:up', () => {
        const connectorID = draggingConnectorHandle?.connectorOwnerId || pendingConnectorSelectionID;
        draggingConnectorHandle = null;
        if (connectorID) {
            window.setTimeout(() => finishConnectorHandle(connectorID), 30);
        }
        if (!isPanning) return;
        isPanning = false;
        boardCanvas.selection = true;
        boardCanvas.defaultCursor = 'default';
        scheduleSave();
    });

    boardCanvas.on('mouse:dblclick', event => {
        if (event.target?.boardKind === 'link') openLinkObject(event.target);
        if (event.target?.boardKind === 'video') openVideoObject(event.target);
        if (event.target?.boardKind === 'table') openTableEditor(event.target);
        if (event.target?.boardKind === 'connector' || ['rectangle', 'circle', 'shape'].includes(event.target?.boardKind)) beginInlineTextEdit(event.target);
    });
}

function restoreViewport(viewportJSON) {
    try {
        const viewport = JSON.parse(viewportJSON || '{}');
        if (Array.isArray(viewport.viewportTransform) && viewport.viewportTransform.length === 6) {
            boardCanvas.setViewportTransform(viewport.viewportTransform.map(Number));
        }
    } catch (error) {
        console.warn('Visuale lavagna non ripristinabile:', error);
    }
}

async function importLegacyNote(noteText) {
    if (!noteText.trim()) return;
    addAndSelect(new Textbox(noteText, {
        left: 80,
        top: 80,
        width: 520,
        padding: 22,
        fill: '#1f2937',
        backgroundColor: '#fef3c7',
        fontFamily: 'Arial',
        fontSize: 20,
        originX: 'left',
        originY: 'top',
        boardKind: 'legacy-note',
    }));
    showNotification('La nota Markdown è stata copiata nella lavagna e conservata come originale', 'info');
}

async function initializeCanvas(board, noteText) {
    if (boardCanvas) await boardCanvas.dispose();
    connectorCreationMode = false;
    connectorCreationFrom = null;
    drawingEnabled = false;
    drawingEraserEnabled = false;
    connectorHelpers = [];
    draggingConnectorHandle = null;
    pendingConnectorSelectionID = null;
    boardCanvas = new Canvas('projectBoardCanvas', {
        preserveObjectStacking: true,
        selectionColor: 'rgba(201, 154, 84, 0.12)',
        selectionBorderColor: '#C99A54',
        targetFindTolerance: 8,
    });
    resizeCanvas();
    installCanvasEvents();

    restoringHistory = true;
    try {
        const document = JSON.parse(board.document_json || '{}');
        boardLoadMigrationNeeded = false;
        boardTrash = Array.isArray(document.prendiTempo?.trash) ? document.prendiTempo.trash.slice(-100) : [];
        if (Array.isArray(document.objects)) {
            await boardCanvas.loadFromJSON(await hydrateBoardDocument(document, true), reviveBoardObject);
            stabilizeAllStickyNotes();
            stabilizeAllTextShapes();
            refreshConnectors();
        }
        restoreViewport(board.viewport_json);
        if (!board.legacy_note_imported) await importLegacyNote(noteText || '');
    } finally {
        restoringHistory = false;
    }
    boardCanvas.requestRenderAll();
    undoStack = [canvasDocument()];
    redoStack = [];
    updateRibbonState();
    if (!board.legacy_note_imported || boardLoadMigrationNeeded) await saveBoard();
    else setSaveStatus('Salvato', 'saved');
}

function boardIsOpen() {
    return document.getElementById('projectBoardModal')?.classList.contains('show');
}

function activeTextEditing() {
    const active = boardCanvas?.getActiveObject();
    return Boolean(active?.isEditing);
}

function handleKeyDown(event) {
    if (!boardIsOpen()) return;
    if (event.target instanceof HTMLElement && event.target.closest('input, textarea, select, [contenteditable="true"]')) return;
    if (!document.getElementById('boardLinkDialog').hidden || !document.getElementById('boardTableDialog').hidden) return;
    if (!document.getElementById('boardContextMenu').hidden) {
        if (event.key === 'Escape') hideContextMenu();
        return;
    }
    if (event.code === 'Space' && !activeTextEditing()) {
        spacePressed = true;
        if (drawingEnabled) {
            drawingPausedForPan = true;
            boardCanvas.isDrawingMode = false;
        }
        event.preventDefault();
        return;
    }
    if (activeTextEditing()) return;

    const activeObject = boardCanvas?.getActiveObject();
    if (!event.ctrlKey && !event.metaKey && !event.altKey && event.key.length === 1
        && (activeObject?.boardKind === 'connector' || ['rectangle', 'circle', 'shape'].includes(activeObject?.boardKind))) {
        event.preventDefault();
        beginInlineTextEdit(activeObject, event.key);
        return;
    }

    if (event.altKey && !event.ctrlKey && !event.metaKey) {
        const tabName = { h: 'home', i: 'insert', d: 'draw', v: 'view', e: 'export' }[event.key.toLowerCase()];
        if (tabName) {
            event.preventDefault();
            setBoardRibbonTab(tabName, true);
            return;
        }
    }

    const modifier = event.ctrlKey || event.metaKey;
    if (modifier && event.key.toLowerCase() === 'c') {
        event.preventDefault();
        copySelection();
    } else if (modifier && event.key.toLowerCase() === 'x') {
        event.preventDefault();
        cutSelection();
    } else if (modifier && event.key.toLowerCase() === 'v') {
        event.preventDefault();
        pasteClipboard();
    } else if (modifier && event.key.toLowerCase() === 'd') {
        event.preventDefault();
        duplicateSelection();
    } else if (modifier && event.shiftKey && event.key.toLowerCase() === 'g') {
        event.preventDefault();
        ungroupSelection();
    } else if (modifier && event.key.toLowerCase() === 'g') {
        event.preventDefault();
        groupSelection();
    } else if (modifier && event.key.toLowerCase() === 'a') {
        event.preventDefault();
        selectAllObjects();
    } else if (modifier && event.key.toLowerCase() === 'z') {
        event.preventDefault();
        event.shiftKey ? redo() : undo();
    } else if (modifier && event.key.toLowerCase() === 'y') {
        event.preventDefault();
        redo();
    } else if (modifier && event.key.toLowerCase() === 's') {
        event.preventDefault();
        saveBoard();
    } else if (modifier && event.key === '0') {
        event.preventDefault();
        fitContent();
    } else if (modifier && (event.key === '+' || event.key === '=')) {
        event.preventDefault();
        changeZoom(0.1);
    } else if (modifier && event.key === '-') {
        event.preventDefault();
        changeZoom(-0.1);
    } else if (event.key === 'Delete' || event.key === 'Backspace') {
        event.preventDefault();
        deleteSelection();
    } else if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(event.key)) {
        const distance = event.shiftKey ? 10 : 1;
        const delta = {
            ArrowUp: [0, -distance],
            ArrowDown: [0, distance],
            ArrowLeft: [-distance, 0],
            ArrowRight: [distance, 0],
        }[event.key];
        const objects = boardCanvas.getActiveObjects();
        if (!objects.length) return;
        event.preventDefault();
        objects.forEach(object => object.set({ left: object.left + delta[0], top: object.top + delta[1] }).setCoords());
        refreshConnectors();
        boardCanvas.requestRenderAll();
        captureHistory();
    } else if (event.key === 'Escape') {
        closeColorGallery();
        if (drawingEnabled) setDrawingMode(false);
        if (drawingEraserEnabled) setDrawingEraserMode(false);
        if (connectorCreationMode) setConnectorCreationMode(false);
        boardCanvas.discardActiveObject();
        boardCanvas.requestRenderAll();
    }
}

function handleKeyUp(event) {
    if (event.code === 'Space') {
        spacePressed = false;
        if (drawingPausedForPan && drawingEnabled) {
            boardCanvas.isDrawingMode = true;
            boardCanvas.selection = false;
            boardCanvas.defaultCursor = 'crosshair';
        }
        drawingPausedForPan = false;
    }
}

export function initializeProjectBoard() {
    window.addEventListener('resize', resizeCanvas);
    document.addEventListener('keydown', handleKeyDown);
    document.addEventListener('keyup', handleKeyUp);

    document.querySelectorAll('[data-board-command]').forEach(button => {
        button.addEventListener('click', () => {
            const commands = {
                text: addText,
                note: addStickyNote,
                rectangle: addRectangle,
                circle: addCircle,
                triangle: addTriangle,
                line: addLine,
                frame: addFrame,
                draw: toggleDrawingMode,
                'erase-drawing': toggleDrawingEraserMode,
                arrow: addArrow,
                connect: addConnector,
                table: () => openTableEditor(),
                image: addImage,
                video: addVideoLink,
                folder: addLocalFolderLink,
                web: () => showLinkDialog('web'),
                'open-link': openLinkObject,
                'edit-object': editSelectedObject,
                shapes: () => {
                    const gallery = document.getElementById('boardShapeGallery');
                    document.getElementById('boardAlignGallery').hidden = true;
                    gallery.hidden = !gallery.hidden;
                },
                copy: copySelection,
                cut: cutSelection,
                paste: pasteClipboard,
                lock: toggleSelectionLock,
                group: groupSelection,
                ungroup: ungroupSelection,
                'align-x': () => alignSelection('center-x'),
                'align-y': () => alignSelection('center-y'),
                'align-menu': () => {
                    const gallery = document.getElementById('boardAlignGallery');
                    document.getElementById('boardShapeGallery').hidden = true;
                    gallery.hidden = !gallery.hidden;
                },
                snap: toggleSnapping,
                'toggle-layers': toggleSidePanel,
                'toggle-minimap': toggleMinimap,
                'context-home-text': () => showHomeGroup('.board-ribbon-text-group'),
                'context-home-object': () => showHomeGroup('.board-ribbon-object-style-group'),
                duplicate: duplicateSelection,
                'select-all': selectAllObjects,
                forward: () => changeSelectionLayer('forward'),
                backward: () => changeSelectionLayer('backward'),
                delete: deleteSelection,
                undo,
                redo,
                'zoom-in': () => changeZoom(0.1),
                'zoom-out': () => changeZoom(-0.1),
                fit: fitContent,
                save: saveBoard,
                'export-png': exportBoardPNG,
                'export-pdf': exportBoardPDF,
            };
            const command = commands[button.dataset.boardCommand];
            if (!['draw', 'erase-drawing'].includes(button.dataset.boardCommand) && (drawingEnabled || drawingEraserEnabled)) {
                setDrawingMode(false);
                setDrawingEraserMode(false);
            }
            if (button.dataset.boardCommand !== 'connect' && connectorCreationMode) setConnectorCreationMode(false);
            if (command) Promise.resolve(command()).catch(error => console.error('Errore comando lavagna:', error));
        });
    });

    document.querySelectorAll('[data-board-tab]').forEach(tab => {
        tab.addEventListener('click', () => setBoardRibbonTab(tab.dataset.boardTab));
        tab.addEventListener('keydown', event => {
            if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
            const tabs = [...document.querySelectorAll('[data-board-tab]:not([hidden])')];
            const index = tabs.indexOf(tab);
            const nextIndex = event.key === 'Home' ? 0
                : event.key === 'End' ? tabs.length - 1
                    : (index + (event.key === 'ArrowRight' ? 1 : -1) + tabs.length) % tabs.length;
            event.preventDefault();
            setBoardRibbonTab(tabs[nextIndex].dataset.boardTab, true);
        });
    });
    document.querySelector('[data-board-ribbon-toggle]')?.addEventListener('click', toggleBoardRibbon);

    document.querySelectorAll('[data-board-shape]').forEach(button => {
        button.addEventListener('click', () => {
            const commands = { rectangle: addRectangle, circle: addCircle, triangle: addTriangle, line: addLine, arrow: addArrow };
            document.getElementById('boardShapeGallery').hidden = true;
            commands[button.dataset.boardShape]?.();
        });
    });
    document.querySelectorAll('[data-board-align]').forEach(button => {
        button.addEventListener('click', () => {
            document.getElementById('boardAlignGallery').hidden = true;
            alignSelection(button.dataset.boardAlign);
        });
    });
    document.addEventListener('click', event => {
        document.querySelectorAll('.board-shape-gallery').forEach(gallery => {
            const command = gallery.id === 'boardShapeGallery' ? 'shapes' : 'align-menu';
            if (!gallery.hidden && !gallery.contains(event.target) && !event.target.closest(`[data-board-command="${command}"]`)) gallery.hidden = true;
        });
        const colorGallery = document.getElementById('boardColorGallery');
        if (colorGallery && !colorGallery.hidden && !colorGallery.contains(event.target) && !event.target.closest('[data-board-color-target]')) closeColorGallery();
    });

    document.querySelectorAll('[data-board-side-tab]').forEach(tab => {
        tab.addEventListener('click', () => setBoardSideTab(tab.dataset.boardSideTab));
    });
    document.getElementById('boardObjectSearch')?.addEventListener('input', renderObjectPanel);
    document.querySelectorAll('[data-board-trash-action]').forEach(button => {
        button.addEventListener('click', () => {
            if (button.dataset.boardTrashAction === 'restore-all') restoreAllTrash();
            else emptyTrash();
        });
    });
    document.querySelector('[data-board-version-action="create"]')?.addEventListener('click', createNamedBoardVersion);
    document.getElementById('boardVersionName')?.addEventListener('keydown', event => {
        if (event.key === 'Enter') {
            event.preventDefault();
            createNamedBoardVersion();
        }
    });
    document.getElementById('boardZoomSlider')?.addEventListener('input', event => setZoom(Number(event.target.value) / 100));
    document.getElementById('boardMinimapCanvas')?.addEventListener('click', event => {
        if (!boardCanvas || !minimapMapping) return;
        const rect = event.currentTarget.getBoundingClientRect();
        const x = (event.clientX - rect.left) * (event.currentTarget.width / rect.width);
        const y = (event.clientY - rect.top) * (event.currentTarget.height / rect.height);
        const worldX = (x - minimapMapping.offsetX) / minimapMapping.scale;
        const worldY = (y - minimapMapping.offsetY) / minimapMapping.scale;
        const transform = boardCanvas.viewportTransform;
        transform[4] = boardCanvas.width / 2 - worldX * transform[0];
        transform[5] = boardCanvas.height / 2 - worldY * transform[3];
        boardCanvas.requestRenderAll();
        scheduleSave();
    });
    document.querySelectorAll('[data-board-color-target]').forEach(button => {
        button.addEventListener('click', event => {
            event.stopPropagation();
            openColorGallery(button.dataset.boardColorTarget, button);
        });
    });
    document.querySelectorAll('[data-board-color-value]').forEach(button => {
        button.addEventListener('click', () => {
            if (!activeColorTarget) return;
            const target = activeColorTarget;
            setBoardColorValue(target, button.dataset.boardColorValue);
            closeColorGallery();
            applyCurrentColor(target);
        });
    });
    document.getElementById('boardMoreColors')?.addEventListener('click', () => {
        const control = BOARD_COLOR_CONTROLS[activeColorTarget];
        if (control) document.getElementById(control.inputID)?.click();
    });
    Object.entries(BOARD_COLOR_CONTROLS).forEach(([target, control]) => {
        document.getElementById(control.inputID)?.addEventListener('input', event => setBoardColorValue(target, event.target.value));
        document.getElementById(control.inputID)?.addEventListener('change', () => applyCurrentColor(target));
    });
    document.querySelectorAll('[data-board-text-style]').forEach(button => {
        button.addEventListener('click', () => applyTextStyleCommand(button.dataset.boardTextStyle));
    });
    document.querySelectorAll('[data-board-list-command]').forEach(button => {
        button.addEventListener('click', () => applyTextListCommand(button.dataset.boardListCommand));
    });
    document.getElementById('boardStrokeWidth')?.addEventListener('change', event => {
        applySelectedObjectStyle({ strokeWidth: Number(event.target.value) }, false);
    });
    document.getElementById('boardFontFamily')?.addEventListener('change', event => {
        applySelectedTextStyle({ fontFamily: event.target.value }, false);
    });
    document.getElementById('boardFontSize')?.addEventListener('change', event => {
        applySelectedTextStyle({ fontSize: Number(event.target.value) }, false);
    });
    document.getElementById('boardFillOpacity')?.addEventListener('change', () => applyCurrentColor('fill'));
    document.getElementById('boardDrawColor')?.addEventListener('input', event => {
        if (boardCanvas?.freeDrawingBrush) boardCanvas.freeDrawingBrush.color = event.target.value;
    });
    document.getElementById('boardDrawWidth')?.addEventListener('change', event => {
        if (boardCanvas?.freeDrawingBrush) boardCanvas.freeDrawingBrush.width = Number(event.target.value);
    });

    document.querySelectorAll('[data-board-link-action]').forEach(button => {
        button.addEventListener('click', () => {
            if (button.dataset.boardLinkAction === 'confirm') confirmLinkDialog();
            else closeLinkDialog();
        });
    });
    document.getElementById('boardLinkDialog').addEventListener('keydown', event => {
        if (event.key === 'Escape') {
            event.preventDefault();
            closeLinkDialog();
        } else if (event.key === 'Enter') {
            event.preventDefault();
            confirmLinkDialog();
        }
    });

    document.querySelectorAll('[data-board-table-action]').forEach(button => {
        button.addEventListener('click', () => handleTableAction(button.dataset.boardTableAction));
    });
    document.querySelectorAll('[data-board-table-format]').forEach(button => {
        button.addEventListener('click', () => handleTableFormatAction(button.dataset.boardTableFormat));
    });
    document.getElementById('boardTableFill')?.addEventListener('input', event => applyTableStylePatch({ fill: event.target.value }));
    document.getElementById('boardTableTextColor')?.addEventListener('input', event => applyTableStylePatch({ textColor: event.target.value }));
    document.getElementById('boardTableFontFamily')?.addEventListener('change', event => applyTableStylePatch({ fontFamily: event.target.value }));
    document.getElementById('boardTableFontSize')?.addEventListener('change', event => applyTableStylePatch({ fontSize: Number(event.target.value) }));
    const resizeTableFromInputs = () => {
        tableDraft = resizeTableData(
            tableDraft,
            Number(document.getElementById('boardTableRows').value),
            Number(document.getElementById('boardTableColumns').value),
        );
        tableDraftStyles = resizeTableCellStyles(tableDraftStyles, tableDraft.length, tableDraft[0].length);
        tableSelectedCells = new Set([...tableSelectedCells].filter(key => {
            const [row, column] = key.split(':').map(Number);
            return row < tableDraft.length && column < tableDraft[0].length;
        }));
        if (!tableSelectedCells.size) tableSelectedCells.add('0:0');
        renderTableEditor();
    };
    document.getElementById('boardTableRows')?.addEventListener('change', resizeTableFromInputs);
    document.getElementById('boardTableColumns')?.addEventListener('change', resizeTableFromInputs);
    ['boardTableRows', 'boardTableColumns'].forEach(inputID => {
        document.getElementById(inputID)?.addEventListener('keydown', event => {
            if (event.key !== 'Enter') return;
            event.preventDefault();
            resizeTableFromInputs();
            document.getElementById(inputID)?.focus();
        });
    });
    document.querySelectorAll('[data-board-context-action]').forEach(button => {
        button.addEventListener('click', () => handleContextAction(button.dataset.boardContextAction));
    });
    document.getElementById('boardTableDialog').addEventListener('keydown', event => {
        if (event.key === 'Escape') {
            event.preventDefault();
            closeTableEditor();
        } else if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
            event.preventDefault();
            applyTableEditor();
        }
    });

    if (import.meta.env.DEV && new URLSearchParams(window.location.search).has('board-preview')) {
        document.addEventListener('DOMContentLoaded', () => {
            currentProject = { id: 0, name: 'Anteprima lavagna', preview: true };
            document.getElementById('projectBoardProjectName').textContent = currentProject.name;
            document.getElementById('projectBoardModal').classList.add('show');
            requestAnimationFrame(() => initializeCanvas({
                document_json: '{}',
                viewport_json: '{}',
                legacy_note_imported: false,
            }, '# Nota Markdown importata\n\nQuesta nota rimane disponibile come testo originale.'));
        }, { once: true });
    }
}

window.openProjectBoard = async function(projectId, projectName, noteText = '') {
    currentProject = { id: Number(projectId), name: projectName };
    document.getElementById('projectBoardProjectId').value = currentProject.id;
    document.getElementById('projectBoardProjectName').textContent = projectName;
    document.getElementById('projectBoardModal').classList.add('show');
    setBoardRibbonTab('home');
    updateRibbonState();
    setSaveStatus('Caricamento…', 'saving');

    try {
        const board = await GetProjectBoard(currentProject.id);
        requestAnimationFrame(async () => {
            try {
                await initializeCanvas(board, noteText);
            } catch (error) {
                console.error('Errore inizializzazione lavagna:', error);
                setSaveStatus('Errore di caricamento', 'error');
                showNotification(`Errore inizializzazione lavagna: ${error}`, 'error');
            }
        });
    } catch (error) {
        console.error('Errore apertura lavagna:', error);
        setSaveStatus('Errore di caricamento', 'error');
        showNotification(`Errore apertura lavagna: ${error}`, 'error');
    }
};

window.closeProjectBoard = async function() {
    try {
        closeInlineTextEditor(true);
        setDrawingMode(false);
        setDrawingEraserMode(false);
        setConnectorCreationMode(false);
        await saveBoard();
        if (!currentProject?.preview) {
            try {
                await CleanupBoardAssets(currentProject.id, referencedBoardAssetIDs());
            } catch (error) {
                console.warn('Pulizia immagini lavagna non riuscita:', error);
            }
        }
        document.getElementById('projectBoardModal').classList.remove('show');
        if (boardCanvas) await boardCanvas.dispose();
        boardCanvas = null;
        connectorHelpers = [];
        draggingConnectorHandle = null;
        pendingConnectorSelectionID = null;
        currentProject = null;
    } catch {
        showNotification('La lavagna resta aperta perché il salvataggio non è riuscito', 'error');
    }
};
