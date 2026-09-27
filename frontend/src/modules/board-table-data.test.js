import test from 'node:test';
import assert from 'node:assert/strict';
import {
    addTableColumn,
    addTableRow,
    createDefaultTableData,
    normalizeTableData,
    removeTableColumn,
    removeTableRow,
    resizeTableData,
    TABLE_MAX_CELL_LENGTH,
} from './board-table-data.js';

test('crea una tabella predefinita rettangolare con intestazioni', () => {
    assert.deepEqual(createDefaultTableData(2, 2), [
        ['Colonna 1', 'Colonna 2'],
        ['', ''],
    ]);
});

test('ridimensiona direttamente la tabella preservando le celle esistenti', () => {
    const resized = resizeTableData([['Nome'], ['Attività']], 3, 2);
    assert.deepEqual(resized, [
        ['Nome', 'Colonna 2'],
        ['Attività', ''],
        ['', ''],
    ]);
    assert.deepEqual(resizeTableData(resized, 1, 1), [['Nome']]);
});

test('normalizza righe irregolari e limita il contenuto delle celle', () => {
    const data = normalizeTableData([['A'], ['B', 'x'.repeat(TABLE_MAX_CELL_LENGTH + 20)]]);
    assert.deepEqual(data[0], ['A', '']);
    assert.equal(data[1][1].length, TABLE_MAX_CELL_LENGTH);
});

test('aggiunge e rimuove righe e colonne mantenendo almeno una cella', () => {
    let data = createDefaultTableData(1, 1);
    data = addTableRow(data);
    data = addTableColumn(data);
    assert.deepEqual(data, [['Colonna 1', 'Colonna 2'], ['', '']]);
    data = removeTableRow(removeTableRow(data));
    data = removeTableColumn(removeTableColumn(data));
    assert.deepEqual(data, [['Colonna 1']]);
});
