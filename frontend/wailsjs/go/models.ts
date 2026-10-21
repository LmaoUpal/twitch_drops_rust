export namespace config {
	
	export class Twitch {
	    clientId: string;
	    userAgent: string;
	    scopes: string[];
	    hashes: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Twitch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.userAgent = source["userAgent"];
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

