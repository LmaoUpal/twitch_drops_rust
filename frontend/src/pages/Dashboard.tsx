import {useState} from 'react';
import {
    ArrowLeftRight, CircleX, ExternalLink, Gift, Hand, Hourglass, Eye, MoonStar, RadioTower, RefreshCw,
    Send, Sparkles, Square, Timer, User, WifiOff, X,
} from 'lucide-react';
import {RefreshCampaigns, StopWatching, WatchChannel} from '../../wailsjs/go/main/App';
import {main, models} from '../../wailsjs/go/models';
import {BrowserOpenURL} from '../../wailsjs/runtime/runtime';
import {useStore} from '../store';
import {campaignPhase, findDrop, isRust, parentCampaignId, upNext} from '../lib/derive';
import {fmtDuration, fmtHM, fmtMinutes, fmtTime, fmtViewers, isSet, pct} from '../lib/format';
import {Avatar, Badge, Banner, Button, Empty, Progress, Ring, Segmented, Thumb} from '../components/ui';

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
            {error && <span style={{color: 'var(--error-text)', fontSize: 12}}>{error}</span>}
        </div>
    );
}

function Header({title, subtitle}: { title: string; subtitle: string }) {
    return (
        <div className="page-header">
            <div className="grow">
                <h1 className="page-title">{title}</h1>
                <div className="page-subtitle">{subtitle}</div>
            </div>
            <Segmented value="manual" onChange={() => undefined} options={[
                {value: 'auto', label: 'Auto', icon: Sparkles, disabled: true, title: 'Auto mode arrives in the next update'},
                {value: 'manual', label: 'Manual', icon: Hand},
            ]}/>
        </div>
    );
}

