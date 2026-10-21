import {useMemo, useState} from 'react';
import {
    AudioLines, Bell, BellRing, Calendar, CircleCheck, ExternalLink, Eye, Gamepad2, Gift, Hourglass, Link, Link2Off,
    Lock, PackageOpen, Plus, RefreshCw, Search, Timer,
} from 'lucide-react';
import {ClaimDrop, RefreshCampaigns, WatchChannel} from '../../wailsjs/go/main/App';
import {models} from '../../wailsjs/go/models';
import {BrowserOpenURL} from '../../wailsjs/runtime/runtime';
import {useStore} from '../store';
import {campaignPhase, coverImage, dropState, DropState, isRust, parentCampaignId, Phase} from '../lib/derive';
import {fmtCountdown, fmtDuration, fmtHM, fmtMinutes, fmtRange, fmtViewers, isSet, pct} from '../lib/format';
import {Avatar, Badge, Banner, Button, Chip, Empty, GameIcon, Progress, Thumb} from '../components/ui';

const FACEPUNCH_URL = 'https://twitch.facepunch.com/';

const stateIcon: Record<DropState, JSX.Element> = {
    claimed: <CircleCheck size={16} color="var(--success-text)"/>,
    ready: <Gift size={16} color="var(--success-text)"/>,
    progress: <Timer size={16} color="var(--accent-text)"/>,
    locked: <Lock size={16} color="var(--text-tertiary)"/>,
    unlinked: <Link2Off size={16} color="var(--error-text)"/>,
};

export function DropTile({c, d, now, current}: { c: models.Campaign; d: models.Drop; now: number; current?: boolean }) {
    const state = dropState(c, d, now);
    const p = pct(d.currentMinutes, d.requiredMinutes);
    const rust = isRust(c.game);
    let right: JSX.Element;
    switch (state) {
        case 'claimed':
            right = <CircleCheck size={12} color="var(--success-text)"/>;
            break;
        case 'ready':
            right = <span style={{color: 'var(--success-text)', fontWeight: 600}}>Ready</span>;
            break;
        case 'locked':
            right = <span>Locked</span>;
            break;
        default:
            right = <span style={{color: current && rust ? 'var(--rust-text)' : 'var(--accent-text)', fontWeight: 600}}>{p}%</span>;
    }
    return (
        <div className={`drop-tile ${state} ${current ? 'current' : ''}`}>
            <div className="row" style={{justifyContent: 'space-between', alignItems: 'flex-start'}}>
                <Thumb src={d.imageUrl} size={56}/>
                {current ? <AudioLines size={16} color="var(--rust-text)"/> : stateIcon[state]}
            </div>
            <div>
                <div className="drop-tile-name">{d.name}</div>
                <div className="drop-tile-req">{fmtMinutes(d.requiredMinutes)} watch time</div>
            </div>
            <div>
                <Progress thin value={state === 'claimed' ? 100 : p}
                          tone={state === 'claimed' || state === 'ready' ? 'success' : current && rust ? 'rust' : undefined}/>
                <div className="drop-tile-meta">
                    <span>{state === 'claimed' ? 'Claimed' : `${fmtMinutes(d.currentMinutes)} / ${fmtMinutes(d.requiredMinutes)}`}</span>
                    {right}
                </div>
            </div>
            {state === 'ready' && d.instanceId && (
                <Button variant="success" size="sm" block icon={Gift} onClick={() => ClaimDrop(d.instanceId)}>Claim</Button>
            )}
        </div>
    );
}

