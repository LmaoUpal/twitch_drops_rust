import {useEffect, useState} from 'react';
import './App.css';
import {GetConfig} from "../wailsjs/go/main/App";
import {config} from "../wailsjs/go/models";

function App() {
    const [cfg, setCfg] = useState<config.Config | null>(null);

    useEffect(() => {
        GetConfig().then(setCfg);
    }, []);

    return (
        <div id="App">
            <header>
                <h1>Twitch Drops Miner</h1>
                <span className="status">not logged in</span>
            </header>
            <main>
                <section>
                    <h2>Games (priority)</h2>
                    <ol>
                        {cfg?.games?.map(g => <li key={g}>{g}</li>)}
                    </ol>
                </section>
                <section>
                    <h2>Campaigns</h2>
                    <p className="muted">Login and campaign list arrive in phases 1–2.</p>
                </section>
            </main>
        </div>
    )
}

export default App