function Hero({watch, campaigns}: { watch: main.WatchState; campaigns: models.Campaign[] }) {
    const {now} = useStore();
    const [switching, setSwitching] = useState(false);
    const ch: models.Channel = watch.channel;
    const offline = watch.state === 'offline';
    const s = watch.session;
    const found = s?.dropId ? findDrop(campaigns, s.dropId) : null;
    const rust = found ? isRust(found.campaign.game) : ch.gameId === '263490';
    const ready = s.requiredMinutes > 0 && s.currentMinutes >= s.requiredMinutes;
    const left = s?.requiredMinutes ? s.requiredMinutes - s.currentMinutes : 0;
    const also = campaigns
        .filter(c => campaignPhase(c, now) === 'active')
        .flatMap(c => (c.drops ?? []).map(d => ({c, d})))
        .filter(({d}) => !d.claimed && d.currentMinutes > 0 && d.id !== s?.dropId)
        .slice(0, 3);
    const st = watch.status;
    const name = ch.displayName || ch.login;

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
                    <div className="stream-title">{offline ? 'Channel went offline. Pick another channel to keep earning.' : ch.title}</div>
                    <div className="row" style={{gap: 6}}>
                        {ch.gameName && <Badge tone={rust ? 'rust' : 'accent'} dot>{ch.gameName}</Badge>}
                        {found?.campaign.allowedChannels && <Badge tone="accent">Streamer drop</Badge>}
                        {found && !found.campaign.allowedChannels && <Badge>General drops</Badge>}
                    </div>
                </div>
                <div className="row">
                    <Button icon={Square} onClick={() => StopWatching()}>Stop</Button>
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
                            {s?.requiredMinutes ? 'Twitch reports a drop for this channel, refresh campaigns to see it.' : 'Twitch reports drop progress a minute or two after watching starts.'}
                        </span>
                    )}
                    {s.requiredMinutes > 0 && (ready ? (
                        <div className="row" style={{color: 'var(--success-text)', fontWeight: 600}}>
                            <Gift size={15}/> Ready to claim <span className="muted" style={{fontWeight: 400}}>· claiming arrives with auto mode</span>
                        </div>
                    ) : (
                        <div className="row">
                            <Hourglass size={15} className="muted"/>
                            <span style={{fontSize: 14, fontWeight: 600}}>~{fmtMinutes(left)} left</span>
                            <span className="muted">· ready ≈ {fmtHM(new Date(now + left * 60000))}</span>
                        </div>
                    ))}
                </div>
                {also.length > 0 && (
                    <>
                        <div className="vdivider"/>
                        <div className="col" style={{gap: 12, width: 236}}>
                            <span className="overline">Also progressing</span>
                            {also.map(({d}) => (
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
                <span><Timer size={12}/>polling every 10 s</span>
            </div>
        </div>
    );
}

function UpNext({campaigns, current}: { campaigns: models.Campaign[]; current?: string }) {
    const {now} = useStore();
    const items = upNext(campaigns, now, current);
    if (!items.length) return null;
    return (
        <div className="col" style={{gap: 6}}>
            <div className="row" style={{paddingBottom: 4}}>
                <span className="grow" style={{fontSize: 15, fontWeight: 600}}>Up next</span>
                <span className="faint" style={{fontSize: 12}}>Live streamer-drop channels</span>
            </div>
            {items.map((it, i) => (
                <div key={it.channel.login} className={`queue-row ${it.skipped ? 'skipped' : ''}`}>
                    <span className="queue-index">{i + 1}</span>
                    <Avatar src={it.channel.avatarUrl} name={it.channel.displayName || it.channel.login} online/>
                    <div className="grow col" style={{gap: 1}}>
                        <span style={{fontWeight: 600}}>{it.channel.displayName || it.channel.login}</span>
                        <span className="muted ellipsis" style={{fontSize: 12}}>
                            {it.campaign.game.name}{it.drop ? ` · ${it.drop.name} ${fmtMinutes(it.drop.currentMinutes)} / ${fmtMinutes(it.drop.requiredMinutes)}` : ''}
                            {it.skipped ? ` · ${it.skipped}` : ''}
                        </span>
                    </div>
                    <Badge tone={it.skipped ? 'error' : 'neutral'}>{it.skipped ? 'skipped' : it.reason}</Badge>
                    <Button size="sm" icon={Eye} disabled={!!it.skipped} onClick={() => WatchChannel(it.channel.login)}>Watch</Button>
                </div>
            ))}
        </div>
    );
}

export default function Dashboard() {
    const {watch, snapshot, now, setPage} = useStore();
    const [dismissed, setDismissed] = useState('');
    const campaigns = snapshot?.campaigns ?? [];
    const watching = watch && (watch.state === 'watching' || watch.state === 'starting');
    const warning = snapshot?.warnings?.[0];
    const upcoming = campaigns.filter(c => campaignPhase(c, now) === 'upcoming' && !parentCampaignId(c))
        .sort((a, b) => Date.parse(a.startAt) - Date.parse(b.startAt)).slice(0, 3);

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
            <Header title={watching ? 'Now watching' : 'Nothing to watch'}
                    subtitle={watching ? 'Manual mode: you choose the channel, the miner keeps it watched.' : 'Start watching a channel that has drops for your games.'}/>
            {watching || watch?.state === 'offline' ? <Hero watch={watch} campaigns={campaigns}/> : (
                <div className="card empty" style={{minHeight: 0, padding: '48px 32px'}}>
                    <div className="empty-orb"><div><MoonStar size={22}/></div></div>
                    <h3>Not watching anything</h3>
                    <p>Pick a live channel from the list below or from Campaigns, or type a channel login. Auto mode arrives in the next update.</p>
                    <div className="row" style={{marginTop: 8}}>
                        <Button variant="primary" icon={Gift} onClick={() => setPage('campaigns')}>Browse campaigns</Button>
                    </div>
                    <div style={{marginTop: 12}}><WatchForm/></div>
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
            )}
            <UpNext campaigns={campaigns} current={watch?.channel?.login}/>
        </>
    );
}