function StreamerSection({c, now, watchingLogin}: { c: models.Campaign; now: number; watchingLogin?: string }) {
    const names = c.allowedChannels.map(ch => ch.displayName || ch.login).join(', ');
    const active = campaignPhase(c, now) === 'active';
    return (
        <div className="section sunken" style={{flexDirection: 'row', gap: 16}}>
            <div className="grow col" style={{gap: 10}}>
                <div className="row">
                    <span className="overline" style={{color: 'var(--rust-text)'}}>Streamer drop · only on</span>
                    <span className="faint" style={{fontSize: 12}}>progress counts only on these channels</span>
                </div>
                {c.allowedChannels.map(ch => {
                    const watching = ch.login.toLowerCase() === watchingLogin;
                    return (
                        <div key={ch.login} className={`streamer-row ${watching ? 'watching' : ''}`}>
                            <Avatar src={ch.avatarUrl} name={ch.displayName || ch.login} online={ch.online}/>
                            <div className="grow col" style={{gap: 1}}>
                                <span style={{fontWeight: 600}}>{ch.displayName || ch.login}</span>
                                <span className="muted" style={{fontSize: 12}}>
                                    {ch.online ? `Live · ${fmtViewers(ch.viewers)} viewers${ch.gameName ? ` · ${ch.gameName}` : ''}` : 'Offline'}
                                </span>
                            </div>
                            {watching
                                ? <Button variant="primary" size="sm" icon={AudioLines}>Watching</Button>
                                : <Button size="sm" icon={Eye} disabled={!ch.online || !active}
                                          onClick={() => WatchChannel(ch.login)}>Watch</Button>}
                        </div>
                    );
                })}
                <span className="faint" style={{fontSize: 12}}>
                    Only {names} carries the {c.drops?.map(d => d.name).join(', ')}. When it goes offline, pick another channel.
                </span>
            </div>
            <div style={{width: 156}}>
                {c.drops?.map(d => <DropTile key={d.id} c={c} d={d} now={now} current={watchingLogin !== undefined && c.allowedChannels.some(ch => ch.login.toLowerCase() === watchingLogin)}/>)}
            </div>
        </div>
    );
}

function LinkStatus({c}: { c: models.Campaign }) {
    if (c.source === 'facepunch') {
        return <span><Link size={13}/><a onClick={() => BrowserOpenURL(c.linkUrl)}>Steam must be linked</a></span>;
    }
    return c.accountLinked
        ? <span className="ok"><CircleCheck size={13}/>Account linked</span>
        : <span className="bad"><Link2Off size={13}/>Account not linked</span>;
}

/** Turns on "campaign started" notifications; they cover every tracked game. */
function NotifyButton() {
    const {settings, updateSettings} = useStore();
    const on = !!settings?.notify.campaignStarted;
    return on
        ? <Button variant="ghost" icon={BellRing} title="You get a desktop notification when tracked campaigns start">Notifying</Button>
        : <Button icon={Bell} onClick={() => updateSettings(c => { c.notify.campaignStarted = true; })}>Notify me</Button>;
}

