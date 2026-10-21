import {useEffect, useRef, useState} from 'react';
import {CircleCheck, CircleX, Copy, Download, Info, ScrollText, Search, TriangleAlert} from 'lucide-react';
import {ExportActivity} from '../../wailsjs/go/main/App';
import {activity} from '../../wailsjs/go/models';
import {ClipboardSetText} from '../../wailsjs/runtime/runtime';
import {useStore} from '../store';
import {fmtTime} from '../lib/format';
import {Button, Chip, Empty} from '../components/ui';

type Level = 'all' | 'info' | 'ok' | 'warn' | 'error';

const levelView: Record<string, { icon: typeof Info; label: string }> = {
    info: {icon: Info, label: 'INFO'},
    ok: {icon: CircleCheck, label: 'OK'},
    warn: {icon: TriangleAlert, label: 'WARN'},
    error: {icon: CircleX, label: 'ERROR'},
};

const dayFmt = new Intl.DateTimeFormat('en-US', {weekday: 'long', month: 'short', day: 'numeric'});

function asText(entries: activity.Entry[]) {
    return entries.map(e => `${fmtTime(e.time)}  ${e.level.toUpperCase().padEnd(5)}  ${e.message}${e.tag ? `  [${e.tag}]` : ''}`).join('\n');
}

export default function Activity() {
    const {activity: entries} = useStore();
    const [level, setLevel] = useState<Level>('all');
    const [query, setQuery] = useState('');
    const [note, setNote] = useState('');
    const listRef = useRef<HTMLDivElement>(null);

    const counts: Record<Level, number> = {all: entries.length, info: 0, ok: 0, warn: 0, error: 0};
    entries.forEach(e => counts[e.level as Level]++);
    const q = query.trim().toLowerCase();
    const shown = entries
        .filter(e => level === 'all' || e.level === level)
        .filter(e => !q || e.message.toLowerCase().includes(q) || e.tag.toLowerCase().includes(q))
        .slice().reverse();

    const days = new Map<string, activity.Entry[]>();
    shown.forEach(e => {
        const d = new Date(e.time);
        const label = d.toDateString() === new Date().toDateString() ? `Today · ${dayFmt.format(d)}` : dayFmt.format(d);
        days.set(label, [...(days.get(label) ?? []), e]);
    });

    useEffect(() => { listRef.current?.scrollTo({top: 0}); }, [entries.length]);

    function flash(text: string) {
        setNote(text);
        setTimeout(() => setNote(''), 2000);
    }

    return (
        <>
            <div className="page-header">
                <div className="grow">
                    <h1 className="page-title">Activity log</h1>
                    <div className="page-subtitle">{note || `This session · ${entries.length} events`}</div>
                </div>
                <Button variant="ghost" icon={Download} onClick={async () => {
                    const path = await ExportActivity().catch(e => { flash(String(e)); return ''; });
                    if (path) flash(`Saved to ${path}`);
                }}>Export .txt</Button>
                <Button icon={Copy} onClick={() => ClipboardSetText(asText(shown.slice().reverse())).then(() => flash('Copied to clipboard'))}>Copy log</Button>
            </div>
            <div className="toolbar">
                {(['all', 'info', 'ok', 'warn', 'error'] as Level[]).map(l => (
                    <Chip key={l} active={level === l} count={counts[l]} onClick={() => setLevel(l)}>
                        {{all: 'All', info: 'Info', ok: 'Success', warn: 'Warnings', error: 'Errors'}[l]}
                    </Chip>
                ))}
                <span className="grow"/>
                <label className="input" style={{width: 220}}>
                    <Search size={14}/>
                    <input placeholder="Filter messages…" value={query} onChange={e => setQuery(e.target.value)}/>
                </label>
            </div>
            <div className="card log">
                {shown.length === 0 ? <Empty icon={ScrollText} title="No events">Events from this session appear here as they happen.</Empty> : (
                    <div ref={listRef} style={{overflowY: 'auto', flex: 1}}>
                        {[...days.entries()].map(([day, list], i) => (
                            <div key={day}>
                                <div className="log-day">
                                    <span>{day}</span>
                                    {i === 0 && <span className="row" style={{gap: 6, fontWeight: 500}}><span className="dot success" style={{width: 6, height: 6}}/>Following live</span>}
                                </div>
                                {list.map((e, n) => {
                                    const v = levelView[e.level] ?? levelView.info;
                                    return (
                                        <div key={`${e.time}-${n}`} className={`log-row ${e.level}`}>
                                            <span className="log-time">{fmtTime(e.time)}</span>
                                            <span className={`log-level ${e.level}`}><v.icon size={12}/>{v.label}</span>
                                            <span className="log-msg">{e.message}</span>
                                            {e.tag && <span className="log-tag">{e.tag}</span>}
                                        </div>
                                    );
                                })}
                            </div>
                        ))}
                    </div>
                )}
            </div>
        </>
    );
}
