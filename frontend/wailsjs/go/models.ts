export namespace main {
	
	export class ConnectionRequest {
	    endpoint: string;
	    securityPolicy: string;
	    securityMode: string;
	    authType: string;
	    username: string;
	    password: string;
	
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

