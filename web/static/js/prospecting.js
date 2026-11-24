'use strict';

(function (root, create) {
    var api = create();
    if (typeof module === 'object' && module.exports) module.exports = api;
    if (root) root.ScraperProspecting = api;
}(typeof globalThis !== 'undefined' ? globalThis : this, function () {
    function safeExternalURL(value) {
        if (typeof value !== 'string' || !value.trim()) return '';
        try {
            var url = new URL(value.trim());
            if (url.protocol !== 'http:' && url.protocol !== 'https:') return '';
            return url.href;
        } catch (_) {
            return '';
        }
    }

    function linkKind(url) {
        var safeURL = safeExternalURL(url);
        if (!safeURL) return 'none';
        if (/instagram\.com|facebook\.com|linktr\.ee|wa\.me|whatsapp\.com|tiktok\.com/i.test(safeURL)) return 'social';
        return 'site';
    }

    function suggestionFor(place) {
        place = place || {};
        var kind = linkKind(place.website);
        if (kind === 'none') {
            return {
                offer: 'Criação de site ou landing page',
                reason: 'O perfil consultado no Maps não mostra um link de site; confirme se há um site fora do perfil.'
            };
        }
        if (kind === 'social') {
            return {
                offer: 'Criação de site ou landing page',
                reason: 'O link público do perfil aponta para uma rede social; confirme se a empresa já tem página própria.'
            };
        }
        return {
            offer: 'Combo Presença Local',
            reason: 'O perfil aparece no Maps; confirme em conversa se querem fortalecer o Perfil do Google e facilitar avaliações, sem presumir uma falha.'
        };
    }

    function quoteFor(place, offers) {
        var suggestedName = suggestionFor(place).offer;
        var offer = (offers || []).filter(function (item) {
            return item && item.active && item.name === suggestedName;
        })[0];
        if (!offer) return null;
        return {
            offerId: Number(offer.id),
            defaultPriceCents: offer.default_price_cents == null ? null : Number(offer.default_price_cents)
        };
    }

    function resolveLeadSelection(currentKey, availableKeys) {
        availableKeys = Array.isArray(availableKeys) ? availableKeys : [];
        if (currentKey && availableKeys.indexOf(currentKey) !== -1) {
            return { key: currentKey, changed: false };
        }
        var key = availableKeys.length ? availableKeys[0] : '';
        return { key: key, changed: key !== (currentKey || '') };
    }

    function messageFor(place, stage) {
        place = place || {};
        var company = place.title || 'sua empresa';
        var category = place.category || 'negócios locais';
        var city = place.city || 'sua região';
        var suggestion = suggestionFor(place);
        var kind = linkKind(place.website);

        if (stage === 'new') {
            if (kind === 'none') {
                return 'Olá! Sou Douglas Cuimar. Encontrei o perfil de ' + company + ' no Google Maps enquanto pesquisava ' + category + ' em ' + city + '. O perfil que consultei não mostra um link de site — isso não significa que vocês não tenham um site. Trabalho com sites e presença digital local; antes de sugerir qualquer serviço, posso perguntar como vocês recebem contatos de quem encontra a empresa no Google? Se não for prioridade agora, tudo certo.';
            }
            if (kind === 'social') {
                return 'Olá! Sou Douglas Cuimar. Encontrei o perfil de ' + company + ' no Google Maps enquanto pesquisava ' + category + ' em ' + city + '. O link público do perfil direciona para uma rede social; não sei se vocês também têm uma página própria. Trabalho com sites e landing pages para negócios locais. Posso perguntar se uma página de contato é uma prioridade para vocês? Se não, sem problema.';
            }
            return 'Olá! Sou Douglas Cuimar. Encontrei o perfil de ' + company + ' no Google Maps enquanto pesquisava ' + category + ' em ' + city + ', e vi que há um site listado. Não vou presumir que exista algum problema. Também trabalho com reputação local — Perfil de Empresa no Google e placas NFC/QR. Isso é uma frente que vocês querem fortalecer? Se não for prioridade, tudo bem.';
        }
        if (stage === 'first_contact') {
            return 'Oi! Retomo com cuidado sobre ' + suggestion.offer + '. Não sei se isso está entre as prioridades de ' + company + '; posso te explicar em poucas linhas se fizer sentido. Se já estiver resolvido ou não for o momento, tudo certo.';
        }
        if (stage === 'follow_up_1') {
            return 'Oi, ' + company + '! Deixo uma pergunta simples para não mandar uma proposta genérica: ' + suggestion.reason + ' Se quiser, posso compartilhar uma explicação curta sobre ' + suggestion.offer + '. Se não for útil, pode me avisar e encerro por aqui.';
        }
        if (stage === 'follow_up_2') {
            return 'Oi, ' + company + '. Esta é minha última mensagem para respeitar seu tempo. Se quiser conversar sobre ' + suggestion.offer + ', fico à disposição. Se não for prioridade, tudo bem — não vou retornar sobre este assunto.';
        }
        if (stage === 'replied') {
            return 'Obrigado por responder! Para eu não sugerir um pacote sem contexto, o que é prioridade para vocês agora: presença no Google, transformar visitas em contatos ou produzir conteúdo? A partir disso vejo, com transparência, se alguma solução faz sentido.';
        }
        return '';
    }

    return { safeExternalURL: safeExternalURL, linkKind: linkKind, suggestionFor: suggestionFor, quoteFor: quoteFor, resolveLeadSelection: resolveLeadSelection, messageFor: messageFor };
}));
