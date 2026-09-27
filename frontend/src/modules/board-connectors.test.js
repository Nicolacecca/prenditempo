import test from 'node:test';
import assert from 'node:assert/strict';
import { anchorPointsFromCorners, closestAnchorPair, nearestAnchor } from './board-connectors.js';

test('calcola quattro vertici e quattro mezzerie dei lati', () => {
    const anchors = anchorPointsFromCorners([
        { x: 0, y: 0 }, { x: 120, y: 0 }, { x: 120, y: 90 }, { x: 0, y: 90 },
    ]);
    assert.equal(anchors.length, 8);
    assert.deepEqual(anchors.map(({ id, x, y }) => ({ id, x, y })), [
        { id: 'top-left', x: 0, y: 0 },
        { id: 'top', x: 60, y: 0 },
        { id: 'top-right', x: 120, y: 0 },
        { id: 'right', x: 120, y: 45 },
        { id: 'bottom-right', x: 120, y: 90 },
        { id: 'bottom', x: 60, y: 90 },
        { id: 'bottom-left', x: 0, y: 90 },
        { id: 'left', x: 0, y: 45 },
    ]);
});

test('trova il punto e la coppia di agganci più vicini', () => {
    const from = anchorPointsFromCorners([
        { x: 0, y: 0 }, { x: 100, y: 0 }, { x: 100, y: 100 }, { x: 0, y: 100 },
    ]);
    const to = anchorPointsFromCorners([
        { x: 300, y: 0 }, { x: 400, y: 0 }, { x: 400, y: 100 }, { x: 300, y: 100 },
    ]);
    assert.equal(nearestAnchor(from, { x: 96, y: 10 }).id, 'top-right');
    const pair = closestAnchorPair(from, to);
    assert.equal(pair.from.id, 'top-right');
    assert.equal(pair.to.id, 'top-left');
});
