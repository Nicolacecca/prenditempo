export const TABLE_MAX_ROWS = 30;
export const TABLE_MAX_COLUMNS = 12;
export const TABLE_MAX_CELL_LENGTH = 500;

export function createDefaultTableData(rows = 3, columns = 3) {
    const safeRows = Math.max(1, Math.min(Number(rows) || 1, TABLE_MAX_ROWS));
    const safeColumns = Math.max(1, Math.min(Number(columns) || 1, TABLE_MAX_COLUMNS));
    return Array.from({ length: safeRows }, (_, row) => (
        Array.from({ length: safeColumns }, (_, column) => (
            row === 0 ? `Colonna ${column + 1}` : ''
        ))
    ));
}

export function normalizeTableData(value) {
    if (!Array.isArray(value) || value.length === 0) return createDefaultTableData();
    const rows = value.slice(0, TABLE_MAX_ROWS);
    const columns = Math.max(1, Math.min(
        TABLE_MAX_COLUMNS,
        Math.max(...rows.map(row => Array.isArray(row) ? row.length : 0)),
    ));
    return rows.map(row => Array.from({ length: columns }, (_, column) => (
        String(Array.isArray(row) ? row[column] ?? '' : '').slice(0, TABLE_MAX_CELL_LENGTH)
    )));
}

export function addTableRow(value) {
    const data = normalizeTableData(value);
    if (data.length >= TABLE_MAX_ROWS) return data;
    return [...data, Array(data[0].length).fill('')];
}

export function removeTableRow(value) {
    const data = normalizeTableData(value);
    return data.length > 1 ? data.slice(0, -1) : data;
}

export function addTableColumn(value) {
    const data = normalizeTableData(value);
    if (data[0].length >= TABLE_MAX_COLUMNS) return data;
    return data.map((row, rowIndex) => [
        ...row,
        rowIndex === 0 ? `Colonna ${row.length + 1}` : '',
    ]);
}

export function removeTableColumn(value) {
    const data = normalizeTableData(value);
    return data[0].length > 1 ? data.map(row => row.slice(0, -1)) : data;
}

export function resizeTableData(value, requestedRows, requestedColumns) {
    const data = normalizeTableData(value);
    const rows = Math.max(1, Math.min(Number(requestedRows) || 1, TABLE_MAX_ROWS));
    const columns = Math.max(1, Math.min(Number(requestedColumns) || 1, TABLE_MAX_COLUMNS));
    return Array.from({ length: rows }, (_, rowIndex) => (
        Array.from({ length: columns }, (_, columnIndex) => {
            if (data[rowIndex]?.[columnIndex] !== undefined) return data[rowIndex][columnIndex];
            return rowIndex === 0 ? `Colonna ${columnIndex + 1}` : '';
        })
    ));
}
