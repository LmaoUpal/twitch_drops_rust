import {config, models} from "../../wailsjs/go/models";

export type Phase = 'upcoming' | 'active' | 'completed' | 'ended';
export type DropState = 'claimed' | 'ready' | 'progress' | 'locked' | 'unlinked';

export const RUST_GAME_ID = '263490';

export function campaignPhase(c: models.Campaign, now: number): Phase {
    if (now < Date.parse(c.startAt)) return 'upcoming';
    if (c.drops?.length && c.drops.every(d => d.claimed)) return 'completed';
    if (now >= Date.parse(c.endAt)) return 'ended';
    return 'active';
}

export function dropState(c: models.Campaign, d: models.Drop, now: number): DropState {
    if (d.claimed) return 'claimed';
    if (d.requiredMinutes > 0 && d.currentMinutes >= d.requiredMinutes) return 'ready';
    if (!c.accountLinked) return 'unlinked';
    if (now < Date.parse(c.startAt)) return 'locked';
    return 'progress';
}

export function isRust(game?: models.Game) {
    return game?.id === RUST_GAME_ID || game?.name?.toLowerCase() === 'rust';
}

const palette = ['#3B5BDB', '#2F9E44', '#E8590C', '#AE3EC9', '#1098AD', '#F08C00', '#5F3DC4'];

export function gameColor(name: string) {
    if (name.toLowerCase() === 'rust') return '#CD412B';
    let h = 0;
    for (const ch of name) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
    return palette[h % palette.length];
}

/** Facepunch streamer campaigns ("facepunch-54-<login>") belong to the general one. */
export function parentCampaignId(c: models.Campaign) {
    const m = c.id.match(/^(facepunch-[^-]+)-(.+)$/);
    if (!m || m[2] === 'general') return null;
    return `${m[1]}-general`;
}

export function findDrop(campaigns: models.Campaign[], dropId: string) {
    for (const c of campaigns) {
        const d = c.drops?.find(x => x.id === dropId);
        if (d) return {campaign: c, drop: d};
    }
    return null;
}

export function coverImage(c: models.Campaign) {
    return c.drops?.find(d => d.imageUrl)?.imageUrl ?? '';
}

export interface QueueItem {
    channel: models.Channel;
    campaign: models.Campaign;
    drop?: models.Drop;
    reason: string;
    skipped?: string;
}

/** Live channels on campaign allow-lists, as candidates to watch next. */
export function upNext(campaigns: models.Campaign[], now: number, currentLogin?: string): QueueItem[] {
    const seen = new Set<string>();
    const items: QueueItem[] = [];
    for (const c of campaigns) {
        if (campaignPhase(c, now) !== 'active') continue;
        for (const ch of c.allowedChannels ?? []) {
            const login = ch.login.toLowerCase();
            if (!ch.online || login === currentLogin || seen.has(login)) continue;
            seen.add(login);
            const drop = c.drops?.find(d => !d.claimed);
            items.push({
                channel: ch, campaign: c, drop,
                reason: 'streamer drop',
                skipped: c.accountLinked ? undefined : 'account not linked',
            });
        }
    }
    return items.sort((a, b) => Number(!!a.skipped) - Number(!!b.skipped));
}

/** The user's streamer queue for a game: logins watched first, in order (game matched case-insensitively). */
export function streamerQueue(settings: config.Config | null, game: string): string[] {
    const queues = settings?.preferredChannels ?? {};
    const key = Object.keys(queues).find(k => k.toLowerCase() === game.toLowerCase());
    return key ? queues[key] ?? [] : [];
}

/** Replaces the streamer queue of a game inside a settings draft. */
export function setStreamerQueue(c: config.Config, game: string, queue: string[]) {
    const queues = {...(c.preferredChannels ?? {})};
    for (const k of Object.keys(queues)) if (k.toLowerCase() === game.toLowerCase()) delete queues[k];
    if (queue.length) queues[game] = queue;
    c.preferredChannels = queues;
}
