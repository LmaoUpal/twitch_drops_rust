import {useState} from 'react';
import {
    ArrowLeftRight, CircleX, ExternalLink, Eye, Gift, Hand, Hourglass, MoonStar, Pause, Play, RadioTower,
    RefreshCw, Send, Sparkles, Square, Timer, User, WifiOff, X,
} from 'lucide-react';
import {ClaimDrop, RefreshCampaigns, SetAutoMode, SetPaused, StopWatching, WatchChannel} from '../../wailsjs/go/main/App';
import {main, models} from '../../wailsjs/go/models';
import {BrowserOpenURL} from '../../wailsjs/runtime/runtime';
import {useStore} from '../store';
import {campaignPhase, findDrop, isRust, parentCampaignId, upNext} from '../lib/derive';
import {fmtCountdown, fmtDuration, fmtHM, fmtMinutes, fmtTime, fmtViewers, isSet, pct} from '../lib/format';
import {Avatar, Badge, Banner, Button, Progress, Ring, Segmented, Thumb} from '../components/ui';

function WatchForm({onDone, autoFocus}: { onDone?(): void; autoFocus?: boolean }) {
    const [login, setLogin] = useState('');
    const [error, setError] = useState('');
    async function go() {
        setError('');
        try {
            await WatchChannel(login);
            onDone?.();
        } catch (e) {
            setError(String(e));
        }
    }
    return (
        <div className="col" style={{gap: 6}}>
            <div className="inline-form">
                <label className="input" style={{width: 220}}>
                    <User size={14}/>
                    <input autoFocus={autoFocus} placeholder="channel login" value={login}
                           onChange={e => setLogin(e.target.value)} onKeyDown={e => e.key === 'Enter' && login && go()}/>
                </label>
                <Button variant="primary" icon={Eye} disabled={!login} onClick={go}>Watch</Button>
                {onDone && <Button variant="ghost" icon={X} onClick={onDone}/>}
            </div>
            <span className="faint" style={{fontSize: 11}}>Picking a channel yourself switches to Manual mode.</span>
            {error && <span style={{color: 'var(--error-text)', fontSize: 12}}>{error}</span>}
        </div>
    );
}

function Header({title, subtitle}: { title: string; subtitle: string }) {
    const {auto} = useStore();
    const isAuto = auto?.mode === 'auto';
    return (
        <div className="page-header">
            <div className="grow">
                <h1 className="page-title">{title}</h1>
                <div className="page-subtitle">{subtitle}</div>
            </div>
            <Segmented value={isAuto ? 'auto' : 'manual'} onChange={v => SetAutoMode(v === 'auto')} options={[
                {value: 'auto', label: 'Auto', icon: Sparkles},
                {value: 'manual', label: 'Manual', icon: Hand},
            ]}/>
            {isAuto && (auto.paused
                ? <Button variant="primary" icon={Play} onClick={() => SetPaused(false)}>Resume</Button>
                : <Button icon={Pause} onClick={() => SetPaused(true)}>Pause</Button>)}
        </div>
    );
}

