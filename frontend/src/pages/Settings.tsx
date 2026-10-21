import {useEffect, useState} from 'react';
import {AtSign, ChevronDown, ChevronRight, FolderOpen, GripVertical, KeyRound, LogOut, Pencil, Plus, RotateCcw, Timer, X} from 'lucide-react';
import {DefaultSettings, Logout, OpenConfigFolder} from '../../wailsjs/go/main/App';
import {config} from '../../wailsjs/go/models';
import {useStore} from '../store';
import {campaignPhase} from '../lib/derive';
import {plural} from '../lib/format';
import {Avatar, Badge, Button, GameIcon, Segmented, Soon, Toggle} from '../components/ui';

function GamesPriority() {
    const {settings, updateSettings, snapshot, now} = useStore();
    const [drag, setDrag] = useState<number | null>(null);
    const [adding, setAdding] = useState('');
    const games = settings?.games ?? [];
    const campaigns = snapshot?.campaigns ?? [];

    function move(from: number, to: number) {
        if (from === to) return;
        updateSettings(c => {
            const [g] = c.games.splice(from, 1);
            c.games.splice(to, 0, g);
        });
    }

    function add() {
        const name = adding.trim();
        if (!name) return;
        updateSettings(c => { c.games = [...c.games, name]; });
        setAdding('');
    }

    return (
        <div className="card settings-card">
            <div>
                <h3>Games priority</h3>
                <div className="setting-help">Drag to reorder. Games higher in the list are watched first.</div>
            </div>
            <div className="col" style={{gap: 6}}>
                {games.map((g, i) => {
                    const mine = campaigns.filter(c => c.game.name.toLowerCase() === g.toLowerCase() && campaignPhase(c, now) !== 'ended');
                    const unlinked = mine.some(c => !c.accountLinked);
                    return (
                        <div key={g} className={`game-row ${drag === i ? 'dragging' : ''}`} draggable
                             onDragStart={() => setDrag(i)} onDragEnd={() => setDrag(null)}
                             onDragOver={e => { e.preventDefault(); if (drag !== null && drag !== i) { move(drag, i); setDrag(i); } }}>
                            <GripVertical size={14} className="grip"/>
                            <span className="idx">{i + 1}</span>
                            <GameIcon name={g} size={24}/>
                            <div className="grow col">
                                <span style={{fontWeight: 500}}>{g}</span>
                                <span style={{fontSize: 11, color: mine.length ? (unlinked ? 'var(--error-text)' : 'var(--success-text)') : 'var(--text-tertiary)'}}>
                                    {mine.length ? `${plural(mine.length, 'campaign')}${unlinked ? ' · not linked' : ''}` : 'no known campaigns'}
                                </span>
                            </div>
                            <button className="icon-btn" title="Remove" onClick={() => updateSettings(c => { c.games.splice(i, 1); })}><X size={14}/></button>
                        </div>
                    );
                })}
                <label className="input" style={{height: 34}}>
                    <Plus size={14}/>
                    <input placeholder="Add a game — exact Twitch category name" value={adding}
                           onChange={e => setAdding(e.target.value)} onKeyDown={e => e.key === 'Enter' && add()}/>
                </label>
            </div>
        </div>
    );
}

function ExcludedChannels() {
    const {settings, updateSettings} = useStore();
    const [name, setName] = useState('');
    const list = settings?.excludedChannels ?? [];
    function add() {
        if (!name.trim()) return;
        updateSettings(c => { c.excludedChannels = [...(c.excludedChannels ?? []), name.trim()]; });
        setName('');
    }
    return (
        <div className="card settings-card">
            <div>
                <h3>Excluded channels</h3>
                <div className="setting-help">Never watch these, even if they carry drops.</div>
            </div>
            {list.length > 0 && (
                <div className="chip-list">
                    {list.map((c, i) => (
                        <span key={c} className="tag-chip">{c}
                            <button className="icon-btn" onClick={() => updateSettings(s => { s.excludedChannels.splice(i, 1); })}><X size={12}/></button>
                        </span>
                    ))}
                </div>
            )}
            <div className="inline-form">
                <label className="input grow">
                    <AtSign size={14}/>
                    <input placeholder="channel name" value={name} onChange={e => setName(e.target.value)} onKeyDown={e => e.key === 'Enter' && add()}/>
                </label>
                <Button icon={Plus} onClick={add}>Exclude</Button>
            </div>
        </div>
    );
}

