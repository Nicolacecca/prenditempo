export const BOARD_ANCHORS = Object.freeze([
    'top-left',
    'top',
    'top-right',
    'right',
    'bottom-right',
    'bottom',
    'bottom-left',
    'left',
]);

function interpolate(from, to, ratio) {
    return {
        x: from.x + (to.x - from.x) * ratio,
        y: from.y + (to.y - from.y) * ratio,
    };
}

export function anchorPointsFromCorners(corners) {
    if (!Array.isArray(corners) || corners.length !== 4) throw new Error('servono quattro vertici');
    const [topLeft, topRight, bottomRight, bottomLeft] = corners;
    return [
        { id: 'top-left', ...topLeft },
        { id: 'top', ...interpolate(topLeft, topRight, 0.5) },
        { id: 'top-right', ...topRight },
        { id: 'right', ...interpolate(topRight, bottomRight, 0.5) },
        { id: 'bottom-right', ...bottomRight },
        { id: 'bottom', ...interpolate(bottomRight, bottomLeft, 0.5) },
        { id: 'bottom-left', ...bottomLeft },
        { id: 'left', ...interpolate(bottomLeft, topLeft, 0.5) },
    ];
}

export function nearestAnchor(anchors, point) {
    if (!anchors?.length) return null;
    return anchors.reduce((nearest, anchor) => {
        const distance = Math.hypot(anchor.x - point.x, anchor.y - point.y);
        return !nearest || distance < nearest.distance ? { ...anchor, distance } : nearest;
    }, null);
}

export function closestAnchorPair(fromAnchors, toAnchors) {
    let best = null;
    fromAnchors.forEach(from => {
        toAnchors.forEach(to => {
            const distance = Math.hypot(to.x - from.x, to.y - from.y);
            if (!best || distance < best.distance) best = { from, to, distance };
        });
    });
    return best;
}