function Hero({watch, campaigns}: { watch: main.WatchState; campaigns: models.Campaign[] }) {
    const {now, settings, auto} = useStore();
    const [switching, setSwitching] = useState(false);
    const [claimError, setClaimError] = useState('');
    const ch = watch.channel;
    const offline = watch.state === 'offline';
    const s = watch.session;
    const found = s?.dropId ? findDrop(campaigns, s.dropId) : null;
    const rust = found ? isRust(found.campaign.game) : ch.gameId === '263490';
    const ready = s.requiredMinutes > 0 && s.currentMinutes >= s.requiredMinutes;
    const instance = found?.drop.instanceId;
    const left = s?.requiredMinutes ? s.requiredMinutes - s.currentMinutes : 0;
    const also = campaigns
        .filter(c => campaignPhase(c, now) === 'active' && c.game.id === ch.gameId)
        .flatMap(c => c.drops ?? [])
        .filter(d => !d.claimed && d.currentMinutes > 0 && d.id !== s?.dropId)
        .slice(0, 3);
    const st = watch.status;
    const name = ch.displayName || ch.login;
    const watchedFor = isSet(watch.started) ? fmtDuration(now - Date.parse(watch.started)) : '';
    const exploring = auto?.state === 'exploring' ? auto.exploring : '';
    const autoRunning = auto?.mode === 'auto' && !auto.paused;

    async function claim() {
        setClaimError('');
        try {
            await ClaimDrop(instance!);
        } catch (e) {
            setClaimError(String(e));
        }
    }

    return (
        <div className="card">
            <div className="hero-stream">
                <div className={`hero-avatar ${offline ? 'offline' : ''}`}>
                    <Avatar src={ch.avatarUrl} name={name} size={48}/>
                </div>
                <div className="grow col" style={{gap: 5}}>
                    <div className="row" style={{gap: 10}}>
                        <span className="stream-name" style={offline ? {color: 'var(--text-secondary)'} : undefined}>{name}</span>
                        {offline ? <Badge>OFFLINE</Badge> : <span className="badge live">LIVE</span>}
                        {!offline && ch.viewers > 0 && (
                            <span className="row muted" style={{gap: 4, fontWeight: 500}}>
                                <User size={13} color="var(--live)"/>{fmtViewers(ch.viewers)}
                            </span>
                        )}
                        {offline && <span className="row muted" style={{gap: 4}}><WifiOff size={13}/>stream ended</span>}
                    </div>
                    <div className="stream-title">
                        {offline ? (autoRunning ? 'Channel went offline. Picking the next channel…' : 'Channel went offline. Pick another channel to keep earning.') : ch.title}
                    </div>
                    <div className="row" style={{gap: 6}}>
                        {ch.gameName && <Badge tone={rust ? 'rust' : 'accent'} dot>{ch.gameName}</Badge>}
                        {exploring && <Badge tone="warning">Exploring</Badge>}
                        {!exploring && watch.reason && <Badge tone={watch.reason === 'streamer drop' ? 'accent' : 'neutral'}>{watch.reason[0].toUpperCase() + watch.reason.slice(1)}</Badge>}
                        {!watch.reason && <Badge>Manual pick</Badge>}
                        {watchedFor && !offline && <span className="faint" style={{fontSize: 12}}>· watching for {watchedFor}</span>}
                    </div>
                </div>
                <div className="row">
                    <Button icon={Square} onClick={() => StopWatching()} title={auto?.mode === 'auto' ? 'Stops and pauses auto mode' : undefined}>Stop</Button>
                    <Button icon={ArrowLeftRight} onClick={() => setSwitching(v => !v)}>Switch channel</Button>
                    <Button icon={ExternalLink} title="Open on Twitch" onClick={() => BrowserOpenURL(`https://www.twitch.tv/${ch.login}`)}/>
                </div>
            </div>
            {switching && (
                <div style={{padding: '14px 20px', borderBottom: '1px solid var(--border)'}}>
                    <WatchForm autoFocus onDone={() => setSwitching(false)}/>
                </div>
            )}
            <div className="hero-drop">
                <Ring value={s?.requiredMinutes ? pct(s.currentMinutes, s.requiredMinutes) : 0}
                      color={ready ? 'var(--success)' : rust ? 'var(--rust)' : 'var(--accent)'}>
                    <span className="ring-value">{s?.requiredMinutes ? s.currentMinutes : '–'}</span>
                    <span className="ring-unit">{s?.requiredMinutes ? `of ${s.requiredMinutes} min` : 'no drop yet'}</span>
                </Ring>
                <div className="grow col" style={{gap: 14}}>
                    <span className="overline">{ready ? 'Current drop · complete' : 'Current drop'}</span>
                    {found ? (
                        <div className="row" style={{gap: 14}}>
                            <Thumb src={found.drop.imageUrl} size={56}/>
                            <div className="col" style={{gap: 2}}>
                                <span className="drop-name">{found.drop.name}</span>
                                <span className="muted" style={{fontSize: 12}}>
                                    {found.campaign.name}
                                    {found.campaign.allowedChannels ? ` · only on ${found.campaign.allowedChannels.map(c => c.displayName || c.login).join(', ')}` : ''}
                                </span>
                            </div>
                        </div>
                    ) : (
                        <span className="muted">
                            {exploring
                                ? `No known ${exploring} campaign yet. If this stream carries drops, Twitch starts reporting progress within a few minutes.`
                                : s?.requiredMinutes ? 'Twitch reports a drop for this channel; the campaign list catches up on the next refresh.' : 'Twitch reports drop progress a minute or two after watching starts.'}
                        </span>
                    )}
                    {s.requiredMinutes > 0 && (ready ? (
                        <div className="col" style={{gap: 10}}>
                            <div className="row" style={{color: 'var(--success-text)', fontWeight: 600}}>
                                <Gift size={15}/> Ready to claim
                                <span className="muted" style={{fontWeight: 400}}>{settings?.autoClaim ? '· auto-claim is on' : ''}</span>
                            </div>
                            {instance ? (
                                <div className="row" style={{gap: 12}}>
                                    <Button variant="success" icon={Gift} onClick={claim}>Claim {found?.drop.name}</Button>
                                    {settings?.autoClaim && <span className="muted">or let auto-claim do it</span>}
                                </div>
                            ) : <span className="muted" style={{fontSize: 12}}>Waiting for Twitch to issue the claim — the inventory is being refreshed.</span>}
                            {claimError && <span style={{color: 'var(--error-text)', fontSize: 12}}>{claimError}</span>}
                        </div>
                    ) : (
                        <div className="row">
                            <Hourglass size={15} className="muted"/>
                            <span style={{fontSize: 14, fontWeight: 600}}>~{fmtMinutes(left)} left</span>
                            <span className="muted">· ready ≈ {fmtHM(new Date(now + left * 60000))}{settings?.autoClaim ? ' · auto-claim on' : ''}</span>
                        </div>
                    ))}
                </div>
                {also.length > 0 && (
                    <>
                        <div className="vdivider"/>
                        <div className="col" style={{gap: 12, width: 236}}>
                            <span className="overline">Also progressing</span>
                            {also.map(d => (
                                <div key={d.id} className="row" style={{gap: 10}}>
                                    <Thumb src={d.imageUrl} size={32}/>
                                    <div className="grow col" style={{gap: 5}}>
                                        <div className="row" style={{justifyContent: 'space-between'}}>
                                            <span className="ellipsis" style={{fontSize: 12, fontWeight: 500}}>{d.name}</span>
                                            <span className="faint" style={{fontSize: 11, whiteSpace: 'nowrap', flexShrink: 0}}>{fmtMinutes(d.currentMinutes)} / {fmtMinutes(d.requiredMinutes)}</span>
                                        </div>
                                        <Progress thin value={pct(d.currentMinutes, d.requiredMinutes)}/>
                                    </div>
                                </div>
                            ))}
                        </div>
                    </>
                )}
            </div>
            <div className="tech-stats">
                <span><RadioTower size={12}/>segments requested {fmtViewers(st?.segments ?? 0)}</span>
                <span><Send size={12}/>minute-watched sent {st?.spadeEvents ?? 0}</span>
                {isSet(st?.lastPoll) && <span><RefreshCw size={12}/>updated {fmtTime(st.lastPoll)}</span>}
                {autoRunning && isSet(auto!.nextCheck)
                    ? <span><Timer size={12}/>next re-rank {fmtCountdown(Date.parse(auto!.nextCheck) - now).replace(/^00:/, '')}</span>
                    : <span><Timer size={12}/>polling every 10 s</span>}
            </div>
        </div>
    );
}

