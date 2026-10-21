import {useEffect, useState} from 'react';
import {Check, ChevronDown, ChevronRight, ExternalLink, Globe, Plus, RefreshCw, Search, Sparkles} from 'lucide-react';
import {GetAllDrops} from '../../wailsjs/go/main/App';
import {dropsapi, main} from '../../wailsjs/go/models';
import {BrowserOpenURL} from '../../wailsjs/runtime/runtime';
import {useStore} from '../store';
import {campaignPhase, parentCampaignId} from '../lib/derive';
import {fmtDuration, fmtHM, fmtMinutes, fmtRange, isSet} from '../lib/format';
import {Badge, Banner, Button, Chip, Empty, Thumb} from '../components/ui';
import {t, tn} from '../i18n';

const CATALOG_HOME = 'https://github.com/SunkwiBOT/twitch-drops-api';

type Filter = 'all' | 'mine' | 'open';

interface Row {
    id: string;
    game: string;
    boxArt: string;
    name: string;
    owner: string;
    startAt: string;
    endAt: string;
    channels: dropsapi.Channel[];
    drops: dropsapi.Drop[];
    detailsUrl: string;
    upcoming: boolean;
}

function fromCatalog(c: dropsapi.Campaign): Row {
    return {
        id: c.id, game: c.game.name, boxArt: c.game.boxArtUrl, name: c.name, owner: c.owner,
        startAt: c.startAt, endAt: c.endAt, channels: c.channels ?? [], drops: c.drops ?? [],
        detailsUrl: c.detailsUrl, upcoming: false,
    };
}

function DropRow({row, mine, now, onAdd}: { row: Row; mine: boolean; now: number; onAdd(): void }) {
    const [open, setOpen] = useState(false);
    const shown = row.channels.slice(0, 12);
    return (
        <div className="all-row">
            <div className="all-row-head" onClick={() => setOpen(v => !v)}>
                <div className="boxart" style={{backgroundImage: row.boxArt ? `url("${row.boxArt}")` : undefined}}/>
                <div className="grow col" style={{gap: 2, minWidth: 0}}>
                    <span className="ellipsis" style={{fontWeight: 600}}>{row.game}</span>
                    <span className="muted ellipsis" style={{fontSize: 12}}>{row.name}{row.owner ? ` · ${row.owner}` : ''}</span>
                </div>
                {row.upcoming ? <Badge tone="warning" dot>{t('dash.startsIn', {t: fmtDuration(Date.parse(row.startAt) - now)})}</Badge>
                    : <span className="faint" style={{fontSize: 12, whiteSpace: 'nowrap'}}>{t('camp.endsIn', {t: fmtDuration(Date.parse(row.endAt) - now)})}</span>}
                <span className="muted" style={{fontSize: 12, whiteSpace: 'nowrap', width: 120, textAlign: 'right'}}>{fmtRange(row.startAt, row.endAt)}</span>
                {mine
                    ? <Badge tone="success"><Check size={11}/>{t('all.inList')}</Badge>
                    : <Button size="sm" icon={Plus} onClick={e => { e.stopPropagation(); onAdd(); }}>{t('all.add')}</Button>}
                {open ? <ChevronDown size={16} className="faint"/> : <ChevronRight size={16} className="faint"/>}
            </div>
            {open && (
                <div className="all-row-body">
                    <div className="all-drops">
                        {row.drops.map(d => (
                            <div key={d.id} className="all-drop">
                                <Thumb src={d.imageUrl} size={40}/>
                                <div className="col" style={{minWidth: 0}}>
                                    <span className="ellipsis" style={{fontWeight: 500, fontSize: 12}}>{d.name}</span>
                                    <span className="faint" style={{fontSize: 11}}>
                                        {d.requiredSubs > 0 ? tn('all.subsN', d.requiredSubs) : t('camp.watchTime', {t: fmtMinutes(d.requiredMinutes)})}
                                    </span>
                                </div>
                            </div>
                        ))}
                    </div>
                    <div className="muted" style={{fontSize: 12}}>
                        {row.channels.length === 0
                            ? <span className="row" style={{gap: 6}}><Globe size={13}/>{t('camp.anyStream', {game: row.game})}</span>
                            : <>{t('all.onlyOn')} {shown.map(c => c.displayName || c.login).join(', ')}
                                {row.channels.length > shown.length && ` ${t('all.more', {n: row.channels.length - shown.length})}`}</>}
                    </div>
                    {row.detailsUrl && (
                        <a className="row" style={{gap: 6, fontSize: 12}} onClick={() => BrowserOpenURL(row.detailsUrl)}>
                            <ExternalLink size={12}/>{t('all.details')}
                        </a>
                    )}
                </div>
            )}
        </div>
    );
}