function CampaignCard({c, children, now, watchingLogin}: {
    c: models.Campaign; children: models.Campaign[]; now: number; watchingLogin?: string;
}) {
    const phase = campaignPhase(c, now);
    const all = [c, ...children];
    const drops = all.flatMap(x => x.drops ?? []);
    const claimed = drops.filter(d => d.claimed).length;
    const general = c.allowedChannels ? null : c;
    const streamer = c.allowedChannels ? all : children;
    const siteUrl = c.source === 'facepunch' ? FACEPUNCH_URL : 'https://www.twitch.tv/drops/campaigns';

    return (
        <div className="card">
            <div className="campaign-head">
                <Thumb src={coverImage(c) || coverImage(children[0] ?? c)} size={44} className=""/>
                <div className="grow col" style={{gap: 6}}>
                    <div className="row" style={{gap: 8}}>
                        <span className="campaign-name">{c.name}</span>
                        {phase === 'active' && <Badge tone="success" dot>Active</Badge>}
                        {phase === 'upcoming' && <Badge tone="warning" dot>Upcoming</Badge>}
                        {phase === 'completed' && <Badge tone="success">Completed</Badge>}
                        {phase === 'ended' && <Badge>Ended</Badge>}
                        {c.source === 'facepunch' ? <Badge tone="rust" dot>Facepunch</Badge> : <Badge tone="accent" dot>Twitch</Badge>}
                        {!c.accountLinked && <Badge tone="error" dot>Not linked</Badge>}
                    </div>
                    <div className="meta-row">
                        <span><Calendar size={13}/>{fmtRange(c.startAt, c.endAt)}</span>
                        {phase === 'active' && <span><Hourglass size={13}/>ends in {fmtDuration(Date.parse(c.endAt) - now)}</span>}
                        {phase === 'upcoming' && <span><Gift size={13}/>{drops.length} drops</span>}
                        <LinkStatus c={c}/>
                        {phase !== 'upcoming' && <span><Gift size={13}/>{claimed} of {drops.length} claimed</span>}
                    </div>
                </div>
                {phase === 'upcoming' ? (
                    <>
                        <div className="countdown">
                            <div className="overline">Starts in</div>
                            <div className="mono">{fmtCountdown(Date.parse(c.startAt) - now)}</div>
                        </div>
                        <NotifyButton/>
                    </>
                ) : (
                    <Button variant="ghost" icon={ExternalLink} onClick={() => BrowserOpenURL(siteUrl)}>
                        {c.source === 'facepunch' ? 'twitch.facepunch.com' : 'twitch.tv/drops'}
                    </Button>
                )}
            </div>
            {phase !== 'upcoming' && (
                <>
                    {!c.accountLinked && (
                        <div style={{padding: '12px 16px 0'}}>
                            <Banner icon={Link2Off} title={`Your ${c.game.name} account isn't linked to Twitch`}
                                    action={c.linkUrl && <Button variant="primary" size="sm" icon={Link} onClick={() => BrowserOpenURL(c.linkUrl)}>Link account</Button>}>
                                Watch time still counts, but drops can't be claimed until you link it. Link on the publisher's site, then press Refresh.
                            </Banner>
                        </div>
                    )}
                    {general && (general.drops?.length ?? 0) > 0 && (
                        <div className="section">
                            <div className="row">
                                <span className="overline">{streamer.length ? 'General drops' : 'Drops'}</span>
                                <span className="faint" style={{fontSize: 12}}>any {c.game.name} stream with drops enabled</span>
                            </div>
                            <div className="drop-grid">
                                {general.drops.map(d => <DropTile key={d.id} c={c} d={d} now={now}/>)}
                            </div>
                        </div>
                    )}
                    {streamer.map(sc => <StreamerSection key={sc.id} c={sc} now={now} watchingLogin={watchingLogin}/>)}
                </>
            )}
        </div>
    );
}

type Filter = Exclude<Phase, 'ended'>;

