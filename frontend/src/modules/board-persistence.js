import { normalizeTableData } from './board-table-data.js';

function hasOwn(value, property) {
    return Object.prototype.hasOwnProperty.call(value || {}, property);
}

function normalizedType(object) {
    return String(object?.type || '').toLowerCase();
}

function recoverLegacyTableData(serialized, allowSingleCell = false) {
    if (normalizedType(serialized) !== 'group' || !Array.isArray(serialized.objects)) return null;

    const children = serialized.objects;
    const minimumChildren = allowSingleCell ? 2 : 4;
    if (children.length < minimumChildren || children.length % 2 !== 0) return null;

    const textCells = [];
    for (let index = 0; index < children.length; index += 2) {
        const background = children[index];
        const text = children[index + 1];
        const textType = normalizedType(text);
        if (normalizedType(background) !== 'rect' || !['textbox', 'text', 'i-text'].includes(textType)) return null;
        if (!Number.isFinite(Number(text.left)) || !Number.isFinite(Number(text.top))) return null;
        textCells.push(text);
    }

    const sorted = [...textCells].sort((first, second) => {
        const verticalDifference = Number(first.top) - Number(second.top);
        return Math.abs(verticalDifference) > 2 ? verticalDifference : Number(first.left) - Number(second.left);
    });
    const rows = [];
    sorted.forEach(cell => {
        let row = rows.find(candidate => Math.abs(candidate.top - Number(cell.top)) <= 2);
        if (!row) {
            row = { top: Number(cell.top), cells: [] };
            rows.push(row);
        }
        row.cells.push(cell);
    });
    rows.sort((first, second) => first.top - second.top);
    rows.forEach(row => row.cells.sort((first, second) => Number(first.left) - Number(second.left)));
    const columnCount = rows[0]?.cells.length || 0;
    if (!columnCount || rows.some(row => row.cells.length !== columnCount)) return null;
    return normalizeTableData(rows.map(row => row.cells.map(cell => String(cell.text ?? ''))));
}

export function restoreBoardObjectMetadata(serialized, instance, customProperties) {
    if (!serialized || !instance) return { migrated: false };

    const restored = {};
    customProperties.forEach(property => {
        if (hasOwn(serialized, property)) restored[property] = serialized[property];
    });

    let migrated = false;
    const savedTableData = hasOwn(serialized, 'tableData') && Array.isArray(serialized.tableData)
        ? normalizeTableData(serialized.tableData)
        : null;
    if (savedTableData) {
        restored.tableData = savedTableData;
        if (!serialized.boardKind) {
            restored.boardKind = 'table';
            migrated = true;
        }
    } else if (serialized.boardKind === 'table' || !serialized.boardKind) {
        const recoveredTableData = recoverLegacyTableData(serialized, serialized.boardKind === 'table');
        if (recoveredTableData) {
            restored.boardKind = 'table';
            restored.tableData = recoveredTableData;
            migrated = true;
        }
    }

    if (typeof instance.set === 'function') instance.set(restored);
    else Object.assign(instance, restored);
    return { migrated };
}
