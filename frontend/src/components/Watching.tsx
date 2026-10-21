import {useEffect, useState} from 'react';
import {GetWatchState, StopWatching, WatchChannel} from "../../wailsjs/go/main/App";
import {main} from "../../wailsjs/go/models";
import {EventsOn} from "../../wailsjs/runtime/runtime";

export function watch(login: string) {
    return WatchChannel(login);
}

function Watching() {
    const [state, setState] = useState<main.WatchState | null>(null);
    const [login, setLogin] = useState('');
    const [error, setError] = useState('');

    useEffect(() => {
        GetWatchState().then(setState);
        return EventsOn('watch:status', setState);
    }, []);

    async function start() {
        setError('');
        try {
            await WatchChannel(login);
        } catch (e) {
            setError(String(e));
        }
    }

    const busy = state?.state === 'watching' || state?.state === 'starting';
    const ch = state?.channel;
    const s = state?.session;

    return (
        <section>
            <h2>Watching</h2>
            <div className="card">
                {busy && ch ? (
                    <>
                        <div className="row">
                            <span><span className="dot on"/>{ch.displayName || ch.login}</span>
                            <span className="muted small">{state!.state}</span>
                            <button className="btn secondary small" onClick={() => StopWatching()}>Stop</button>
                        </div>
                        <div className="muted small">
                            segments {state!.status.segments} · minute-watched {state!.status.spadeEvents}
                            {state!.status.error && <span className="error"> · {state!.status.error}</span>}
                        </div>
                        <div className="small">
                            {s?.dropId
                                ? <>Drop progress: {s.currentMinutes} / {s.requiredMinutes} min</>
                                : <span className="muted">No drop progressing on this channel yet (checked every minute).</span>}
                        </div>
                    </>
                ) : (
                    <>
                        {state?.state === 'offline' && <p className="muted small">{ch?.login} is offline.</p>}
                        {state?.state === 'error' && <p className="error small">{state.error}</p>}
                        <div className="row">
                            <input className="input" placeholder="channel login" value={login}
                                   onChange={e => setLogin(e.target.value)}
                                   onKeyDown={e => e.key === 'Enter' && login && start()}/>
                            <button className="btn" disabled={!login} onClick={start}>Watch</button>
                        </div>
                    </>
                )}
                {error && <p className="error small">{error}</p>}
            </div>
        </section>
    );
}

export default Watching