export default function Campaigns() {
    const {snapshot, now, watch, setPage} = useStore();
    const [filter, setFilter] = useState<Filter>('active');
    const [query, setQuery] = useState('');
    const [game, setGame] = useState('');
    const [hideUnlinked, setHideUnlinked] = useState(false);
    const [refreshing, setRefreshing] = useState(false);
    const campaigns = snapshot?.campaigns ?? [];
    const watchingLogin = watch?.state === 'watching' ? watch.channel.login.toLowerCase() : undefined;

    // Facepunch streamer campaigns render inside their general campaign card.
    const {roots, childrenOf} = useMemo(() => {
        const ids = new Set(campaigns.map(c => c.id));
        const childrenOf = new Map<string, models.Campaign[]>();
        const roots: models.Campaign[] = [];
        for (const c of campaigns) {
            const parent = parentCampaignId(c);
            if (parent && ids.has(parent)) childrenOf.set(parent, [...(childrenOf.get(parent) ?? []), c]);
            else roots.push(c);
        }
        return {roots, childrenOf};
    }, [campaigns]);

    const phaseOf = (c: models.Campaign) => {
        const p = campaignPhase(c, now);
        return p === 'ended' ? 'completed' : p;
    };
    const counts = {active: 0, upcoming: 0, completed: 0};
    roots.forEach(c => counts[phaseOf(c)]++);
    const games = [...new Set(roots.map(c => c.game.name))];
    const q = query.trim().toLowerCase();
    const shown = roots
        .filter(c => phaseOf(c) === filter)
        .filter(c => !game || c.game.name === game)
        .filter(c => !hideUnlinked || c.accountLinked)
        .filter(c => !q || c.name.toLowerCase().includes(q) || c.game.name.toLowerCase().includes(q) ||
            [c, ...(childrenOf.get(c.id) ?? [])].some(x => x.drops?.some(d => d.name.toLowerCase().includes(q))))
        .sort((a, b) => filter === 'upcoming' ? Date.parse(a.startAt) - Date.parse(b.startAt) : Date.parse(a.endAt) - Date.parse(b.endAt));
    const byGame = new Map<string, models.Campaign[]>();
    shown.forEach(c => byGame.set(c.game.name, [...(byGame.get(c.game.name) ?? []), c]));
    const rustFirst = [...byGame.keys()].sort((a, b) => Number(b.toLowerCase() === 'rust') - Number(a.toLowerCase() === 'rust'));

    async function refresh() {
        setRefreshing(true);
        await RefreshCampaigns();
        setTimeout(() => setRefreshing(false), 1500);
    }

    const synced = snapshot && isSet(snapshot.updatedAt) ? ` · synced ${fmtHM(snapshot.updatedAt)}` : '';
    return (
        <>
            <div className="page-header">
                <div className="grow">
                    <h1 className="page-title">Campaigns</h1>
                    <div className="page-subtitle">{roots.length} campaigns for {games.length} tracked {games.length === 1 ? 'game' : 'games'}{synced}</div>
                </div>
                <label className="input" style={{width: 220}}>
                    <Search size={14}/>
                    <input placeholder="Search campaigns…" value={query} onChange={e => setQuery(e.target.value)}/>
                </label>
                <label className="input" style={{width: 'auto', paddingRight: 4, borderColor: 'var(--border-strong)'}}>
                    <Gamepad2 size={14} color="var(--text-primary)"/>
                    <select value={game} onChange={e => setGame(e.target.value)} style={{fontWeight: 600, cursor: 'pointer'}}>
                        <option value="">All games</option>
                        {games.map(g => <option key={g} value={g}>{g}</option>)}
                    </select>
                </label>
                <Button icon={RefreshCw} title="Refresh" onClick={refresh} className={refreshing ? 'refreshing' : ''}/>
            </div>
            <div className="toolbar">
                <Chip active={filter === 'active'} count={counts.active} onClick={() => setFilter('active')}>Active</Chip>
                <Chip active={filter === 'upcoming'} count={counts.upcoming} onClick={() => setFilter('upcoming')}>Upcoming</Chip>
                <Chip active={filter === 'completed'} count={counts.completed} onClick={() => setFilter('completed')}>Completed</Chip>
                <span className="sep"/>
                <Chip active={hideUnlinked} onClick={() => setHideUnlinked(v => !v)}>Hide unlinked</Chip>
                <span className="grow"/>
                <span className="faint" style={{fontSize: 12}}>Sort: {filter === 'upcoming' ? 'starts soonest' : 'ends soonest'}</span>
            </div>
            {snapshot?.warnings?.map(w => <Banner key={w} icon={Link2Off} title="A campaign source failed">{w}</Banner>)}
            {shown.length === 0 ? (
                <div className="card" style={{flex: 1, display: 'flex'}}>
                    <Empty icon={PackageOpen} title={roots.length ? `No ${filter} campaigns` : 'No campaigns right now'}
                           actions={<>
                               <Button variant="primary" icon={Plus} onClick={() => setPage('settings')}>Add games</Button>
                               <Button icon={ExternalLink} onClick={() => BrowserOpenURL(FACEPUNCH_URL)}>twitch.facepunch.com</Button>
                           </>}>
                        Twitch only lists campaigns you already have progress in; Rust campaigns come from twitch.facepunch.com.
                        Facepunch usually announces Rust drops a few days ahead.
                    </Empty>
                </div>
            ) : rustFirst.map(g => {
                const list = byGame.get(g)!;
                const active = list.filter(c => campaignPhase(c, now) === 'active').length;
                return (
                    <div key={g} className="col" style={{gap: 12}}>
                        <div className="game-group">
                            <GameIcon name={g}/>
                            <span className="game-group-name">{g}</span>
                            <span className="faint" style={{fontSize: 12}}>
                                {list[0].source === 'facepunch' ? 'Facepunch · ' : ''}{active ? `${active} active` : `${list.length} ${filter}`}
                            </span>
                        </div>
                        {list.map(c => <CampaignCard key={c.id} c={c} children={childrenOf.get(c.id) ?? []} now={now} watchingLogin={watchingLogin}/>)}
                    </div>
                );
            })}
        </>
    );
}
