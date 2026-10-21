import {ChevronsUpDown, Compass, Gift, LayoutDashboard, LucideIcon, Package, Pickaxe, ScrollText, Settings} from 'lucide-react';
import {Page, useStore} from '../store';
import {campaignPhase, findDrop, isRust} from '../lib/derive';
import {fmtCountdown, fmtMinutes, isSet} from '../lib/format';
import {Avatar, Progress} from './ui';
import {t} from '../i18n';

export const APP_VERSION = 'v0.5.0';

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
    const {auth, snapshot, now} = useStore();
    const activeCount = (snapshot?.campaigns ?? []).filter(c => campaignPhase(c, now) === 'active').length;
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
            <NavItem page="inventory" icon={Package} label={t('nav.inventory')}/>
            <NavItem page="activity" icon={ScrollText} label={t('nav.activity')}/>
            <div className="grow"/>
            <StatusCard/>
            <NavItem page="settings" icon={Settings} label={t('nav.settings')}/>
            <div className="account">
                <Avatar src={auth.avatarUrl} name={auth.login} size={24}/>
                <span className="grow ellipsis">{auth.displayName || auth.login}</span>
                <ChevronsUpDown size={14} className="faint"/>
            </div>
        </aside>
    );
}
