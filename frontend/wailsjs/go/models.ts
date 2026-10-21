export namespace activity {
	
	export class Entry {
	    // Go type: time
	    time: any;
	    level: string;
	    message: string;
	    tag: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = this.convertValues(source["time"], null);
	        this.level = source["level"];
	        this.message = source["message"];
	        this.tag = source["tag"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace catalog {
	
	export class Snapshot {
	    campaigns: models.Campaign[];
	    claimed: models.ClaimedDrop[];
	    // Go type: time
	    updatedAt: any;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.campaigns = this.convertValues(source["campaigns"], models.Campaign);
	        this.claimed = this.convertValues(source["claimed"], models.ClaimedDrop);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace config {
	
	export class Twitch {
	    clientId: string;
	    userAgent: string;
	    clientUrl: string;
	    scopes: string[];
	    hashes: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Twitch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.userAgent = source["userAgent"];
	        this.clientUrl = source["clientUrl"];
	        this.scopes = source["scopes"];
	        this.hashes = source["hashes"];
	    }
	}
	export class Notify {
	    dropClaimed: boolean;
	    campaignStarted: boolean;
	    accountNotLinked: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Notify(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dropClaimed = source["dropClaimed"];
	        this.campaignStarted = source["campaignStarted"];
	        this.accountNotLinked = source["accountNotLinked"];
	    }
	}
	export class Config {
	    games: string[];
	    excludedChannels: string[];
	    autoMode: boolean;
	    autoClaim: boolean;
	    startWithWindows: boolean;
	    minimizeToTray: boolean;
	    notify: Notify;
	    language: string;
	    refreshSeconds: number;
	    twitch: Twitch;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.games = source["games"];
	        this.excludedChannels = source["excludedChannels"];
	        this.autoMode = source["autoMode"];
	        this.autoClaim = source["autoClaim"];
	        this.startWithWindows = source["startWithWindows"];
	        this.minimizeToTray = source["minimizeToTray"];
	        this.notify = this.convertValues(source["notify"], Notify);
	        this.language = source["language"];
	        this.refreshSeconds = source["refreshSeconds"];
	        this.twitch = this.convertValues(source["twitch"], Twitch);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace dropsapi {
	
	export class Drop {
	    id: string;
	    name: string;
	    imageUrl: string;
	    requiredMinutes: number;
	    requiredSubs: number;
	    // Go type: time
	    startAt: any;
	    // Go type: time
	    endAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Drop(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.imageUrl = source["imageUrl"];
	        this.requiredMinutes = source["requiredMinutes"];
	        this.requiredSubs = source["requiredSubs"];
	        this.startAt = this.convertValues(source["startAt"], null);
	        this.endAt = this.convertValues(source["endAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Channel {
	    id: string;
	    login: string;
	    displayName: string;
	
	    static createFrom(source: any = {}) {
	        return new Channel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.login = source["login"];
	        this.displayName = source["displayName"];
	    }
	}
	export class Game {
	    id: string;
	    name: string;
	    slug: string;
	    boxArtUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.slug = source["slug"];
	        this.boxArtUrl = source["boxArtUrl"];
	    }
	}
	export class Campaign {
	    id: string;
	    name: string;
	    owner: string;
	    game: Game;
	    status: string;
	    // Go type: time
	    startAt: any;
	    // Go type: time
	    endAt: any;
	    imageUrl: string;
	    detailsUrl: string;
	    accountLinkUrl: string;
	    channels: Channel[];
	    drops: Drop[];
	
	    static createFrom(source: any = {}) {
	        return new Campaign(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.owner = source["owner"];
	        this.game = this.convertValues(source["game"], Game);
	        this.status = source["status"];
	        this.startAt = this.convertValues(source["startAt"], null);
	        this.endAt = this.convertValues(source["endAt"], null);
	        this.imageUrl = source["imageUrl"];
	        this.detailsUrl = source["detailsUrl"];
	        this.accountLinkUrl = source["accountLinkUrl"];
	        this.channels = this.convertValues(source["channels"], Channel);
	        this.drops = this.convertValues(source["drops"], Drop);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	

}

export namespace gql {
	
	export class DropSession {
	    dropId: string;
	    channelId: string;
	    currentMinutes: number;
	    requiredMinutes: number;
	
	    static createFrom(source: any = {}) {
	        return new DropSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dropId = source["dropId"];
	        this.channelId = source["channelId"];
	        this.currentMinutes = source["currentMinutes"];
	        this.requiredMinutes = source["requiredMinutes"];
	    }
	}

}

export namespace main {
	
	export class AllDrops {
	    campaigns: dropsapi.Campaign[];
	    // Go type: time
	    updatedAt: any;
	    // Go type: time
	    fetchedAt: any;
	    source: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new AllDrops(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.campaigns = this.convertValues(source["campaigns"], dropsapi.Campaign);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.fetchedAt = this.convertValues(source["fetchedAt"], null);
	        this.source = source["source"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AuthStatus {
	    state: string;
	    login: string;
	    displayName: string;
	    avatarUrl: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.login = source["login"];
	        this.displayName = source["displayName"];
	        this.avatarUrl = source["avatarUrl"];
	        this.error = source["error"];
	    }
	}
	export class QueueItem {
	    channel: models.Channel;
	    gameName: string;
	    campaign: string;
	    drop: string;
	    dropCurrent: number;
	    dropNeeded: number;
	    minutesLeft: number;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new QueueItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.channel = this.convertValues(source["channel"], models.Channel);
	        this.gameName = source["gameName"];
	        this.campaign = source["campaign"];
	        this.drop = source["drop"];
	        this.dropCurrent = source["dropCurrent"];
	        this.dropNeeded = source["dropNeeded"];
	        this.minutesLeft = source["minutesLeft"];
	        this.reason = source["reason"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AutoState {
	    mode: string;
	    paused: boolean;
	    state: string;
	    exploring: string;
	    queue: QueueItem[];
	    // Go type: time
	    nextCheck: any;
	    lastError: string;
	
	    static createFrom(source: any = {}) {
	        return new AutoState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.paused = source["paused"];
	        this.state = source["state"];
	        this.exploring = source["exploring"];
	        this.queue = this.convertValues(source["queue"], QueueItem);
	        this.nextCheck = this.convertValues(source["nextCheck"], null);
	        this.lastError = source["lastError"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LoginPrompt {
	    userCode: string;
	    verificationUri: string;
	    expiresIn: number;
	
	    static createFrom(source: any = {}) {
	        return new LoginPrompt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.userCode = source["userCode"];
	        this.verificationUri = source["verificationUri"];
	        this.expiresIn = source["expiresIn"];
	    }
	}
	
	export class WatchState {
	    state: string;
	    channel: models.Channel;
	    status: watcher.Status;
	    session: gql.DropSession;
	    reason: string;
	    // Go type: time
	    started: any;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new WatchState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.channel = this.convertValues(source["channel"], models.Channel);
	        this.status = this.convertValues(source["status"], watcher.Status);
	        this.session = this.convertValues(source["session"], gql.DropSession);
	        this.reason = source["reason"];
	        this.started = this.convertValues(source["started"], null);
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace models {
	
	export class Drop {
	    id: string;
	    name: string;
	    benefits: string[];
	    requiredMinutes: number;
	    currentMinutes: number;
	    claimed: boolean;
	    instanceId: string;
	    imageUrl: string;
	    // Go type: time
	    startAt: any;
	    // Go type: time
	    endAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Drop(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.benefits = source["benefits"];
	        this.requiredMinutes = source["requiredMinutes"];
	        this.currentMinutes = source["currentMinutes"];
	        this.claimed = source["claimed"];
	        this.instanceId = source["instanceId"];
	        this.imageUrl = source["imageUrl"];
	        this.startAt = this.convertValues(source["startAt"], null);
	        this.endAt = this.convertValues(source["endAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Channel {
	    id: string;
	    login: string;
	    displayName: string;
	    online: boolean;
	    gameId: string;
	    dropsEnabled: boolean;
	    viewers: number;
	    broadcastId: string;
	    title: string;
	    gameName: string;
	    avatarUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Channel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.login = source["login"];
	        this.displayName = source["displayName"];
	        this.online = source["online"];
	        this.gameId = source["gameId"];
	        this.dropsEnabled = source["dropsEnabled"];
	        this.viewers = source["viewers"];
	        this.broadcastId = source["broadcastId"];
	        this.title = source["title"];
	        this.gameName = source["gameName"];
	        this.avatarUrl = source["avatarUrl"];
	    }
	}
	export class Game {
	    id: string;
	    name: string;
	    slug: string;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.slug = source["slug"];
	    }
	}
	export class Campaign {
	    id: string;
	    name: string;
	    game: Game;
	    // Go type: time
	    startAt: any;
	    // Go type: time
	    endAt: any;
	    accountLinked: boolean;
	    linkUrl: string;
	    source: string;
	    allowedChannels: Channel[];
	    drops: Drop[];
	
	    static createFrom(source: any = {}) {
	        return new Campaign(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.game = this.convertValues(source["game"], Game);
	        this.startAt = this.convertValues(source["startAt"], null);
	        this.endAt = this.convertValues(source["endAt"], null);
	        this.accountLinked = source["accountLinked"];
	        this.linkUrl = source["linkUrl"];
	        this.source = source["source"];
	        this.allowedChannels = this.convertValues(source["allowedChannels"], Channel);
	        this.drops = this.convertValues(source["drops"], Drop);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ClaimedDrop {
	    id: string;
	    name: string;
	    game: Game;
	    imageUrl: string;
	    // Go type: time
	    claimedAt: any;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new ClaimedDrop(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.game = this.convertValues(source["game"], Game);
	        this.imageUrl = source["imageUrl"];
	        this.claimedAt = this.convertValues(source["claimedAt"], null);
	        this.count = source["count"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace watcher {
	
	export class Status {
	    segments: number;
	    spadeEvents: number;
	    // Go type: time
	    lastPoll: any;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.segments = source["segments"];
	        this.spadeEvents = source["spadeEvents"];
	        this.lastPoll = this.convertValues(source["lastPoll"], null);
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