function QueueRow({index, channel, sub, reason, eta, skipped, onWatch}: {
    index: number; channel: models.Channel; sub: string; reason: string; eta?: string; skipped?: boolean; onWatch?(): void;
}) {
    const tone = skipped ? 'error' : reason === 'ends soonest' ? 'warning' : reason === 'streamer drop' ? 'accent' : 'neutral';
    return (
        <div className={`queue-row ${skipped ? 'skipped' : ''}`}>
            <span className="queue-index">{index}</span>
            <Avatar src={channel.avatarUrl} name={channel.displayName || channel.login} online={!skipped}/>
            <div className="grow col" style={{gap: 1}}>
                <span style={{fontWeight: 600}}>{channel.displayName || channel.login}</span>
                <span className="muted ellipsis" style={{fontSize: 12}}>{sub}</span>
            </div>
            <Badge tone={tone}>{skipped ? 'skipped' : reason}</Badge>
            {eta !== undefined && <span className="queue-eta">{eta}</span>}
            {onWatch && <Button size="sm" icon={Eye} disabled={skipped} onClick={onWatch}>Watch</Button>}
        </div>
    );
}

function UpNext({campaigns, current}: { campaigns: models.Campaign[]; current?: string }) {
    const {now, auto} = useStore();
    if (auto?.mode === 'auto') {
        const q = auto.queue ?? [];
        if (!q.length || auto.paused) return null;
        return (
            <div className="col" style={{gap: 6}}>
                <div className="row" style={{paddingBottom: 4}}>
                    <span className="grow" style={{fontSize: 15, fontWeight: 600}}>Up next</span>
                    <span className="faint" style={{fontSize: 12}}>Re-ranked every minute</span>
                </div>
                {q.map((it, i) => (
                    <QueueRow key={it.channel.login} index={i + 1} channel={it.channel} reason={it.reason}
                              sub={`${it.gameName}${it.drop ? ` · ${it.drop} ${fmtMinutes(it.dropCurrent)} / ${fmtMinutes(it.dropNeeded)}` : ''}`}
                              eta={it.minutesLeft >= 0 ? `~${fmtMinutes(it.minutesLeft)}` : '—'}/>
                ))}
            </div>
        );
    }
    const items = upNext(campaigns, now, current);
    if (!items.length) return null;
    return (
        <div className="col" style={{gap: 6}}>
            <div className="row" style={{paddingBottom: 4}}>
                <span className="grow" style={{fontSize: 15, fontWeight: 600}}>Live streamer-drop channels</span>
            </div>
            {items.map((it, i) => (
                <QueueRow key={it.channel.login} index={i + 1} channel={it.channel} reason={it.reason} skipped={!!it.skipped}
                          sub={`${it.campaign.game.name}${it.drop ? ` · ${it.drop.name} ${fmtMinutes(it.drop.currentMinutes)} / ${fmtMinutes(it.drop.requiredMinutes)}` : ''}${it.skipped ? ` · ${it.skipped}` : ''}`}
                          onWatch={() => WatchChannel(it.channel.login)}/>
            ))}
        </div>
    );
}

