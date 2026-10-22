import {useEffect, useState} from 'react';
import {ArrowLeft, CircleX, Clipboard, Copy, ExternalLink, LoaderCircle, Lock, Pickaxe, RefreshCw, ShieldAlert, WifiOff} from 'lucide-react';
import {CancelLogin, Restore, StartLogin, SwitchAccount} from '../../wailsjs/go/main/App';
import {main} from '../../wailsjs/go/models';
import {BrowserOpenURL, ClipboardSetText, EventsOn} from '../../wailsjs/runtime/runtime';
import {useStore} from '../store';
import {fmtCountdown} from '../lib/format';
import {Avatar, Button, TwitchIcon} from '../components/ui';
import {t} from '../i18n';

function formatCode(code: string) {
    return code.length === 8 ? `${code.slice(0, 4)}-${code.slice(4)}` : code;
}

/** The sign-in screen. With onCancel it adds another account while one stays signed in. */
export default function Login({onCancel}: { onCancel?: () => void }) {
    const {auth, accounts, now} = useStore();
    const adding = !!onCancel;
    const others = adding ? [] : accounts.filter(a => !a.active);
    const [prompt, setPrompt] = useState<main.LoginPrompt | null>(null);
    const [expiresAt, setExpiresAt] = useState(0);
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);

    // A failed poll comes back as a loggedOut status with an error.
    useEffect(() => {
        if (auth.error) setError(auth.error);
    }, [auth.error]);
    // While another account is signed in the auth status stays loggedIn, so failures come separately.
    useEffect(() => EventsOn('login:error', (e: string) => setError(e)), []);

    const expired = !!prompt && (now >= expiresAt || /expired/i.test(error));
    const offline = !adding && auth.state === 'offline';
    const unreachable = offline || /dial|timeout|no such host|connect/i.test(error);

    async function start() {
        setError('');
        setBusy(true);
        try {
            const p = await StartLogin();
            setPrompt(p);
            setExpiresAt(Date.now() + p.expiresIn * 1000);
            BrowserOpenURL(p.verificationUri);
        } catch (e) {
            setError(String(e));
            setPrompt(null);
        } finally {
            setBusy(false);
        }
    }

    function cancel() {
        CancelLogin();
        setPrompt(null);
        setError('');
    }

    function back() {
        CancelLogin();
        onCancel?.();
    }

    let card;
    if (prompt) {
        card = (
            <>
                <h1>{t('login.authorize')}</h1>
                <p>{t('login.enterCode')}</p>
                <div className={`code-box ${expired ? 'expired' : ''}`}>
                    {formatCode(prompt.userCode)}
                    <Button size="sm" icon={Copy} title={t('login.copyCode')} onClick={() => ClipboardSetText(prompt.userCode)}/>
                </div>
                {expired ? (
                    <>
                        <div className="status-line error">
                            <CircleX size={16}/>
                            <span className="grow">{t('login.expired')}</span>
                            <span className="mono" style={{fontSize: 12, color: 'var(--error-text)'}}>00:00</span>
                        </div>
                        <Button variant="primary" size="lg" block icon={RefreshCw} disabled={busy} onClick={start}>{t('login.newCode')}</Button>
                    </>
                ) : (
                    <>
                        <Button variant="primary" size="lg" block icon={ExternalLink} onClick={() => BrowserOpenURL(prompt.verificationUri)}>{t('login.open')}</Button>
                        {error ? (
                            <div className="status-line error"><CircleX size={16}/><span className="grow">{error}</span></div>
                        ) : (
                            <div className="status-line">
                                <LoaderCircle size={16} className="spin" color="var(--accent-text)"/>
                                <span className="grow">{t('login.waiting')}</span>
                                <span className="mono faint" style={{fontSize: 12}}>{t('login.expiresIn', {t: fmtCountdown(expiresAt - now).replace(/^00:/, '')})}</span>
                            </div>
                        )}
                    </>
                )}
                <button className="link-btn" onClick={cancel}>{t('common.cancel')}</button>
            </>
        );
    } else {
        card = (
            <>
                <h1>{adding ? t('acc.addTitle') : t('login.title')}</h1>
                <p>{adding ? t('acc.addIntro') : t('login.intro')}</p>
                {(error || offline) && (
                    <div className="status-line error" style={{alignItems: 'flex-start', padding: 14}}>
                        {unreachable ? <WifiOff size={16}/> : <CircleX size={16}/>}
                        <div className="grow">
                            <div style={{fontWeight: 600}}>{unreachable ? t('login.cantReach') : t('login.failed')}</div>
                            <div className="muted" style={{fontSize: 12, marginTop: 2}}>{error || auth.error || t('login.checkConn')}</div>
                        </div>
                    </div>
                )}
                {offline ? (
                    <Button variant="primary" size="lg" block icon={RefreshCw} onClick={() => Restore()}>{t('common.retry')}</Button>
                ) : (
                    <Button variant="primary" size="lg" block disabled={busy} onClick={start}>
                        {busy ? <LoaderCircle size={16} className="spin"/> : <TwitchIcon/>}
                        {error ? t('common.retry') : t('login.login')}
                    </Button>
                )}
                {error || offline ? (
                    <button className="link-btn" onClick={() => ClipboardSetText(error || auth.error)}><Clipboard size={14}/>{t('login.copyErr')}</button>
                ) : (
                    <span className="row faint" style={{fontSize: 12, gap: 6}}><Lock size={12}/>{t('login.password')}</span>
                )}
                {others.length > 0 && (
                    <div className="saved-accounts">
                        <span className="label">{t('acc.saved')}</span>
                        {others.map(a => (
                            <Button key={a.key} block onClick={() => SwitchAccount(a.key)}>
                                <Avatar src={a.avatarUrl} name={a.login} size={20}/>
                                <span className="grow ellipsis" style={{textAlign: 'left'}}>{a.displayName || a.login}</span>
                            </Button>
                        ))}
                    </div>
                )}
                {adding && <button className="link-btn" onClick={back}><ArrowLeft size={14}/>{t('acc.back')}</button>}
            </>
        );
    }

    return (
        <div className="login-screen">
            <div className="login-card">
                <div className="login-logo"><Pickaxe size={28}/></div>
                {card}
            </div>
            <div className="login-footnote"><ShieldAlert size={13}/>{t('login.tos')}</div>
        </div>
    );
}
