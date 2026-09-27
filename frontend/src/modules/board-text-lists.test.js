import test from 'node:test';
import assert from 'node:assert/strict';
import { listPrefixEdits } from './board-text-lists.js';

function applyEdits(text, edits) {
    let result = text;
    [...edits].sort((a, b) => b.start - a.start).forEach(edit => {
        result = result.slice(0, edit.start) + edit.insert + result.slice(edit.start + edit.removeLength);
    });
    return result;
}

test('crea e rimuove elenchi puntati e numerati', () => {
    const source = 'Prima\nSeconda';
    const bullets = applyEdits(source, listPrefixEdits(source, { mode: 'bullet' }));
    assert.equal(bullets, '• Prima\n• Seconda');
    assert.equal(applyEdits(bullets, listPrefixEdits(bullets, { mode: 'numbered' })), '1. Prima\n2. Seconda');
    assert.equal(applyEdits(bullets, listPrefixEdits(bullets, { mode: 'bullet' })), source);
});

test('inserisce checklist e alterna lo stato delle sole righe selezionate', () => {
    const source = 'Preparare offerta\nInviare email';
    const checklist = applyEdits(source, listPrefixEdits(source, { mode: 'checklist' }));
    assert.equal(checklist, '☐ Preparare offerta\n☐ Inviare email');
    const completed = applyEdits(checklist, listPrefixEdits(checklist, {
        mode: 'toggle-check', start: 2, end: 2, editing: true,
    }));
    assert.equal(completed, '☑ Preparare offerta\n☐ Inviare email');
});
