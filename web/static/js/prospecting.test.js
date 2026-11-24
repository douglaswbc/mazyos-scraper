'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const prospecting = require('./prospecting.js');
const { suggestionFor, messageFor } = prospecting;

test('missing Maps website suggests a site page without claiming the business has none', function () {
    const place = { title: 'Clínica Sol', category: 'Clínica', city: 'Belém', website: '' };
    const suggestion = suggestionFor(place);
    const message = messageFor(place, 'new');

    assert.equal(suggestion.offer, 'Criação de site ou landing page');
    assert.match(suggestion.reason, /perfil.*Maps.*não mostra/i);
    assert.match(message, /não mostra um link de site/i);
    assert.match(message, /não significa que vocês não tenham um site/i);
    assert.match(message, /posso perguntar/i);
});

test('social-only listing suggests a landing page and names the observable signal', function () {
    const place = { title: 'Studio Sol', category: 'Salão', city: 'Belém', website: 'https://instagram.com/studiosol' };
    const suggestion = suggestionFor(place);

    assert.equal(suggestion.offer, 'Criação de site ou landing page');
    assert.match(suggestion.reason, /rede social/i);
});

test('a listed website only qualifies interest in local presence; it does not diagnose a problem', function () {
    const place = { title: 'Restaurante Mar', category: 'Restaurante', city: 'Belém', website: 'https://restaurante.example' };
    const suggestion = suggestionFor(place);
    const message = messageFor(place, 'new');

    assert.equal(suggestion.offer, 'Combo Presença Local');
    assert.match(suggestion.reason, /confirm/i);
    assert.match(message, /não vou presumir/i);
});

test('follow-up sequence ends with a respectful final touch and stops for uninterested leads', function () {
    const place = { title: 'Empresa Norte', category: 'Serviços', city: 'Belém', website: '' };
    const finalTouch = messageFor(place, 'follow_up_2');

    assert.match(finalTouch, /última mensagem/i);
    assert.match(finalTouch, /não vou retornar/i);
    assert.equal(messageFor(place, 'not_interested'), '');
});

test('external scraped links only allow HTTP and HTTPS schemes', function () {
    assert.equal(prospecting.safeExternalURL('https://example.com/path'), 'https://example.com/path');
    assert.equal(prospecting.safeExternalURL('http://example.com'), 'http://example.com/');
    assert.equal(prospecting.safeExternalURL('javascript:alert(1)'), '');
    assert.equal(prospecting.safeExternalURL('data:text/html,<script>alert(1)</script>'), '');
    assert.equal(prospecting.safeExternalURL('ftp://example.com'), '');
    assert.equal(prospecting.safeExternalURL('//example.com'), '');
    assert.equal(prospecting.linkKind('javascript:alert(1)'), 'none');
    assert.equal(suggestionFor({ website: 'javascript:alert(1)' }).offer, 'Criação de site ou landing page');
});

test('suggested quote recalculates the recommended offer and its one-time default', function () {
    const offers = [
        { id: 4, name: 'Combo Presença Local', active: true, default_price_cents: 49000 },
        { id: 5, name: 'Criação de site ou landing page', active: true, default_price_cents: null }
    ];

    assert.deepEqual(prospecting.quoteFor({ website: 'https://empresa.example' }, offers), {
        offerId: 4,
        defaultPriceCents: 49000
    });
    assert.deepEqual(prospecting.quoteFor({ website: '' }, offers), {
        offerId: 5,
        defaultPriceCents: null
    });
    assert.equal(prospecting.quoteFor({ website: '' }, [offers[0]]), null);
});

test('CRM refresh keeps the selected lead when it is still available', function () {
    assert.deepEqual(prospecting.resolveLeadSelection('lead-b', ['lead-a', 'lead-b']), {
        key: 'lead-b',
        changed: false
    });
    assert.deepEqual(prospecting.resolveLeadSelection('removed-lead', ['lead-a', 'lead-b']), {
        key: 'lead-a',
        changed: true
    });
    assert.deepEqual(prospecting.resolveLeadSelection('', []), { key: '', changed: false });
});
