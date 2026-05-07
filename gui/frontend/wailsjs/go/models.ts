export namespace main {
	
	export class BackupInfo {
	    id: string;
	    message: string;
	    device: string;
	    time: string;
	
	    static createFrom(source: any = {}) {
	        return new BackupInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.message = source["message"];
	        this.device = source["device"];
	        this.time = source["time"];
	    }
	}
	export class BinaryInfo {
	    name: string;
	    version: string;
	    latest: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BinaryInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.latest = source["latest"];
	    }
	}
	export class ChangeInfo {
	    status: string;
	    path: string;
	    time: string;
	
	    static createFrom(source: any = {}) {
	        return new ChangeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.path = source["path"];
	        this.time = source["time"];
	    }
	}
	export class ConflictRef {
	    path: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new ConflictRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.detail = source["detail"];
	    }
	}
	export class DeviceInfo {
	    name: string;
	    platform: string;
	    version: string;
	    lastActive: string;
	    isCurrent: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DeviceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.platform = source["platform"];
	        this.version = source["version"];
	        this.lastActive = source["lastActive"];
	        this.isCurrent = source["isCurrent"];
	    }
	}
	export class DashboardData {
	    syncStatus: string;
	    lastSync: string;
	    claudeVersion: string;
	    claudeLatest: boolean;
	    conflicts: number;
	    conflictFiles: ConflictRef[];
	    devices: DeviceInfo[];
	    recentChanges: ChangeInfo[];
	    backups: BackupInfo[];
	    binaries: BinaryInfo[];
	
	    static createFrom(source: any = {}) {
	        return new DashboardData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.syncStatus = source["syncStatus"];
	        this.lastSync = source["lastSync"];
	        this.claudeVersion = source["claudeVersion"];
	        this.claudeLatest = source["claudeLatest"];
	        this.conflicts = source["conflicts"];
	        this.conflictFiles = this.convertValues(source["conflictFiles"], ConflictRef);
	        this.devices = this.convertValues(source["devices"], DeviceInfo);
	        this.recentChanges = this.convertValues(source["recentChanges"], ChangeInfo);
	        this.backups = this.convertValues(source["backups"], BackupInfo);
	        this.binaries = this.convertValues(source["binaries"], BinaryInfo);
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