function SettingRow({label, help, children, soon}: { label: string; help?: string; children: React.ReactNode; soon?: boolean }) {
    return (
        <div className="setting-row">
            <div className="grow">
                <div className="setting-label row" style={{gap: 6}}>{label}{soon && <Soon/>}</div>
                {help && <div className="setting-help">{help}</div>}
            </div>
            {children}
        </div>
    );
}

function Advanced() {
    const {settings, updateSettings} = useStore();
    const [open, setOpen] = useState(false);
    const [defaults, setDefaults] = useState<config.Config | null>(null);
    const [editing, setEditing] = useState<string | null>(null);
    const [newOp, setNewOp] = useState({op: '', hash: ''});
    useEffect(() => { DefaultSettings().then(setDefaults); }, []);
    if (!settings) return null;

    const hashes = settings.twitch.hashes ?? {};
    const ops = Object.keys(hashes).sort();
    const isOverride = (op: string) => defaults && defaults.twitch.hashes[op] !== hashes[op];
    const overrides = ops.filter(isOverride).length;
    const short = (h: string) => h.length > 16 ? `${h.slice(0, 7)}…${h.slice(-6)}` : h;

    return (
        <div className="card settings-card flush">
            <div className="setting-row" style={{cursor: 'pointer'}} onClick={() => setOpen(v => !v)}>
                {open ? <ChevronDown size={16} className="faint"/> : <ChevronRight size={16} className="faint"/>}
                <div className="grow">
                    <div className="setting-label" style={{fontWeight: 600}}>Advanced</div>
                    {!open && <div className="setting-help">Client ID, GQL hash overrides, refresh interval</div>}
                </div>
                {open && <Badge tone="warning" dot>for troubleshooting</Badge>}
            </div>
            {open && (
                <div className="col" style={{gap: 14, padding: '4px 16px 16px', borderTop: '1px solid var(--border)'}}>
                    <div className="col" style={{gap: 6, paddingTop: 12}}>
                        <div className="row"><span className="grow" style={{fontWeight: 600}}>Twitch client ID</span>
                            <span className="faint" style={{fontSize: 11}}>{defaults && settings.twitch.clientId === defaults.twitch.clientId ? 'Android TV app · default' : 'custom'}</span></div>
                        <label className="input mono">
                            <KeyRound size={14}/>
                            <input className="mono" value={settings.twitch.clientId} onChange={e => updateSettings(c => { c.twitch.clientId = e.target.value; })}/>
                        </label>
                        <span className="faint" style={{fontSize: 11}}>Takes effect after restarting the app.</span>
                    </div>
                    <div className="col" style={{gap: 6}}>
                        <div className="row"><span className="grow" style={{fontWeight: 600}}>GQL hash overrides</span>
                            <span className="faint" style={{fontSize: 11}}>{plural(overrides, 'override')} active</span></div>
                        <div className="hash-table">
                            <div className="hrow"><span>Operation</span><span>sha256Hash</span><span/></div>
                            {ops.map(op => (
                                <div key={op} className={`hrow ${isOverride(op) ? 'override' : ''}`}>
                                    <span className="ellipsis" title={op}>{op}</span>
                                    {editing === op
                                        ? <input autoFocus defaultValue={hashes[op]} onBlur={e => { const v = e.target.value.trim(); if (v) updateSettings(c => { c.twitch.hashes[op] = v; }); setEditing(null); }}
                                                 onKeyDown={e => e.key === 'Enter' && (e.target as HTMLInputElement).blur()}/>
                                        : <span className="hval faint" title={hashes[op]}>{short(hashes[op])}</span>}
                                    {isOverride(op) && defaults?.twitch.hashes[op]
                                        ? <button className="icon-btn" title="Reset to default" onClick={() => updateSettings(c => { c.twitch.hashes[op] = defaults!.twitch.hashes[op]; })}><RotateCcw size={12}/></button>
                                        : <button className="icon-btn" title="Edit" onClick={() => setEditing(op)}><Pencil size={12}/></button>}
                                </div>
                            ))}
                        </div>
                        <div className="inline-form">
                            <input className="input mono grow" style={{fontSize: 11}} placeholder="Operation" value={newOp.op} onChange={e => setNewOp({...newOp, op: e.target.value})}/>
                            <input className="input mono grow" style={{fontSize: 11}} placeholder="sha256Hash" value={newOp.hash} onChange={e => setNewOp({...newOp, hash: e.target.value})}/>
                            <Button variant="ghost" icon={Plus} disabled={!newOp.op || !newOp.hash}
                                    onClick={() => { updateSettings(c => { c.twitch.hashes[newOp.op.trim()] = newOp.hash.trim(); }); setNewOp({op: '', hash: ''}); }}>Add override</Button>
                        </div>
                    </div>
                    <div className="col" style={{gap: 6}}>
                        <div className="row"><span className="grow" style={{fontWeight: 600}}>Refresh interval</span><span className="faint" style={{fontSize: 11}}>min 30 s</span></div>
                        <div className="row">
                            <label className="input" style={{width: 96}}>
                                <Timer size={14}/>
                                <input type="number" min={30} value={settings.refreshSeconds}
                                       onChange={e => updateSettings(c => { c.refreshSeconds = Math.max(30, Number(e.target.value) || 30); })}/>
                            </label>
                            <span className="muted" style={{fontSize: 12}}>seconds between campaign & inventory syncs</span>
                        </div>
                    </div>
                    <div className="row" style={{borderTop: '1px solid var(--border)', paddingTop: 14, justifyContent: 'space-between'}}>
                        <Button variant="danger" icon={RotateCcw} disabled={!defaults}
                                onClick={() => updateSettings(c => { c.twitch = defaults!.twitch; c.refreshSeconds = defaults!.refreshSeconds; })}>Reset to defaults</Button>
                        <Button variant="ghost" icon={FolderOpen} onClick={() => OpenConfigFolder()}>Open config folder</Button>
                    </div>
                </div>
            )}
        </div>
    );
}

