const DEFAULT_HEADER_STYLE = Object.freeze({
    fill: '#362C21',
    textColor: '#E4DACB',
    fontFamily: 'Arial',
    fontSize: 17,
    fontWeight: 'bold',
    fontStyle: 'normal',
    underline: false,
    horizontalAlign: 'left',
    verticalAlign: 'middle',
    wrap: true,
    borderColor: '#443A2E',
    borderWidth: 1,
});

const DEFAULT_CELL_STYLE = Object.freeze({
    ...DEFAULT_HEADER_STYLE,
    fill: '#211C17',
    textColor: '#E4DACB',
    fontWeight: 'normal',
});

function validColor(value, fallback) {
    return typeof value === 'string' && value.trim() ? value : fallback;
}

export function defaultTableCellStyle(rowIndex = 0) {
    return { ...(rowIndex === 0 ? DEFAULT_HEADER_STYLE : DEFAULT_CELL_STYLE) };
}

export function normalizeTableCellStyle(value, rowIndex = 0) {
    const fallback = defaultTableCellStyle(rowIndex);
    const source = value && typeof value === 'object' ? value : {};
    return {
        fill: validColor(source.fill, fallback.fill),
        textColor: validColor(source.textColor, fallback.textColor),
        fontFamily: String(source.fontFamily || fallback.fontFamily).slice(0, 80),
        fontSize: Math.max(8, Math.min(Number(source.fontSize) || fallback.fontSize, 96)),
        fontWeight: source.fontWeight === undefined ? fallback.fontWeight : source.fontWeight === 'bold' ? 'bold' : 'normal',
        fontStyle: source.fontStyle === undefined ? fallback.fontStyle : source.fontStyle === 'italic' ? 'italic' : 'normal',
        underline: source.underline === undefined ? fallback.underline : Boolean(source.underline),
        horizontalAlign: ['left', 'center', 'right'].includes(source.horizontalAlign) ? source.horizontalAlign : fallback.horizontalAlign,
        verticalAlign: ['top', 'middle', 'bottom'].includes(source.verticalAlign) ? source.verticalAlign : fallback.verticalAlign,
        wrap: source.wrap !== false,
        borderColor: validColor(source.borderColor, fallback.borderColor),
        borderWidth: Math.max(0, Math.min(Number(source.borderWidth) || fallback.borderWidth, 12)),
    };
}

export function resizeTableCellStyles(value, rows, columns) {
    const safeRows = Math.max(1, Number(rows) || 1);
    const safeColumns = Math.max(1, Number(columns) || 1);
    return Array.from({ length: safeRows }, (_, rowIndex) => (
        Array.from({ length: safeColumns }, (_, columnIndex) => (
            normalizeTableCellStyle(value?.[rowIndex]?.[columnIndex], rowIndex)
        ))
    ));
}

export function patchSelectedTableCellStyles(value, selection, patch) {
    const rows = Array.isArray(value) ? value.length : 0;
    const columns = rows && Array.isArray(value[0]) ? value[0].length : 0;
    const selected = new Set(selection || []);
    return resizeTableCellStyles(value, rows || 1, columns || 1).map((row, rowIndex) => row.map((style, columnIndex) => (
        selected.has(`${rowIndex}:${columnIndex}`) ? normalizeTableCellStyle({ ...style, ...patch }, rowIndex) : style
    )));
}
