import {ReactNode, useEffect, useRef, useState} from 'react';
import {Check, ChevronDown, Flame, Gamepad2, LucideIcon} from 'lucide-react';
import {gameColor} from '../lib/derive';
import {t} from '../i18n';

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'success';

export function Button({variant = 'secondary', size, icon: Icon, children, block, className = '', ...rest}: {
    variant?: Variant;
    size?: 'sm' | 'lg';
    icon?: LucideIcon;
    block?: boolean;
    children?: ReactNode;
} & React.ButtonHTMLAttributes<HTMLButtonElement>) {
    const iconOnly = Icon && !children;
    return (
        <button className={`btn ${variant} ${size ?? ''} ${block ? 'block' : ''} ${iconOnly ? 'icon' : ''} ${className}`} {...rest}>
            {Icon && <Icon size={size === 'sm' ? 14 : 16}/>}
            {children}
        </button>
    );
}

export type Tone = 'accent' | 'rust' | 'success' | 'warning' | 'error' | 'neutral';

export function Badge({tone = 'neutral', dot, children}: { tone?: Tone; dot?: boolean; children: ReactNode }) {
    return <span className={`badge ${tone}`}>{dot && <span className="bdot"/>}{children}</span>;
}

export function Soon() {
    return <span className="badge soon" title={t('common.soonTitle')}>{t('common.soon')}</span>;
}

export function Toggle({on, onChange, disabled}: { on: boolean; onChange(v: boolean): void; disabled?: boolean }) {
    return <button className={`toggle ${on ? 'on' : ''}`} disabled={disabled} aria-pressed={on} onClick={() => onChange(!on)}/>;
}

export function Progress({value, tone, thin}: { value: number; tone?: 'rust' | 'success'; thin?: boolean }) {
    return (
        <div className={`progress ${tone ?? ''} ${thin ? 'thin' : ''}`}>
            <div style={{width: `${Math.max(0, Math.min(100, value))}%`}}/>
        </div>
    );
}

export function Ring({value, color, children}: { value: number; color: string; children: ReactNode }) {
    const r = 62, c = 2 * Math.PI * r;
    return (
        <div className="ring">
            <svg viewBox="0 0 132 132">
                <circle cx="66" cy="66" r={r} fill="none" stroke="var(--surface-3)" strokeWidth="8"/>
                <circle cx="66" cy="66" r={r} fill="none" stroke={color} strokeWidth="8"
                        strokeDasharray={c} strokeDashoffset={c * (1 - Math.max(0, Math.min(100, value)) / 100)}
                        style={{transition: 'stroke-dashoffset 0.4s'}}/>
            </svg>
            <div className="ring-center">{children}</div>
        </div>
    );
}

export function Avatar({src, name, size = 32, online}: { src?: string; name: string; size?: number; online?: boolean }) {
    return (
        <div className="avatar" style={{width: size, height: size, fontSize: size * 0.4}}>
            {src ? <img src={src} alt=""/> : name.slice(0, 1).toUpperCase()}
            {online && <span className="online"/>}
        </div>
    );
}

export function Thumb({src, size = 56, className = ''}: { src?: string; size?: number; className?: string }) {
    return <div className={`thumb ${className}`} style={{width: size, height: size, backgroundImage: src ? `url("${src}")` : undefined}}/>;
}

export function GameIcon({name, size = 22}: { name: string; size?: number }) {
    const Icon = name.toLowerCase() === 'rust' ? Flame : Gamepad2;
    return (
        <span className="game-icon" style={{background: gameColor(name), width: size, height: size}}>
            <Icon size={Math.round(size * 0.6)}/>
        </span>
    );
}

export function Segmented<T extends string>({value, options, onChange}: {
    value: T;
    options: { value: T; label: string; icon?: LucideIcon; disabled?: boolean; title?: string }[];
    onChange(v: T): void;
}) {
    return (
        <div className="segmented">
            {options.map(o => (
                <button key={o.value} className={value === o.value ? 'active' : ''} disabled={o.disabled}
                        title={o.title} onClick={() => onChange(o.value)}>
                    {o.icon && <o.icon size={14}/>}{o.label}
                </button>
            ))}
        </div>
    );
}

export function Chip({active, count, children, onClick, disabled}: {
    active?: boolean; count?: number; children: ReactNode; onClick?(): void; disabled?: boolean;
}) {
    return (
        <button className={`chip ${active ? 'active' : ''}`} onClick={onClick} disabled={disabled}>
            {children}{count !== undefined && <span className="count">{count}</span>}
        </button>
    );
}

export function Banner({tone = 'warning', icon: Icon, title, children, action}: {
    tone?: 'warning' | 'error'; icon: LucideIcon; title: string; children?: ReactNode; action?: ReactNode;
}) {
    return (
        <div className={`banner ${tone}`}>
            <Icon size={16}/>
            <div className="grow">
                <div className="banner-title">{title}</div>
                {children && <div className="banner-text">{children}</div>}
            </div>
            {action}
        </div>
    );
}

export function Empty({icon: Icon, title, children, actions}: {
    icon: LucideIcon; title: string; children?: ReactNode; actions?: ReactNode;
}) {
    return (
        <div className="empty">
            <div className="empty-orb"><div><Icon size={22}/></div></div>
            <h3>{title}</h3>
            {children && <p>{children}</p>}
            {actions && <div className="row" style={{marginTop: 8}}>{actions}</div>}
        </div>
    );
}

/** The Twitch glitch mark (Simple Icons, CC0); lucide-react no longer ships brand icons. */
export function TwitchIcon({size = 16}: { size?: number }) {
    return (
        <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" aria-hidden>
            <path d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0 1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714Z"/>
        </svg>
    );
}

export interface SelectOption<T extends string> {
    value: T;
    label: string;
}

/** Calls close on a click outside ref or on Escape while open. */
export function useDismiss(ref: React.RefObject<HTMLElement | null>, open: boolean, close: () => void) {
    useEffect(() => {
        if (!open) return;
        const outside = (e: MouseEvent) => { if (!ref.current?.contains(e.target as Node)) close(); };
        const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') close(); };
        document.addEventListener('mousedown', outside);
        document.addEventListener('keydown', esc);
        return () => {
            document.removeEventListener('mousedown', outside);
            document.removeEventListener('keydown', esc);
        };
    }, [open]);
}

/** Dropdown in the app style; the native <select> popup ignores the dark theme in WebView2. */
export function Select<T extends string>({value, options, onChange, icon: Icon}: {
    value: T; options: SelectOption<T>[]; onChange(v: T): void; icon?: LucideIcon;
}) {
    const [open, setOpen] = useState(false);
    const ref = useRef<HTMLDivElement>(null);
    useDismiss(ref, open, () => setOpen(false));
    const current = options.find(o => o.value === value) ?? options[0];
    return (
        <div className="select" ref={ref}>
            <button className="btn secondary" onClick={() => setOpen(v => !v)} aria-expanded={open}>
                {Icon && <Icon size={14}/>}
                <span className="ellipsis">{current?.label}</span>
                <ChevronDown size={14} className="faint"/>
            </button>
            {open && (
                <div className="select-menu" role="listbox">
                    {options.map(o => (
                        <button key={o.value} role="option" aria-selected={o.value === value}
                                className={o.value === value ? 'selected' : ''}
                                onClick={() => { onChange(o.value); setOpen(false); }}>
                            <span className="grow ellipsis">{o.label}</span>
                            {o.value === value && <Check size={14}/>}
                        </button>
                    ))}
                </div>
            )}
        </div>
    );
}
