export function timeToMinutes(value) {
    if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(value || '')) return NaN;
    const [hours, minutes] = value.split(':').map(Number);
    return hours * 60 + minutes;
}

// Duplica computeContractedHours in ../validation.go per il preview live lato UI
// senza round-trip IPC: tieni le due formule allineate.
export function calculateContractedHours(start, end, lunchEnabled, lunchStart, lunchEnd) {
    const startMinutes = timeToMinutes(start);
    const endMinutes = timeToMinutes(end);
    if (!Number.isFinite(startMinutes) || !Number.isFinite(endMinutes) || endMinutes <= startMinutes) return 0;

    let totalMinutes = endMinutes - startMinutes;
    if (lunchEnabled) {
        const lunchStartMinutes = timeToMinutes(lunchStart);
        const lunchEndMinutes = timeToMinutes(lunchEnd);
        if (!Number.isFinite(lunchStartMinutes) || !Number.isFinite(lunchEndMinutes) ||
            lunchStartMinutes < startMinutes || lunchEndMinutes > endMinutes || lunchEndMinutes <= lunchStartMinutes) {
            return 0;
        }
        totalMinutes -= lunchEndMinutes - lunchStartMinutes;
    }
    return totalMinutes / 60;
}
