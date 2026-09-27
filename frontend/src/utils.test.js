import test from 'node:test';
import assert from 'node:assert/strict';

import { escapeHtml, escapeJs, validateLength } from './utils.js';

test('escapeHtml codifica testo e delimitatori di attributo', () => {
    assert.equal(
        escapeHtml(`<img src="x" onerror='alert(1)'>&`),
        '&lt;img src=&quot;x&quot; onerror=&#39;alert(1)&#39;&gt;&amp;'
    );
});

test('escapeJs protegge una stringa JS dentro un attributo HTML', () => {
    const escaped = escapeJs(`x');alert(1);//"&quot;`);
    assert.equal(escaped, `x\\');alert(1);//&quot;&amp;quot;`);
});

test('validateLength rifiuta valori troppo lunghi', () => {
    assert.throws(() => validateLength('abcd', 3, 'Campo'), /Campo/);
    assert.equal(validateLength('abc', 3, 'Campo'), 'abc');
});
