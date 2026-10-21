import {useEffect, useState} from 'react';
import {GetCampaigns, RefreshCampaigns} from "../../wailsjs/go/main/App";
import {catalog, models} from "../../wailsjs/go/models";
import {BrowserOpenURL, EventsOn} from "../../wailsjs/runtime/runtime";

const ZERO_TIME = '0001-01-01T00:00:00Z';

function fmtDate(v: string) {
    return new Date(v).toLocaleString(undefined, {dateStyle: 'medium', timeStyle: 'short'});
}

function fmtMinutes(m: number) {
    const h = Math.floor(m / 60), min = m % 60;
    return h ? (min ? `${h}h ${min}m` : `${h}h`) : `${min}m`;
}

function phase(c: models.Campaign, now: number) {
    if (now < Date.parse(c.startAt)) return 'upcoming';
    if (now >= Date.parse(c.endAt)) return 'ended';
    return 'active';
}

function DropRow({drop}: { drop: models.Drop }) {
    const pct = drop.requiredMinutes ? Math.min(100, Math.round(100 * drop.currentMinutes / drop.requiredMinutes)) : 0;
    return (
        <li className="drop">
            {drop.imageUrl && <img src={drop.imageUrl} alt=""/>}
            <div className="drop-info">
                <div className="drop-name">
                    {drop.name}
                    {drop.claimed && <span className="badge ok">claimed</span>}
                </div>
                <div className="progress"><div style={{width: `${drop.claimed ? 100 : pct}%`}}/></div>
                <div className="muted small">{fmtMinutes(drop.currentMinutes)} / {fmtMinutes(drop.requiredMinutes)}</div>
            </div>
        </li>
    );
}

function CampaignCard({c, now}: { c: models.Campaign, now: number }) {
    const p = phase(c, now);
    return (
        <div className="card campaign">
            <div className="campaign-head">
                <div>
                    <div className="campaign-name">{c.name}</div>
                    <div className="muted small">
                        {c.game.name} · {fmtDate(c.startAt)} – {fmtDate(c.endAt)}
                    </div>
                </div>
                <div className="badges">
                    <span className={`badge ${p}`}>{p}</span>
                    <span className="badge">{c.source === 'facepunch' ? 'Facepunch' : 'Twitch'}</span>
                </div>
            </div>
            {!c.accountLinked && (
                <p className="error small">
                    Game account not linked.{' '}
                    {c.linkUrl && <a href="#" onClick={() => BrowserOpenURL(c.linkUrl)}>Link it</a>}
                </p>
            )}
            {c.allowedChannels && (
                <div className="channels small">
                    Only on:{' '}
                    {c.allowedChannels.map(ch => (
                        <span key={ch.login} className="channel">
                            <span className={`dot ${ch.online ? 'on' : ''}`}/>
                            <a href="#" onClick={() => BrowserOpenURL(`https://www.twitch.tv/${ch.login}`)}>{ch.displayName || ch.login}</a>
                            {ch.online && <span className="muted"> {ch.viewers}</span>}
                        </span>
                    ))}
                </div>
            )}
            <ul className="drops">
                {c.drops?.map(d => <DropRow key={d.id} drop={d}/>)}
            </ul>
        </div>
    );
}

function Campaigns() {
    const [snap, setSnap] = useState<catalog.Snapshot | null>(null);

    useEffect(() => {
        GetCampaigns().then(setSnap);
        return EventsOn('campaigns:update', setSnap);
    }, []);

    const loaded = snap && snap.updatedAt && snap.updatedAt !== ZERO_TIME;
    const now = Date.now();
    const campaigns = [...(snap?.campaigns ?? [])]
        .filter(c => phase(c, now) !== 'ended')
        .sort((a, b) => Date.parse(a.endAt) - Date.parse(b.endAt));

    return (
        <section>
            <div className="section-head">
                <h2>Campaigns</h2>
                <span className="muted small">
                    {loaded ? `updated ${new Date(snap!.updatedAt).toLocaleTimeString()}` : 'loading…'}
                    <button className="btn secondary small" onClick={() => RefreshCampaigns()}>Refresh</button>
                </span>
            </div>
            {snap?.warnings?.map(w => <p key={w} className="error small">{w}</p>)}
            {loaded && campaigns.length === 0 && (
                <p className="muted">No campaigns yet. Twitch only lists campaigns you have progress in; Rust campaigns come from twitch.facepunch.com.</p>
            )}
            {campaigns.map(c => <CampaignCard key={c.id} c={c} now={now}/>)}
            <p className="muted small">
                Rust drops need your Steam account linked on{' '}
                <a href="#" onClick={() => BrowserOpenURL('https://twitch.facepunch.com/#account')}>twitch.facepunch.com</a>.
            </p>
        </section>
    );
}

export default Campaigns
