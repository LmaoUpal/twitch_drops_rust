import {useState} from 'react';
import {AtSign, Coins, Gift, GripVertical, Plus, UserPlus, X} from 'lucide-react';
import {AddPointsChannel, ImportFollows, MovePointsChannel, RemovePointsChannel} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';
import {useStore} from '../store';
import {fmtViewers} from '../lib/format';
import {Key, locale, t, tn} from '../i18n';
import {Avatar, Badge, Button, Empty, Toggle} from '../components/ui';

const stateBadge: Record<string, { tone: 'success' | 'accent' | 'warning' | 'neutral'; key: Key }> = {
    watching: {tone: 'success', key: 'pts.watching'},
    drops: {tone: 'accent', key: 'pts.drops'},
    live: {tone: 'warning', key: 'pts.waiting'},
    offline: {tone: 'neutral', key: 'pts.offline'},
    unknown: {tone: 'neutral', key: 'pts.unknown'},
    disabled: {tone: 'neutral', key: 'pts.disabledOnChannel'},
};

const num = (n: number) => n.toLocaleString(locale());

function ChannelRow({c, i, drag, setDrag}: { c: main.PointsChannelState; i: number; drag: number | null; setDrag(i: number | null): void }) {
    const ch = c.channel;
    const badge = stateBadge[c.state] ?? stateBadge.unknown;
    const name = ch.displayName || ch.login;
    return (
        <div className={`game-row points-row ${drag === i ? 'dragging' : ''}`} draggable
             onDragStart={() => setDrag(i)} onDragEnd={() => setDrag(null)}
             onDragOver={e => { e.preventDefault(); if (drag !== null && drag !== i) { MovePointsChannel(ch.login, i); setDrag(i); } }}>
            <GripVertical size={14} className="grip"/>
            <span className="idx">{i + 1}</span>
            <Avatar src={ch.avatarUrl} name={name} online={ch.online}/>
            <div className="grow col" style={{gap: 2, minWidth: 0}}>
                <div className="row" style={{gap: 8}}>
                    <span style={{fontWeight: 600}}>{name}</span>
                    <Badge tone={badge.tone} dot>{t(badge.key)}</Badge>
                </div>
                <span className="muted ellipsis" style={{fontSize: 12}}>
                    {c.error
                        ? <span style={{color: 'var(--error-text)'}}>{c.error}</span>
                        : ch.online ? `${t('camp.liveViewers', {n: fmtViewers(ch.viewers)})}${ch.gameName ? ` · ${ch.gameName}` : ''}` : t('camp.offline')}
                </span>
            </div>
            <div className="col points-stats">
                <span className="mono points-balance"><Coins size={13}/>{num(c.balance)}</span>
                <span className="faint" style={{fontSize: 11}}>
                    {c.earned > 0 && <span style={{color: 'var(--success-text)'}}>+{num(c.earned)} </span>}
                    {c.bonuses > 0 && <span title={t('pts.bonusesTitle')}><Gift size={11}/> {c.bonuses}</span>}
                </span>
            </div>
            <button className="icon-btn" title={t('set.remove')} onClick={() => RemovePointsChannel(ch.login)}><X size={14}/></button>
        </div>
    );
}

export default function Points() {
    const {points, settings, updateSettings} = useStore();
    const [name, setName] = useState('');
    const [error, setError] = useState('');
    const [note, setNote] = useState('');
    const [busy, setBusy] = useState(false);
    const [drag, setDrag] = useState<number | null>(null);
    const channels = points?.channels ?? [];
    const watching = channels.filter(c => c.state === 'watching').length;
    const total = channels.reduce((s, c) => s + c.earned, 0);
    if (!settings) return null;

    async function add() {
        if (!name.trim()) return;
        setBusy(true);
        setError('');
        try {
            await AddPointsChannel(name);
            setName('');
        } catch (e) {
            setError(String(e));
        }
        setBusy(false);
    }

    async function importFollows() {
        setBusy(true);
        setError('');
        try {
            setNote(tn('pts.imported', await ImportFollows()));
        } catch (e) {
            setError(String(e));
        }
        setBusy(false);
    }

    return (
        <>
            <div className="page-header">
                <div className="grow">
                    <h1 className="page-title">{t('pts.title')}</h1>
                    <div className="page-subtitle">
                        {settings.points.enabled
                            ? t('pts.subtitle', {n: watching, slots: points?.slots ?? 0}) + (total > 0 ? ` · ${t('pts.earnedTotal', {n: num(total)})}` : '')
                            : t('pts.disabled')}
                    </div>
                </div>
                <Button icon={UserPlus} disabled={busy} onClick={importFollows}>{t('pts.import')}</Button>
                <Toggle on={settings.points.enabled} onChange={v => updateSettings(c => { c.points.enabled = v; })}/>
            </div>

            <div className="settings-cols points-cols">
                <div className="settings-col">
                    <div className="card settings-card">
                        <div>
                            <h3>{t('pts.channels')}</h3>
                            <div className="setting-help">{t('pts.channelsHelp')}</div>
                        </div>
                        <div className="inline-form">
                            <label className="input grow">
                                <AtSign size={14}/>
                                <input placeholder={t('set.channelName')} value={name} onChange={e => setName(e.target.value)} onKeyDown={e => e.key === 'Enter' && add()}/>
                            </label>
                            <Button icon={Plus} disabled={busy || !name.trim()} onClick={add}>{t('pts.add')}</Button>
                        </div>
                        {error && <span style={{color: 'var(--error-text)', fontSize: 12}}>{error}</span>}
                        {note && !error && <span className="muted" style={{fontSize: 12}}>{note}</span>}
                        {channels.length === 0
                            ? <Empty icon={Coins} title={t('pts.emptyTitle')}>{t('pts.emptyText')}</Empty>
                            : <div className="col" style={{gap: 6}}>
                                {channels.map((c, i) => <ChannelRow key={c.channel.login} c={c} i={i} drag={drag} setDrag={setDrag}/>)}
                            </div>}
                    </div>
                </div>
                <div className="settings-col">
                    <div className="card settings-card flush">
                        <h3>{t('pts.options')}</h3>
                        <div className="setting-row">
                            <div className="grow">
                                <div className="setting-label">{t('pts.claimBonus')}</div>
                                <div className="setting-help">{t('pts.claimBonusHelp')}</div>
                            </div>
                            <Toggle on={settings.points.claimBonus} onChange={v => updateSettings(c => { c.points.claimBonus = v; })}/>
                        </div>
                        <div className="setting-row">
                            <div className="grow">
                                <div className="setting-label">{t('pts.follow')}</div>
                                <div className="setting-help">{t('pts.followHelp')}</div>
                            </div>
                            <Toggle on={settings.points.followOnAdd} onChange={v => updateSettings(c => { c.points.followOnAdd = v; })}/>
                        </div>
                        <div className="setting-row">
                            <div className="grow">
                                <div className="setting-label">{t('pts.chat')}</div>
                                <div className="setting-help">{t('pts.chatHelp')}</div>
                            </div>
                            <Toggle on={settings.points.joinChat} onChange={v => updateSettings(c => { c.points.joinChat = v; })}/>
                        </div>
                    </div>
                    <div className="card settings-card">
                        <h3>{t('pts.howTitle')}</h3>
                        <div className="setting-help" style={{marginTop: 0}}>{t('pts.how')}</div>
                    </div>
                </div>
            </div>
        </>
    );
}
