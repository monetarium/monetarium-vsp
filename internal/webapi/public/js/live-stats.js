// Keeps the numbers on the page current without a reload.
//
// The figures come from /api/v3/vspinfo, the same public endpoint wallets
// poll: no new route, no admin access, nothing here that a wallet could not
// already read. The server refreshes its own cache once a minute, so polling
// faster than that would only re-fetch identical bytes.
//
// Everything degrades to the server-rendered values: on a failed request, a
// stalled tab, or with scripting off, the page keeps whatever the template
// produced. No dependencies, so it works under a strict CSP.
(function () {
    'use strict';

    var POLL_MS = 60000;

    function el(name) {
        return document.querySelector('[data-stat="' + name + '"]');
    }

    function setText(name, value) {
        var node = el(name);
        if (node && node.textContent !== value) {
            node.textContent = value;
        }
    }

    // Matches the server's humanize.Comma so a refreshed number cannot
    // suddenly render in a different style from the one beside it.
    function comma(n) {
        return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, ',');
    }

    function percent(fraction) {
        return (fraction * 100).toFixed(2) + '%';
    }

    // Mirrors cache.update(): expired and missed are shares of all tickets
    // that have reached an outcome, not of every ticket ever seen.
    function outcomeShare(part, voted, expired, missed) {
        var total = voted + expired + missed;
        return total === 0 ? 0 : part / total;
    }

    function timeAgo(seconds) {
        if (seconds < 60) {
            return seconds + (seconds === 1 ? ' second ago' : ' seconds ago');
        }
        var minutes = Math.floor(seconds / 60);
        if (minutes < 60) {
            return minutes + (minutes === 1 ? ' minute ago' : ' minutes ago');
        }
        var hours = Math.floor(minutes / 60);
        return hours + (hours === 1 ? ' hour ago' : ' hours ago');
    }

    function apply(info) {
        setText('voting', comma(info.voting));
        setText('voted', comma(info.voted));
        setText('expired', comma(info.expired));
        setText('missed', comma(info.missed));
        setText('expiredProportion', percent(outcomeShare(info.expired, info.voted, info.expired, info.missed)));
        setText('missedProportion', percent(outcomeShare(info.missed, info.voted, info.expired, info.missed)));
        setText('feePercentage', info.feepercentage + '%');
        setText('networkProportion', percent(info.estimatednetworkproportion));

        // vspinfo carries the moment the server built its cache, so the
        // footer can age honestly instead of counting from page load.
        var updated = el('statsUpdated');
        if (updated && typeof info.timestamp === 'number') {
            updated.setAttribute('data-timestamp', String(info.timestamp));
        }
    }

    function refresh() {
        fetch('/api/v3/vspinfo', { cache: 'no-store' })
            .then(function (resp) { return resp.ok ? resp.json() : null; })
            .then(function (info) { if (info) { apply(info); } })
            .catch(function () { /* keep the rendered values */ });
    }

    // The "N seconds ago" line has to tick on its own; without it the page
    // would claim the stats are fresh for a whole minute after they aged.
    function tickAge() {
        var updated = el('statsUpdated');
        if (!updated) {
            return;
        }
        var stamp = parseInt(updated.getAttribute('data-timestamp'), 10);
        if (!stamp) {
            return;
        }
        var age = Math.max(0, Math.floor(Date.now() / 1000) - stamp);
        updated.textContent = timeAgo(age);
    }

    // A hidden tab polling every minute is pure waste; catch up on return.
    function onVisible() {
        if (!document.hidden) {
            refresh();
        }
    }

    document.addEventListener('visibilitychange', onVisible);
    setInterval(function () { if (!document.hidden) { refresh(); } }, POLL_MS);
    setInterval(tickAge, 1000);
    refresh();
})();
