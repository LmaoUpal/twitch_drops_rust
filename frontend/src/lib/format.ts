import {lang, locale, t} from '../i18n';

const ZERO_TIME = '0001-01-01T00:00:00Z';

export function isSet(t?: string) {
    return !!t && t !== ZERO_TIME && !t.startsWith('0001-');
}

/** Number with unit: "5m" in English, "5 мин" in Russian. */
function unit(n: number | string, u: 'unit.h' | 'unit.m' | 'unit.d') {
    return lang() === 'ru' ? `${n} ${t(u)}` : `${n}${t(u)}`;
}

export function fmtMinutes(m: number) {
    const h = Math.floor(m / 60), min = Math.round(m % 60);
    if (!h) return unit(min, 'unit.m');
    return min ? `${unit(h, 'unit.h')} ${unit(String(min).padStart(2, '0'), 'unit.m')}` : unit(h, 'unit.h');
}

export function fmtDuration(ms: number) {
    const min = Math.max(0, Math.round(ms / 60000));
    const d = Math.floor(min / 1440), h = Math.floor((min % 1440) / 60), m = min % 60;
    if (d) return h ? `${unit(d, 'unit.d')} ${unit(h, 'unit.h')}` : unit(d, 'unit.d');
    if (h) return m ? `${unit(h, 'unit.h')} ${unit(m, 'unit.m')}` : unit(h, 'unit.h');
    return unit(m, 'unit.m');
}

export function fmtCountdown(ms: number) {
    const s = Math.max(0, Math.floor(ms / 1000));
    const d = Math.floor(s / 86400), rest = s % 86400;
    const hh = String(Math.floor(rest / 3600)).padStart(2, '0');
    const mm = String(Math.floor((rest % 3600) / 60)).padStart(2, '0');
    const ss = String(rest % 60).padStart(2, '0');
    return d ? `${d}${t('unit.d')} ${hh}:${mm}:${ss}` : `${hh}:${mm}:${ss}`;
}

// Intl formatters are costly to build; keep one per locale and pattern.
const cache = new Map<string, Intl.DateTimeFormat>();

function dtf(name: string, opts: Intl.DateTimeFormatOptions) {
    const key = `${locale()}|${name}`;
    let f = cache.get(key);
    if (!f) {
        f = new Intl.DateTimeFormat(locale(), opts);
        cache.set(key, f);
    }
    return f;
}

export function fmtDay(v: string | Date) {
    return dtf('day', {month: 'short', day: 'numeric'}).format(new Date(v));
}

export function fmtRange(start: string, end: string) {
    return `${fmtDay(start)} – ${fmtDay(end)}`;
}

export function fmtTime(v: string | Date) {
    return dtf('time', {hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false}).format(new Date(v));
}

export function fmtHM(v: string | Date) {
    return dtf('hm', {hour: '2-digit', minute: '2-digit', hour12: false}).format(new Date(v));
}

export function fmtMonth(v: Date) {
    const s = dtf('month', {month: 'long', year: 'numeric'}).format(v);
    return s.charAt(0).toUpperCase() + s.slice(1);
}

export function fmtWeekday(v: Date) {
    return dtf('weekday', {weekday: 'long', month: 'short', day: 'numeric'}).format(v);
}

export function fmtViewers(n: number) {
    return n.toLocaleString(locale());
}

export function pct(cur: number, req: number) {
    return req ? Math.min(100, Math.round(100 * cur / req)) : 0;
}
