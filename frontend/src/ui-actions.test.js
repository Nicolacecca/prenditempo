import test from 'node:test';
import assert from 'node:assert/strict';
import { runWindowAction } from './ui-actions.js';

test('runWindowAction passa elemento e argomento dichiarativo alle azioni click', () => {
    const calls = [];
    globalThis.window = { select: (...args) => calls.push(args) };
    const element = { dataset: { actionArg: 'end' } };
    const event = { type: 'click' };

    runWindowAction(element, 'select', event, true);

    assert.deepEqual(calls, [[element, 'end']]);
});

test('runWindowAction passa alle azioni change anche l’evento originale', () => {
    const calls = [];
    globalThis.window = { importFile: (...args) => calls.push(args) };
    const element = { dataset: {} };
    const event = { type: 'change', target: element };

    runWindowAction(element, 'importFile', event);

    assert.deepEqual(calls, [[event]]);
});
