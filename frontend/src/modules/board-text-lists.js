const PREFIX_PATTERN = /^(?:•\s+|\d+\.\s+|[☐☑]\s+)/;

function textLines(value) {
    const text = String(value ?? '');
    let offset = 0;
    return text.split('\n').map((content, index) => {
        const line = { index, content, start: offset, end: offset + content.length };
        offset = line.end + 1;
        return line;
    });
}

function affectedLines(lines, start, end, editing) {
    if (!editing) return lines;
    const from = Math.max(0, Number(start) || 0);
    const to = Math.max(from, Number(end) || from);
    if (to === from) {
        return lines.filter(line => from >= line.start && from <= line.end + 1).slice(0, 1);
    }
    return lines.filter(line => line.start < to && line.end + 1 > from);
}

function prefixOf(content) {
    return content.match(PREFIX_PATTERN)?.[0] || '';
}

export function listPrefixEdits(text, options = {}) {
    const lines = affectedLines(textLines(text), options.start, options.end, Boolean(options.editing));
    if (!lines.length) return [];
    const prefixes = lines.map(line => prefixOf(line.content));
    const mode = options.mode;
    const removeExisting = (line, index, insert) => ({
        start: line.start,
        removeLength: prefixes[index].length,
        insert,
    });

    if (mode === 'bullet') {
        const remove = prefixes.every(prefix => prefix.startsWith('•'));
        return lines.map((line, index) => removeExisting(line, index, remove ? '' : '• '));
    }
    if (mode === 'numbered') {
        const remove = prefixes.every(prefix => /^\d+\.\s+$/.test(prefix));
        return lines.map((line, index) => removeExisting(line, index, remove ? '' : `${index + 1}. `));
    }
    if (mode === 'checklist') {
        const remove = prefixes.every(prefix => /^[☐☑]\s+$/.test(prefix));
        return lines.map((line, index) => removeExisting(line, index, remove ? '' : '☐ '));
    }
    if (mode === 'toggle-check') {
        return lines.map((line, index) => {
            const prefix = prefixes[index];
            const insert = prefix.startsWith('☐') ? '☑ ' : prefix.startsWith('☑') ? '☐ ' : '☐ ';
            return removeExisting(line, index, insert);
        });
    }
    return [];
}
