import test from 'node:test';
import assert from 'node:assert/strict';
import {
    patchSelectedTableCellStyles,
    resizeTableCellStyles,
} from './board-table-styles.js';

test('crea stili distinti per intestazione e celle normali', () => {
    const styles = resizeTableCellStyles([], 2, 2);
    assert.equal(styles[0][0].fontWeight, 'bold');
    assert.equal(styles[1][0].fontWeight, 'normal');
    assert.notEqual(styles[0][0].fill, styles[1][0].fill);
});

test('preserva gli stili esistenti quando la tabella viene ampliata', () => {
    const original = resizeTableCellStyles([], 2, 2);
    original[1][1] = { ...original[1][1], fill: '#ff0000', horizontalAlign: 'right' };
    const resized = resizeTableCellStyles(original, 3, 3);
    assert.equal(resized[1][1].fill, '#ff0000');
    assert.equal(resized[1][1].horizontalAlign, 'right');
    assert.equal(resized[2][2].fill, '#211C17');
});

test('formatta soltanto le celle selezionate', () => {
    const original = resizeTableCellStyles([], 2, 2);
    const changed = patchSelectedTableCellStyles(original, ['1:0'], { verticalAlign: 'bottom', underline: true });
    assert.equal(changed[1][0].verticalAlign, 'bottom');
    assert.equal(changed[1][0].underline, true);
    assert.equal(changed[1][1].verticalAlign, 'middle');
});
