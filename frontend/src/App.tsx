import {useEffect, useState} from 'react';
import './App.css';
import {GetAuthStatus, GetConfig, Logout} from "../wailsjs/go/main/App";
import {config, main} from "../wailsjs/go/models";
import {EventsOn} from "../wailsjs/runtime/runtime";
import Login from "./components/Login";

function App() {
    const [cfg, setCfg] = useState<config.Config | null>(null);
    const [auth, setAuth] = useState<main.AuthStatus>({state: 'checking', login: '', error: ''});

    useEffect(() => {
        GetConfig().then(setCfg);
        GetAuthStatus().then(setAuth);
        return EventsOn('auth:status', setAuth);
    }, []);

    return (
        <div id="App">
            <header>
                <h1>Twitch Drops Miner</h1>
                {auth.state === 'loggedIn' ? (
                    <span className="status">
                        {auth.login} <button className="btn secondary small" onClick={() => Logout()}>Log out</button>
                    </span>
                ) : (
                    <span className="status">{auth.state === 'checking' ? 'checking login…' : 'not logged in'}</span>
                )}
            </header>
            <main>
                {auth.state === 'checking' && <p className="muted">Checking saved login…</p>}
                {(auth.state === 'loggedOut' || auth.state === 'offline') && <Login status={auth}/>}
                {auth.state === 'loggedIn' && (
                    <>
                        <section>
                            <h2>Games (priority)</h2>
                            <ol>
                                {cfg?.games?.map(g => <li key={g}>{g}</li>)}
                            </ol>
                        </section>
                        <section>
                            <h2>Campaigns</h2>
                            <p className="muted">Campaign list arrives in phase 2.</p>
                        </section>
                    </>
                )}
            </main>
        </div>
    )
}

export default App
