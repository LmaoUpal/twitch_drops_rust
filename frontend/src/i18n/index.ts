import {en, Key} from './en';
import {ru} from './ru';

export type Lang = 'en' | 'ru';
export type {Key};

const dicts: Record<Lang, Record<Key, string>> = {en, ru};
const locales: Record<Lang, string> = {en: 'en-US', ru: 'ru-RU'};

let current: Lang = 'en';

/** Selects the UI language. The store calls this on every render, so t() always matches settings. */
export function setLang(lang: string) {
    current = lang === 'ru' ? 'ru' : 'en';
}

export function lang(): Lang {
    return current;
}

/** BCP 47 locale for Intl formatters. */
export function locale() {
    return locales[current];
}

function fill(s: string, vars?: Record<string, string | number>) {
    return vars ? s.replace(/\{(\w+)\}/g, (m, k) => (k in vars ? String(vars[k]) : m)) : s;
}

/** Translates key, filling {placeholders} from vars. */
export function t(key: Key, vars?: Record<string, string | number>) {
    return fill(dicts[current][key] ?? en[key], vars);
}

/** Translates a plural key ("one|other" in English, "one|few|many" in Russian) for count n. */
export function tn(key: Key, n: number, vars?: Record<string, string | number>) {
    const forms = (dicts[current][key] ?? en[key]).split('|');
    let i = 0;
    if (current === 'ru') {
        const m10 = n % 10, m100 = n % 100;
        i = m10 === 1 && m100 !== 11 ? 0 : m10 >= 2 && m10 <= 4 && (m100 < 12 || m100 > 14) ? 1 : 2;
    } else {
        i = n === 1 ? 0 : 1;
    }
    return fill(forms[Math.min(i, forms.length - 1)], {n, ...vars});
}

/** Translates a reason label sent by the backend ("streamer drop", …), falling back to the raw text. */
export function tReason(reason: string) {
    const key = `reason.${reason}` as Key;
    return key in en ? t(key) : reason;
}
