import test from 'node:test';
import assert from 'node:assert/strict';

import { calculateContractedHours, timeToMinutes } from './schedule.js';

test('timeToMinutes accetta solo HH:MM rigoroso', () => {
    assert.equal(timeToMinutes('08:30'), 510);
    assert.equal(Number.isNaN(timeToMinutes('8:30')), true);
    assert.equal(Number.isNaN(timeToMinutes('24:00')), true);
});

test('calculateContractedHours sottrae una pausa valida', () => {
    assert.equal(calculateContractedHours('08:00', '17:00', true, '13:00', '14:00'), 8);
});

test('calculateContractedHours rifiuta intervalli incoerenti', () => {
    assert.equal(calculateContractedHours('17:00', '08:00', false, '13:00', '14:00'), 0);
    assert.equal(calculateContractedHours('08:00', '17:00', true, '07:00', '14:00'), 0);
});
