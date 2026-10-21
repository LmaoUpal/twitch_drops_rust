import {useState} from 'react';
import {CancelLogin, Restore, StartLogin} from "../../wailsjs/go/main/App";
import {main} from "../../wailsjs/go/models";
import {BrowserOpenURL} from "../../wailsjs/runtime/runtime";

type Props = {
    status: main.AuthStatus;
};

function Login({status}: Props) {
    const [prompt, setPrompt] = useState<main.LoginPrompt | null>(null);
    const [error, setError] = useState('');
    const [starting, setStarting] = useState(false);

    async function start() {
        setError('');
        setStarting(true);
        try {
            const p = await StartLogin();
            setPrompt(p);
            BrowserOpenURL(p.verificationUri);
        } catch (e) {
            setError(String(e));
        } finally {
            setStarting(false);
        }
    }

    function cancel() {
        CancelLogin();
        setPrompt(null);
    }

    const shownError = error || status.error;

    return (
        <section className="login">
            {status.state === 'offline' && (
                <div className="card">
                    <p>Can't reach Twitch to check your saved login.</p>
                    <button className="btn" onClick={() => Restore()}>Retry</button>
                </div>
            )}
            {prompt ? (
                <div className="card">
                    <p>Enter this code on Twitch to sign in:</p>
                    <div className="user-code">{prompt.userCode}</div>
                    <p className="muted">Waiting for approval…</p>
                    <div className="row">
                        <button className="btn" onClick={() => BrowserOpenURL(prompt.verificationUri)}>Open twitch.tv/activate</button>
                        <button className="btn secondary" onClick={cancel}>Cancel</button>
                    </div>
                </div>
            ) : (
                <div className="card">
                    <p>Sign in with the Twitch account that should earn drops.</p>
                    <button className="btn" disabled={starting} onClick={start}>Log in with Twitch</button>
                </div>
            )}
            {shownError && <p className="error">{shownError}</p>}
        </section>
    );
}

export default Login
