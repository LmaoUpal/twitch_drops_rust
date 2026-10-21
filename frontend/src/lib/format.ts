const ZERO_TIME = '0001-01-01T00:00:00Z';

export function isSet(t?: string) {
    return !!t && t !== ZERO_TIME && !t.startsWith('0001-');
}

export function fmtMinutes(m: number) {
    const h = Math.floor(m / 60), min = Math.round(m % 60);
    if (!h) return `${min}m`;
    return min ? `${h}h ${String(min).padStart(2, '0')}m` : `${h}h`;
}

export function fmtDuration(ms: number) {
    const min = Math.max(0, Math.round(ms / 60000));
    const d = Math.floor(min / 1440), h = Math.floor((min % 1440) / 60), m = min % 60;
    if (d) return h ? `${d}d ${h}h` : `${d}d`;
    if (h) return m ? `${h}h ${m}m` : `${h}h`;
    return `${m}m`;
}

export function fmtCountdown(ms: number) {
    const s = Math.max(0, Math.floor(ms / 1000));
    const d = Math.floor(s / 86400), rest = s % 86400;
    const hh = String(Math.floor(rest / 3600)).padStart(2, '0');
    const mm = String(Math.floor((rest % 3600) / 60)).padStart(2, '0');
    const ss = String(rest % 60).padStart(2, '0');
    return d ? `${d}d ${hh}:${mm}:${ss}` : `${hh}:${mm}:${ss}`;
}

const monthDay = new Intl.DateTimeFormat('en-US', {month: 'short', day: 'numeric'});
const timeFmt = new Intl.DateTimeFormat('en-GB', {hour: '2-digit', minute: '2-digit', second: '2-digit'});
const hmFmt = new Intl.DateTimeFormat('en-GB', {hour: '2-digit', minute: '2-digit'});

export function fmtDay(t: string | Date) {
    return monthDay.format(new Date(t));
}

export function fmtRange(start: string, end: string) {
    return `${fmtDay(start)} – ${fmtDay(end)}`;
}

export function fmtTime(t: string | Date) {
    return timeFmt.format(new Date(t));
}

export function fmtHM(t: string | Date) {
    return hmFmt.format(new Date(t));
}

export function fmtViewers(n: number) {
    return n.toLocaleString('en-US');
}

export function pct(cur: number, req: number) {
    return req ? Math.min(100, Math.round(100 * cur / req)) : 0;
}

export function plural(n: number, word: string) {
    return `${n} ${word}${n === 1 ? '' : 's'}`;
}
