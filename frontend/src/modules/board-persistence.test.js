import test from 'node:test';
import assert from 'node:assert/strict';
import { restoreBoardObjectMetadata } from './board-persistence.js';

const customProperties = ['boardKind', 'tableData', 'tableCellStyles', 'linkTarget', 'connectorLabel'];

function fakeInstance() {
    return {
        set(values) {
            Object.assign(this, values);
        },
    };
}

test('ripristina tipo e dati editabili di una tabella salvata', () => {
    const instance = fakeInstance();
    const result = restoreBoardObjectMetadata({
        type: 'Group',
        boardKind: 'table',
        tableData: [['Nome', 'Stato'], ['Prova', 'Fatto']],
        tableCellStyles: [[{ fill: '#123456' }, {}], [{}, {}]],
    }, instance, customProperties);

    assert.equal(instance.boardKind, 'table');
    assert.deepEqual(instance.tableData, [['Nome', 'Stato'], ['Prova', 'Fatto']]);
    assert.equal(instance.tableCellStyles[0][0].fill, '#123456');
    assert.equal(result.migrated, false);
});

test('ripristina anche le proprietà personalizzate degli altri gruppi', () => {
    const instance = fakeInstance();
    restoreBoardObjectMetadata({
        type: 'Group',
        boardKind: 'link',
        linkTarget: 'https://example.com',
    }, instance, customProperties);

    assert.equal(instance.boardKind, 'link');
    assert.equal(instance.linkTarget, 'https://example.com');
});

test('riconosce e migra una vecchia tabella priva di metadati', () => {
    const instance = fakeInstance();
    const objects = [];
    [['A', 'B'], ['1', '2']].forEach((row, rowIndex) => {
        row.forEach((cell, columnIndex) => {
            objects.push({ type: 'Rect', left: columnIndex * 180, top: rowIndex * 52, width: 180, height: 52 });
            objects.push({ type: 'Textbox', left: columnIndex * 180 + 10, top: rowIndex * 52 + 9, text: cell });
        });
    });

    const result = restoreBoardObjectMetadata({ type: 'Group', objects }, instance, customProperties);

    assert.equal(instance.boardKind, 'table');
    assert.deepEqual(instance.tableData, [['A', 'B'], ['1', '2']]);
    assert.equal(result.migrated, true);
});

test('non scambia un normale gruppo grafico per una tabella', () => {
    const instance = fakeInstance();
    const result = restoreBoardObjectMetadata({
        type: 'Group',
        objects: [
            { type: 'Rect', left: 0, top: 0 },
            { type: 'Textbox', left: 10, top: 10, text: 'Cornice' },
        ],
    }, instance, customProperties);

    assert.equal(instance.boardKind, undefined);
    assert.equal(instance.tableData, undefined);
    assert.equal(result.migrated, false);
});
