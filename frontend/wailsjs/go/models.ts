export namespace config {
	
	export class Twitch {
	    clientId: string;
	    userAgent: string;
	    hashes: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Twitch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.userAgent = source["userAgent"];
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

