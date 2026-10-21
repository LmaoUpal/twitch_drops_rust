export namespace catalog {
	
	export class Snapshot {
	    campaigns: models.Campaign[];
	    // Go type: time
	    updatedAt: any;
	    warnings: string[];
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.campaigns = this.convertValues(source["campaigns"], models.Campaign);
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
	export class Config {
	    games: string[];
	    excludedChannels: string[];
	    autoClaim: boolean;
	    twitch: Twitch;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.games = source["games"];
	        this.excludedChannels = source["excludedChannels"];
	        this.autoClaim = source["autoClaim"];
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

export namespace main {
	
	export class AuthStatus {
	    state: string;
	    login: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.login = source["login"];
	        this.error = source["error"];
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
	
	

}

