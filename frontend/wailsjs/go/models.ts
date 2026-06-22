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

}

