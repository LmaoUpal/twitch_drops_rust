import {useRef, useState} from 'react';
import {Check, ChevronsUpDown, Coins, Compass, Gift, LayoutDashboard, LucideIcon, Package, Pickaxe, ScrollText, Settings, UserPlus} from 'lucide-react';
import {SwitchAccount} from '../../wailsjs/go/main/App';
import {Page, useStore} from '../store';
import {campaignPhase, findDrop, isRust} from '../lib/derive';
import {fmtCountdown, fmtMinutes, isSet} from '../lib/format';
import {Avatar, Progress, useDismiss} from './ui';
import {t} from '../i18n';

export const APP_VERSION = 'v0.6.0';

function NavItem({page, icon: Icon, label, count}: { page: Page; icon: LucideIcon; label: string; count?: number }) {
    const {page: current, setPage} = useStore();
    return (
        <button className={`nav-item ${current === page ? 'active' : ''}`} onClick={() => setPage(page)}>
            <Icon size={16}/>
            {label}
            {!!count && <span className="count">{count}</span>}
        </button>
    );
}

/** The account row; opens a switcher with the saved logins and "Add account". */
function AccountMenu() {
    const {auth, accounts, setAddingAccount} = useStore();
    const [open, setOpen] = useState(false);
    const ref = useRef<HTMLDivElement>(null);
    useDismiss(ref, open, () => setOpen(false));
    return (
        <div className="account-switch" ref={ref}>
            {open && (
                <div className="select-menu account-menu" role="menu">
                    {accounts.map(a => (
                        <button key={a.key} role="menuitem" className={a.active ? 'selected' : ''}
                                onClick={() => { setOpen(false); if (!a.active) SwitchAccount(a.key); }}>
                            <Avatar src={a.avatarUrl} name={a.login} size={20}/>
                            <span className="grow ellipsis">{a.displayName || a.login}</span>
                            {a.active && <Check size={14}/>}
                        </button>
                    ))}
                    <div className="menu-sep"/>
                    <button role="menuitem" onClick={() => { setOpen(false); setAddingAccount(true); }}>
                        <UserPlus size={14}/>
                        <span className="grow">{t('acc.add')}</span>
                    </button>
                </div>
            )}
            <button className="account" onClick={() => setOpen(v => !v)} aria-expanded={open} title={t('acc.switchHint')}>
                <Avatar src={auth.avatarUrl} name={auth.login} size={24}/>
                <span className="grow ellipsis">{auth.displayName || auth.login}</span>
                <ChevronsUpDown size={14} className="faint"/>
            </button>
        </div>
    );
}

function StatusCard() {
    const {watch, snapshot, auto, now} = useStore();
    const campaigns = snapshot?.campaigns ?? [];
    const state = watch?.state ?? 'idle';
    const ch = watch?.channel;
    const s = watch?.session;
    const found = s?.dropId ? findDrop(campaigns, s.dropId) : null;
    const name = ch?.displayName || ch?.login || '';

    const isAuto = auto?.mode === 'auto';
    let dot = '', title = t('status.idle'), detail = isAuto ? t('status.nothingLive') : t('status.notWatching');
    let foot = isAuto && isSet(auto.nextCheck)
        ? t('status.nextCheck', {t: fmtCountdown(Date.parse(auto.nextCheck) - now).replace(/^00:/, '')})
        : t('status.pickChannel');
    if (isAuto && auto.paused) {
        [title, detail, foot] = [t('status.paused'), t('status.pausedDetail'), t('status.resumeHint')];
    }
    if (state === 'watching' || state === 'starting') {
        dot = 'success';
        title = state === 'starting' ? t('status.starting') : auto?.state === 'exploring' ? t('status.exploring', {game: auto.exploring}) : t('status.mining');
        detail = found ? `${name} · ${found.drop.name}` : name;
        foot = s?.requiredMinutes
            ? t('status.progress', {cur: s.currentMinutes, req: s.requiredMinutes, left: fmtMinutes(s.requiredMinutes - s.currentMinutes)})
            : t('status.waiting');
    } else if (state === 'offline') {
        dot = 'warning';
        title = t('status.offline');
        detail = t('status.wentOffline', {name});
    } else if (state === 'error') {
        dot = 'error';
        title = t('status.stopped');
        detail = watch?.error || t('status.failed');
    }
    const value = s?.requiredMinutes ? (100 * s.currentMinutes) / s.requiredMinutes : 0;
    const rust = found ? isRust(found.campaign.game) : isRust({id: ch?.gameId ?? '', name: ch?.gameName ?? '', slug: ''});

    return (
        <div className="status-card">
            <div className="status-head">
                <span className={`dot ${dot}`}/>
                {title}
                <span className="status-mode">{isAuto ? t('status.auto') : t('status.manual')}</span>
            </div>
            <div className="muted ellipsis" style={{fontSize: 12}}>{detail}</div>
            {(state === 'watching' || state === 'starting') && <Progress thin value={value} tone={value >= 100 ? 'success' : rust ? 'rust' : undefined}/>}
            <div className="faint" style={{fontSize: 11}}>{foot}</div>
        </div>
    );
}

export function Sidebar() {
    const {auth, snapshot, points, now} = useStore();
    const activeCount = (snapshot?.campaigns ?? []).filter(c => campaignPhase(c, now) === 'active').length;
    const pointsWatching = (points?.channels ?? []).filter(c => c.state === 'watching').length;
    return (
        <aside className="sidebar">
            <div className="brand">
                <div className="brand-mark"><Pickaxe size={16}/></div>
                <div>
                    <div className="brand-name">Drops Miner</div>
                    <div className="brand-version">{t('nav.forTwitch', {v: APP_VERSION})}</div>
                </div>
            </div>
            <NavItem page="dashboard" icon={LayoutDashboard} label={t('nav.dashboard')}/>
            <NavItem page="campaigns" icon={Gift} label={t('nav.campaigns')} count={activeCount}/>
            <NavItem page="drops" icon={Compass} label={t('nav.allDrops')}/>
            <NavItem page="points" icon={Coins} label={t('nav.points')} count={pointsWatching}/>
            <NavItem page="inventory" icon={Package} label={t('nav.inventory')}/>
            <NavItem page="activity" icon={ScrollText} label={t('nav.activity')}/>
            <div className="grow"/>
            <StatusCard/>
            <NavItem page="settings" icon={Settings} label={t('nav.settings')}/>
            <AccountMenu/>
        </aside>
    );
}
