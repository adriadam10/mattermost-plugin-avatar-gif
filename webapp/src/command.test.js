import assert from 'node:assert/strict';
import {test} from 'node:test';

import {wantsPicker} from './command.js';

test('only the bare command opens the picker', () => {
    assert.equal(wantsPicker('/avatar-gif'), true);
    assert.equal(wantsPicker('  /avatar-gif  '), true);
    assert.equal(wantsPicker('/avatar-gif off'), false);
    assert.equal(wantsPicker('/avatar-gif https://example.com/a.gif'), false);
    assert.equal(wantsPicker('/avatar-gifs'), false);
});
