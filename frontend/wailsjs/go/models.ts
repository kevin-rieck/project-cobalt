export namespace main {
	
	export class ConnectionRequest {
	    endpoint: string;
	    securityPolicy: string;
	    securityMode: string;
	    authType: string;
	    username: string;
	    password: string;
	    clientCertificatePath: string;
	    clientPrivateKeyPath: string;
	    serverThumbprint: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = source["endpoint"];
	        this.securityPolicy = source["securityPolicy"];
	        this.securityMode = source["securityMode"];
	        this.authType = source["authType"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.clientCertificatePath = source["clientCertificatePath"];
	        this.clientPrivateKeyPath = source["clientPrivateKeyPath"];
	        this.serverThumbprint = source["serverThumbprint"];
	    }
	}
	export class DiagnosticLogEntry {
	    timestamp: string;
	    level: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticLogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.level = source["level"];
	        this.message = source["message"];
	    }
	}
	export class WatchlistRowView {
	    node: opcua.AddressNode;
	    value: opcua.LiveValue;
	    dataType: string;
	    engineeringUnit: string;
	    stale: boolean;
	    outOfRange: string;
	    updateCount: number;
	    error: string;
	    detailsError: string;
	
	    static createFrom(source: any = {}) {
	        return new WatchlistRowView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.node = this.convertValues(source["node"], opcua.AddressNode);
	        this.value = this.convertValues(source["value"], opcua.LiveValue);
	        this.dataType = source["dataType"];
	        this.engineeringUnit = source["engineeringUnit"];
	        this.stale = source["stale"];
	        this.outOfRange = source["outOfRange"];
	        this.updateCount = source["updateCount"];
	        this.error = source["error"];
	        this.detailsError = source["detailsError"];
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

export namespace opcua {
	
	export class AddressNode {
	    NodeID: string;
	    DisplayName: string;
	    BrowseName: string;
	    NodeClass: string;
	
	    static createFrom(source: any = {}) {
	        return new AddressNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.NodeID = source["NodeID"];
	        this.DisplayName = source["DisplayName"];
	        this.BrowseName = source["BrowseName"];
	        this.NodeClass = source["NodeClass"];
	    }
	}
	export class Endpoint {
	    URL: string;
	    SecurityPolicy: string;
	    SecurityMode: string;
	    SecurityLevel: number;
	    UserTokenTypes: string[];
	    ServerThumbprint: string;
	
	    static createFrom(source: any = {}) {
	        return new Endpoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.URL = source["URL"];
	        this.SecurityPolicy = source["SecurityPolicy"];
	        this.SecurityMode = source["SecurityMode"];
	        this.SecurityLevel = source["SecurityLevel"];
	        this.UserTokenTypes = source["UserTokenTypes"];
	        this.ServerThumbprint = source["ServerThumbprint"];
	    }
	}
	export class LiveValue {
	    NodeID: string;
	    Value: string;
	    Status: string;
	    // Go type: time
	    SourceTimestamp: any;
	    // Go type: time
	    ServerTimestamp: any;
	
	    static createFrom(source: any = {}) {
	        return new LiveValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.NodeID = source["NodeID"];
	        this.Value = source["Value"];
	        this.Status = source["Status"];
	        this.SourceTimestamp = this.convertValues(source["SourceTimestamp"], null);
	        this.ServerTimestamp = this.convertValues(source["ServerTimestamp"], null);
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

export namespace search {
	
	export class AddressSpaceSearchResult {
	    node: opcua.AddressNode;
	    matchKind: string;
	    matchText: string;
	    source: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new AddressSpaceSearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.node = this.convertValues(source["node"], opcua.AddressNode);
	        this.matchKind = source["matchKind"];
	        this.matchText = source["matchText"];
	        this.source = source["source"];
	        this.score = source["score"];
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
	export class AddressSpaceSearchView {
	    query: string;
	    results: AddressSpaceSearchResult[];
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new AddressSpaceSearchView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.results = this.convertValues(source["results"], AddressSpaceSearchResult);
	        this.status = source["status"];
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

export namespace session {
	
	export class SessionTrendNode {
	    node: opcua.AddressNode;
	    latestValue: string;
	    status: string;
	    pointCount: number;
	
	    static createFrom(source: any = {}) {
	        return new SessionTrendNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.node = this.convertValues(source["node"], opcua.AddressNode);
	        this.latestValue = source["latestValue"];
	        this.status = source["status"];
	        this.pointCount = source["pointCount"];
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
	export class SessionTrendPoint {
	    value: string;
	    status: string;
	    timestamp: string;
	    sourceTimestamp: string;
	    serverTimestamp: string;
	    receivedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionTrendPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.status = source["status"];
	        this.timestamp = source["timestamp"];
	        this.sourceTimestamp = source["sourceTimestamp"];
	        this.serverTimestamp = source["serverTimestamp"];
	        this.receivedAt = source["receivedAt"];
	    }
	}
	export class SessionTrendView {
	    nodes: SessionTrendNode[];
	    points: SessionTrendPoint[];
	
	    static createFrom(source: any = {}) {
	        return new SessionTrendView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], SessionTrendNode);
	        this.points = this.convertValues(source["points"], SessionTrendPoint);
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