export default function AllDrops() {
    const {snapshot, settings, updateSettings, now} = useStore();
    const [data, setData] = useState<main.AllDrops | null>(null);
    const [loading, setLoading] = useState(false);
    const [filter, setFilter] = useState<Filter>('all');
    const [query, setQuery] = useState('');

    async function load(refresh: boolean) {
        setLoading(true);
        try {
            setData(await GetAllDrops(refresh));
        } finally {
            setLoading(false);
        }
    }

    useEffect(() => { load(false); }, []);

    const myGames = (settings?.games ?? []).map(g => g.toLowerCase());
    const isMine = (game: string) => myGames.includes(game.toLowerCase());
    const add = (game: string) => updateSettings(c => { if (!c.games.some(g => g.toLowerCase() === game.toLowerCase())) c.games = [...c.games, game]; });

    // The catalog only has running campaigns; upcoming Rust ones come from Facepunch.
    const upcoming: Row[] = (snapshot?.campaigns ?? [])
        .filter(c => c.source === 'facepunch' && !parentCampaignId(c) && campaignPhase(c, now) === 'upcoming')
        .map(c => ({
            id: c.id, game: c.game.name, boxArt: `https://static-cdn.jtvnw.net/ttv-boxart/${c.game.id}_IGDB-120x160.jpg`,
            name: c.name, owner: 'Facepunch', startAt: c.startAt, endAt: c.endAt,
            channels: [], drops: (c.drops ?? []).map(d => dropsapi.Drop.createFrom({id: d.id, name: d.name, imageUrl: d.imageUrl, requiredMinutes: d.requiredMinutes, requiredSubs: 0})),
            detailsUrl: 'https://twitch.facepunch.com/', upcoming: true,
        }));
    const running = (data?.campaigns ?? []).map(fromCatalog).filter(r => Date.parse(r.endAt) > now);
    const rows = [...upcoming, ...running.sort((a, b) => Date.parse(a.endAt) - Date.parse(b.endAt))];

    const q = query.trim().toLowerCase();
    const shown = rows
        .filter(r => filter !== 'mine' || isMine(r.game))
        .filter(r => filter !== 'open' || r.channels.length === 0)
        .filter(r => !q || r.game.toLowerCase().includes(q) || r.name.toLowerCase().includes(q) || r.drops.some(d => d.name.toLowerCase().includes(q)));
    const games = new Set(rows.map(r => r.game.toLowerCase())).size;
    const mineCount = rows.filter(r => isMine(r.game)).length;

    return (
        <>
            <div className="page-header">
                <div className="grow">
                    <h1 className="page-title">{t('all.title')}</h1>
                    <div className="page-subtitle">
                        {tn('camp.campaignsN', rows.length)} · {tn('camp.gamesN', games)}
                        {data && isSet(data.updatedAt) && t('all.catalogUpdated', {t: fmtHM(data.updatedAt)})}
                    </div>
                </div>
                <label className="input" style={{width: 220}}>
                    <Search size={14}/>
                    <input placeholder={t('all.search')} value={query} onChange={e => setQuery(e.target.value)}/>
                </label>
                <Button icon={RefreshCw} title={t('common.refresh')} onClick={() => load(true)} className={loading ? 'refreshing' : ''}/>
            </div>
            <div className="toolbar">
                <Chip active={filter === 'all'} count={rows.length} onClick={() => setFilter('all')}>{t('all.filterAll')}</Chip>
                <Chip active={filter === 'mine'} count={mineCount} onClick={() => setFilter('mine')}>{t('all.filterMine')}</Chip>
                <Chip active={filter === 'open'} onClick={() => setFilter('open')}>{t('all.filterOpen')}</Chip>
                <span className="grow"/>
                <span className="faint" style={{fontSize: 12}}>{t('camp.sortEnds')}</span>
            </div>
            <div className="row faint" style={{fontSize: 12, gap: 6}}>
                <Sparkles size={13}/>
                <span>{t('all.sourceNote')} <a onClick={() => BrowserOpenURL(CATALOG_HOME)}>twitch-drops-api</a>.</span>
            </div>
            {data?.error && <Banner icon={Globe} title={t('all.loadFailed')}>{data.error}</Banner>}
            {shown.length === 0 ? (
                <div className="card" style={{flex: 1, display: 'flex'}}>
                    <Empty icon={Sparkles} title={loading ? t('all.loading') : t('all.none')}>{loading ? '' : t('all.noneText')}</Empty>
                </div>
            ) : (
                <div className="card">
                    {shown.map(r => <DropRow key={r.id} row={r} mine={isMine(r.game)} now={now} onAdd={() => add(r.game)}/>)}
                </div>
            )}
        </>
    );
}