export default function Settings() {
    const {auth, settings, updateSettings, settingsError} = useStore();
    if (!settings) return null;
    const set = (change: (c: config.Config) => void) => updateSettings(change);

    return (
        <>
            <div className="page-header">
                <div className="grow">
                    <h1 className="page-title">Settings</h1>
                    <div className="page-subtitle">{settingsError ? <span style={{color: 'var(--error-text)'}}>{settingsError}</span> : 'Changes save automatically.'}</div>
                </div>
            </div>
            <div className="settings-cols">
                <div className="settings-col">
                    <div className="card settings-card">
                        <h3>Account</h3>
                        <div className="row" style={{gap: 12}}>
                            <Avatar src={auth.avatarUrl} name={auth.login} size={40}/>
                            <div className="grow col" style={{gap: 2}}>
                                <span style={{fontSize: 14, fontWeight: 600}}>{auth.displayName || auth.login}</span>
                                <span className="muted" style={{fontSize: 12}}>Earning drops on this account</span>
                            </div>
                            <Button variant="danger" icon={LogOut} onClick={() => Logout()}>Log out</Button>
                        </div>
                    </div>
                    <GamesPriority/>
                    <ExcludedChannels/>
                </div>
                <div className="settings-col">
                    <div className="card settings-card flush">
                        <h3>Behavior</h3>
                        <SettingRow label="Auto-claim drops" help="Claim as soon as a drop is ready.">
                            <Toggle on={settings.autoClaim} onChange={v => set(c => { c.autoClaim = v; })}/>
                        </SettingRow>
                        <SettingRow label="Start with Windows" help="Launch minimized to the tray at sign-in.">
                            <Toggle on={settings.startWithWindows} onChange={v => set(c => { c.startWithWindows = v; })}/>
                        </SettingRow>
                        <SettingRow label="Minimize to tray on close" help="The × button hides the window; mining continues.">
                            <Toggle on={settings.minimizeToTray} onChange={v => set(c => { c.minimizeToTray = v; })}/>
                        </SettingRow>
                    </div>
                    <div className="card settings-card flush">
                        <h3>Desktop notifications</h3>
                        <SettingRow label="Drop claimed">
                            <Toggle on={settings.notify.dropClaimed} onChange={v => set(c => { c.notify.dropClaimed = v; })}/>
                        </SettingRow>
                        <SettingRow label="Campaign started">
                            <Toggle on={settings.notify.campaignStarted} onChange={v => set(c => { c.notify.campaignStarted = v; })}/>
                        </SettingRow>
                        <SettingRow label="Account not linked">
                            <Toggle on={settings.notify.accountNotLinked} onChange={v => set(c => { c.notify.accountNotLinked = v; })}/>
                        </SettingRow>
                    </div>
                    <div className="card settings-card">
                        <h3>Language</h3>
                        <div>
                            <Segmented value={settings.language || 'en'} onChange={v => set(c => { c.language = v; })} options={[
                                {value: 'en', label: 'English'},
                                {value: 'ru', label: 'Русский', disabled: true, title: 'Russian arrives in a later update'},
                            ]}/>
                        </div>
                    </div>
                    <Advanced/>
                </div>
            </div>
        </>
    );
}