function Idle({campaigns}: { campaigns: models.Campaign[] }) {
    const {now, auto, setPage} = useStore();
    const isAuto = auto?.mode === 'auto';
    const paused = isAuto && auto.paused;
    const upcoming = campaigns.filter(c => campaignPhase(c, now) === 'upcoming' && !parentCampaignId(c))
        .sort((a, b) => Date.parse(a.startAt) - Date.parse(b.startAt)).slice(0, 3);
    const nextCheck = isAuto && isSet(auto.nextCheck) ? fmtCountdown(Date.parse(auto.nextCheck) - now).replace(/^00:/, '') : '';
    const games = [...new Set(campaigns.map(c => c.game.name))];

    return (
        <div className="card empty" style={{minHeight: 0, padding: '48px 32px'}}>
            <div className="empty-orb"><div>{paused ? <Pause size={22}/> : <MoonStar size={22}/>}</div></div>
            <h3>{paused ? 'Mining is paused' : isAuto ? 'No live channels have drops for your games' : 'Not watching anything'}</h3>
            <p>
                {paused ? 'Resume to let auto mode pick a channel again.'
                    : isAuto ? `Checked ${campaigns.length} campaigns${games.length ? ` across ${games.join(' and ')}` : ''}. We'll look again${nextCheck ? ` in ${nextCheck}` : ' shortly'} and start watching as soon as something goes live.`
                        : 'Pick a live channel from the list below or from Campaigns, or type a channel login.'}
            </p>
            <div className="row" style={{marginTop: 8}}>
                {paused
                    ? <Button variant="primary" icon={Play} onClick={() => SetPaused(false)}>Resume mining</Button>
                    : <Button variant="primary" icon={Gift} onClick={() => setPage('campaigns')}>Browse campaigns</Button>}
                {isAuto && !paused && <Button icon={RefreshCw} onClick={() => RefreshCampaigns()}>Check now</Button>}
            </div>
            {!isAuto && <div style={{marginTop: 12}}><WatchForm/></div>}
            {upcoming.length > 0 && (
                <div className="card" style={{marginTop: 16, width: 440, textAlign: 'left'}}>
                    {upcoming.map(c => (
                        <div key={c.id} className="row" style={{gap: 12, padding: '10px 14px'}}>
                            <Thumb src={c.drops?.[0]?.imageUrl} size={32}/>
                            <div className="grow col">
                                <span style={{fontWeight: 500}} className="ellipsis">{c.name}</span>
                                <span className="muted" style={{fontSize: 12}}>{c.game.name}</span>
                            </div>
                            <Badge tone="warning" dot>starts in {fmtDuration(Date.parse(c.startAt) - now)}</Badge>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}

export default function Dashboard() {
    const {watch, snapshot, auto} = useStore();
    const [dismissed, setDismissed] = useState('');
    const campaigns = snapshot?.campaigns ?? [];
    const watching = watch && (watch.state === 'watching' || watch.state === 'starting');
    const warning = snapshot?.warnings?.[0];
    const isAuto = auto?.mode === 'auto';
    const exploring = auto?.state === 'exploring';

    const title = watching ? (exploring ? `Exploring ${auto!.exploring}` : 'Now watching') : 'Nothing to watch';
    const subtitle = isAuto
        ? (auto!.paused ? 'Auto mode is paused.' : exploring ? 'No known campaign for this game yet — watching a drops stream to find out.' : 'Auto mode picks the channel that completes your drops soonest.')
        : 'Manual mode: you choose the channel, the miner keeps it watched.';

    return (
        <>
            {watch?.state === 'error' && dismissed !== watch.error && (
                <Banner tone="error" icon={CircleX} title="Watching stopped" action={
                    <button className="icon-btn" onClick={() => setDismissed(watch.error)}><X size={14}/></button>}>
                    {watch.error}
                </Banner>
            )}
            {warning && dismissed !== warning && (
                <Banner icon={WifiOff} title="Some data is unavailable" action={<>
                    <Button variant="ghost" size="sm" onClick={() => RefreshCampaigns()}>Retry now</Button>
                    <button className="icon-btn" onClick={() => setDismissed(warning)}><X size={14}/></button>
                </>}>{warning}</Banner>
            )}
            <Header title={title} subtitle={subtitle}/>
            {watching || watch?.state === 'offline' ? <Hero watch={watch!} campaigns={campaigns}/> : <Idle campaigns={campaigns}/>}
            <UpNext campaigns={campaigns} current={watch?.channel?.login}/>
        </>
    );
}
