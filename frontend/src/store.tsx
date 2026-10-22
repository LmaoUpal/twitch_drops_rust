import {createContext, ReactNode, useContext, useEffect, useRef, useState} from 'react';
import {GetAccounts, GetActivity, GetAuthStatus, GetAutoState, GetCampaigns, GetPointsState, GetSettings, GetWatchState, SaveSettings} from "../wailsjs/go/main/App";
import {activity, catalog, config, main} from "../wailsjs/go/models";
import {EventsOn} from "../wailsjs/runtime/runtime";
import {setLang} from "./i18n";

export type Page = 'dashboard' | 'campaigns' | 'drops' | 'points' | 'inventory' | 'activity' | 'settings';

interface Store {
    auth: main.AuthStatus;
    accounts: main.AccountInfo[];
    /** The login screen is open to add another account while one is signed in. */
    addingAccount: boolean;
    setAddingAccount(v: boolean): void;
    snapshot: catalog.Snapshot | null;
    watch: main.WatchState | null;
    auto: main.AutoState | null;
    points: main.PointsState | null;
    settings: config.Config | null;
    activity: activity.Entry[];
    now: number;
    page: Page;
    setPage(p: Page): void;
    updateSettings(change: (c: config.Config) => void): void;
    settingsError: string;
}

const Ctx = createContext<Store | null>(null);

export function useStore() {
    const s = useContext(Ctx);
    if (!s) throw new Error('useStore outside StoreProvider');
    return s;
}

/** Re-render every `ms` milliseconds; used for countdowns and relative times. */
function useNow(ms: number) {
    const [now, setNow] = useState(Date.now());
    useEffect(() => {
        const t = setInterval(() => setNow(Date.now()), ms);
        return () => clearInterval(t);
    }, [ms]);
    return now;
}

/** Sets data-theme on <html>; "system" follows the OS light/dark mode live. */
function useTheme(theme: string) {
    useEffect(() => {
        const root = document.documentElement;
        if (theme !== 'system') {
            root.dataset.theme = theme === 'light' ? 'light' : 'dark';
            return;
        }
        const mq = window.matchMedia('(prefers-color-scheme: light)');
        const apply = () => { root.dataset.theme = mq.matches ? 'light' : 'dark'; };
        apply();
        mq.addEventListener('change', apply);
        return () => mq.removeEventListener('change', apply);
    }, [theme]);
}

export function StoreProvider({children}: { children: ReactNode }) {
    const [auth, setAuth] = useState<main.AuthStatus>({state: 'checking', login: '', displayName: '', avatarUrl: '', error: ''});
    const [accounts, setAccounts] = useState<main.AccountInfo[]>([]);
    const [addingAccount, setAddingAccount] = useState(false);
    const [snapshot, setSnapshot] = useState<catalog.Snapshot | null>(null);
    const [watch, setWatch] = useState<main.WatchState | null>(null);
    const [auto, setAuto] = useState<main.AutoState | null>(null);
    const [points, setPoints] = useState<main.PointsState | null>(null);
    const [settings, setSettings] = useState<config.Config | null>(null);
    const [log, setLog] = useState<activity.Entry[]>([]);
    const [page, setPage] = useState<Page>('dashboard');
    const [settingsError, setSettingsError] = useState('');
    const saveTimer = useRef<number | undefined>(undefined);
    const now = useNow(1000);

    useEffect(() => {
        GetAuthStatus().then(setAuth);
        GetAccounts().then(a => setAccounts(a ?? []));
        GetCampaigns().then(setSnapshot);
        GetWatchState().then(setWatch);
        GetAutoState().then(setAuto);
        GetPointsState().then(setPoints);
        GetSettings().then(setSettings);
        GetActivity().then(e => setLog(e ?? []));
        const offs = [
            EventsOn('auth:status', setAuth),
            EventsOn('accounts:update', (a: main.AccountInfo[]) => setAccounts(a ?? [])),
            EventsOn('campaigns:update', setSnapshot),
            EventsOn('watch:status', setWatch),
            EventsOn('auto:status', setAuto),
            EventsOn('points:status', setPoints),
            EventsOn('activity:entry', (e: activity.Entry) => setLog(l => [...l.slice(-999), e])),
        ];
        return () => offs.forEach(off => off());
    }, []);

    // A finished login (or a switch) lands on the new account, so the add screen closes.
    useEffect(() => setAddingAccount(false), [auth.login]);

    // Settings save automatically, debounced so typing doesn't hit the disk per key.
    function updateSettings(change: (c: config.Config) => void) {
        setSettings(prev => {
            if (!prev) return prev;
            const next = config.Config.createFrom(JSON.parse(JSON.stringify(prev)));
            change(next);
            window.clearTimeout(saveTimer.current);
            saveTimer.current = window.setTimeout(() => {
                SaveSettings(next).then(() => setSettingsError(''), e => setSettingsError(String(e)));
            }, 400);
            return next;
        });
    }

    useTheme(settings?.theme ?? 'dark');
    setLang(settings?.language ?? 'en');
    return (
        <Ctx.Provider value={{auth, accounts, addingAccount, setAddingAccount, snapshot, watch, auto, points, settings, activity: log, now, page, setPage, updateSettings, settingsError}}>
            {children}
        </Ctx.Provider>
    );
}
