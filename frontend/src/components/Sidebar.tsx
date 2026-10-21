import {ChevronsUpDown, Gift, LayoutDashboard, LucideIcon, Package, Pickaxe, ScrollText, Settings} from 'lucide-react';
import {Page, useStore} from '../store';
import {campaignPhase, findDrop, isRust} from '../lib/derive';
import {fmtMinutes} from '../lib/format';
import {Avatar, Progress} from './ui';

export const APP_VERSION = 'v0.4.0';

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
    const {watch, snapshot} = useStore();
    const campaigns = snapshot?.campaigns ?? [];
    const state = watch?.state ?? 'idle';
    const ch = watch?.channel;
    const s = watch?.session;
    const found = s?.dropId ? findDrop(campaigns, s.dropId) : null;
    const name = ch?.displayName || ch?.login || '';

    let dot = '', title = 'Idle', detail = 'Not watching anything', foot = 'Pick a channel to start';
    if (state === 'watching' || state === 'starting') {
        dot = 'success';
        title = state === 'starting' ? 'Starting…' : 'Mining';
        detail = found ? `${name} · ${found.drop.name}` : name;
        foot = s?.requiredMinutes
            ? `${s.currentMinutes} / ${s.requiredMinutes} min · ~${fmtMinutes(s.requiredMinutes - s.currentMinutes)} left`
            : 'waiting for drop progress';
    } else if (state === 'offline') {
        dot = 'warning';
        title = 'Channel offline';
        detail = `${name} went offline`;
    } else if (state === 'error') {
        dot = 'error';
        title = 'Stopped';
        detail = watch?.error || 'Watching failed';
    }
    const value = s?.requiredMinutes ? (100 * s.currentMinutes) / s.requiredMinutes : 0;
    const rust = found ? isRust(found.campaign.game) : isRust({id: ch?.gameId ?? '', name: ch?.gameName ?? '', slug: ''});

    return (
        <div className="status-card">
            <div className="status-head">
                <span className={`dot ${dot}`}/>
                {title}
                <span className="status-mode">MANUAL</span>
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
                    <div className="brand-version">{APP_VERSION} · for Twitch</div>
                </div>
            </div>
            <NavItem page="dashboard" icon={LayoutDashboard} label="Dashboard"/>
            <NavItem page="campaigns" icon={Gift} label="Campaigns" count={activeCount}/>
            <NavItem page="inventory" icon={Package} label="Inventory"/>
            <NavItem page="activity" icon={ScrollText} label="Activity log"/>
            <div className="grow"/>
            <StatusCard/>
            <NavItem page="settings" icon={Settings} label="Settings"/>
            <div className="account">
                <Avatar src={auth.avatarUrl} name={auth.login} size={24}/>
                <span className="grow ellipsis">{auth.displayName || auth.login}</span>
                <ChevronsUpDown size={14} className="faint"/>
            </div>
        </aside>
    );
}
