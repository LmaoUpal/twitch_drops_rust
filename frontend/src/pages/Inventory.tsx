import {useState} from 'react';
import {CircleCheck, Gamepad2, LayoutGrid, List, Package, Search} from 'lucide-react';
import {useStore} from '../store';
import {fmtDay, fmtHM, fmtMonth, isSet} from '../lib/format';
import {t, tn} from '../i18n';
import {Badge, Button, Empty, Segmented} from '../components/ui';

interface Item {
    key: string;
    name: string;
    game: string;
    campaign: string;
    imageUrl: string;
    claimedAt?: Date;
}


export default function Inventory() {
    const {snapshot, setPage} = useStore();
    const [view, setView] = useState<'grid' | 'list'>('grid');
    const [query, setQuery] = useState('');
    const [game, setGame] = useState('');

    // Claimed rewards come from Twitch's inventory history plus drops marked claimed in
    // campaigns that are still running (those have no claim date).
    const items: Item[] = [];
    const seen = new Set<string>();
    for (const d of snapshot?.claimed ?? []) {
        const key = `${d.game.name}|${d.name}`.toLowerCase();
        seen.add(key);
        items.push({
            key: d.id || key, name: d.name, game: d.game.name, campaign: d.count > 1 ? `×${d.count}` : '',
            imageUrl: d.imageUrl, claimedAt: isSet(d.claimedAt) ? new Date(d.claimedAt) : undefined,
        });
    }
    for (const c of snapshot?.campaigns ?? []) {
        for (const d of c.drops ?? []) {
            const key = `${c.game.name}|${d.name}`.toLowerCase();
            if (!d.claimed || seen.has(key)) continue;
            seen.add(key);
            items.push({key: d.id, name: d.name, game: c.game.name, campaign: c.name, imageUrl: d.imageUrl});
        }
    }
    items.sort((a, b) => (b.claimedAt?.getTime() ?? Infinity) - (a.claimedAt?.getTime() ?? Infinity));

    const games = [...new Set(items.map(i => i.game))];
    const perGame = games.map(g => ({g, n: items.filter(i => i.game === g).length})).sort((a, b) => b.n - a.n);
    const now = new Date();
    const thisMonth = items.filter(i => i.claimedAt && i.claimedAt.getMonth() === now.getMonth() && i.claimedAt.getFullYear() === now.getFullYear()).length;
    const today = items.filter(i => i.claimedAt && i.claimedAt.toDateString() === now.toDateString()).length;
    const oldest = items.filter(i => i.claimedAt).at(-1)?.claimedAt;

    const q = query.trim().toLowerCase();
    const shown = items.filter(i => (!game || i.game === game) && (!q || i.name.toLowerCase().includes(q) || i.game.toLowerCase().includes(q)));
    const groups = new Map<string, Item[]>();
    for (const i of shown) {
        const label = i.claimedAt ? fmtMonth(i.claimedAt) : t('inv.current');
        groups.set(label, [...(groups.get(label) ?? []), i]);
    }

    return (
        <>
            <div className="page-header">
                <div className="grow">
                    <h1 className="page-title">{t('inv.title')}</h1>
                    <div className="page-subtitle">{t('inv.subtitle')}</div>
                </div>
                <label className="input" style={{width: 200}}>
                    <Search size={14}/>
                    <input placeholder={t('inv.search')} value={query} onChange={e => setQuery(e.target.value)}/>
                </label>
                <label className="input" style={{width: 'auto', paddingRight: 4, borderColor: 'var(--border-strong)'}}>
                    <Gamepad2 size={14} color="var(--text-primary)"/>
                    <select value={game} onChange={e => setGame(e.target.value)} style={{fontWeight: 600, cursor: 'pointer'}}>
                        <option value="">{t('common.allGames')}</option>
                        {games.map(g => <option key={g} value={g}>{g}</option>)}
                    </select>
                </label>
            </div>

            {items.length === 0 ? (
                <div className="card" style={{flex: 1, display: 'flex'}}>
                    <Empty icon={Package} title={t('inv.empty')}
                           actions={<Button variant="primary" onClick={() => setPage('campaigns')}>{t('dash.browse')}</Button>}>
                        {t('inv.emptyText')}
                    </Empty>
                </div>
            ) : (
                <>
                    <div className="card stats-strip">
                        <div>
                            <span className="stat-big">{items.length}</span>
                            <div className="col">
                                <span style={{fontWeight: 500}}>{t('inv.dropsClaimed')}</span>
                                {oldest && <span className="faint" style={{fontSize: 12}}>{t('inv.since', {d: `${fmtDay(oldest)} ${oldest.getFullYear()}`})}</span>}
                            </div>
                        </div>
                        {perGame.slice(0, 2).map(({g, n}) => (
                            <div key={g}>
                                <div className="muted" style={{fontSize: 12}}>{g}</div>
                                <div className="stat-value">{n}</div>
                            </div>
                        ))}
                        <div>
                            <div className="muted" style={{fontSize: 12}}>{t('inv.thisMonth')}</div>
                            <div className="row"><span className="stat-value">{thisMonth}</span>
                                {today > 0 && <span style={{color: 'var(--success-text)', fontSize: 12}}>{t('inv.today', {n: today})}</span>}</div>
                        </div>
                    </div>

                    {[...groups.entries()].map(([label, list], gi) => (
                        <div key={label} className="col" style={{gap: 12}}>
                            <div className="row">
                                <span className="grow" style={{fontSize: 14, fontWeight: 600}}>
                                    {label} <span className="faint" style={{fontWeight: 400, fontSize: 12, marginLeft: 6}}>{tn('inv.itemsN', list.length)}</span>
                                </span>
                                {gi === 0 && <Segmented<"grid" | "list"> value={view} onChange={setView} options={[
                                    {value: 'grid', label: t('inv.grid'), icon: LayoutGrid},
                                    {value: 'list', label: t('inv.list'), icon: List},
                                ]}/>}
                            </div>
                            {view === 'grid' ? (
                                <div className="item-grid">
                                    {list.map((i, n) => (
                                        <div key={i.key} className="item-card" style={gi === 0 && n === 0 && today ? {borderColor: '#9146FF88'} : undefined}>
                                            <div className="item-image" style={{backgroundImage: i.imageUrl ? `url("${i.imageUrl}")` : undefined}}/>
                                            {gi === 0 && n === 0 && today > 0 && <span className="new"><Badge tone="accent">{t('inv.new')}</Badge></span>}
                                            <div style={{fontWeight: 600}}>{i.name}</div>
                                            <div className="faint" style={{fontSize: 11}}>{i.game}{i.campaign ? ` · ${i.campaign}` : ''}</div>
                                            <div className="row muted" style={{gap: 5, fontSize: 12}}>
                                                <CircleCheck size={13} color="var(--success-text)"/>
                                                {i.claimedAt ? (i.claimedAt.toDateString() === now.toDateString() ? t('inv.todayAt', {t: fmtHM(i.claimedAt)}) : fmtDay(i.claimedAt)) : t('camp.claimed')}
                                            </div>
                                        </div>
                                    ))}
                                </div>
                            ) : (
                                <div className="card">
                                    {list.map(i => (
                                        <div key={i.key} className="item-list-row">
                                            <div className="thumb" style={{width: 36, height: 36, backgroundImage: i.imageUrl ? `url("${i.imageUrl}")` : undefined}}/>
                                            <div className="grow col">
                                                <span style={{fontWeight: 600}}>{i.name}</span>
                                                <span className="faint" style={{fontSize: 12}}>{i.game}{i.campaign ? ` · ${i.campaign}` : ''}</span>
                                            </div>
                                            <span className="muted" style={{fontSize: 12}}>{i.claimedAt ? fmtDay(i.claimedAt) : t('camp.claimed')}</span>
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                    ))}
                </>
            )}
        </>
    );
}
