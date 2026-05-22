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
	    installed: boolean;

	    static createFrom(source: any = {}) {
	        return new BinaryInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.latest = source["latest"];
	        this.installed = source["installed"];
	    }
	}
	export class BinaryVersionInfo {
	    version: string;
	    size: number;
	    refs: number;
	    uploadedBy: string;
	    uploadedAt: string;
	    isCurrent: boolean;
	    isLocal: boolean;
	    isRemote: boolean;

	    static createFrom(source: any = {}) {
	        return new BinaryVersionInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.size = source["size"];
	        this.refs = source["refs"];
	        this.uploadedBy = source["uploadedBy"];
	        this.uploadedAt = source["uploadedAt"];
	        this.isCurrent = source["isCurrent"];
	        this.isLocal = source["isLocal"];
	        this.isRemote = source["isRemote"];
	    }
	}
	export class BinaryPageData {
	    currentVersion: string;
	    allVersions: BinaryVersionInfo[];
	    versions: BinaryVersionInfo[];
	    localVersions: BinaryVersionInfo[];
	    platform: string;
	    binaryPath: string;
	    managedPath: string;
	    binarySource: string;
	    binaryReadOnly: boolean;
	    binaryShim: boolean;
	    binaryError: string;
	    versionsDir: string;
	    localExists: boolean;

	    static createFrom(source: any = {}) {
	        return new BinaryPageData(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.allVersions = this.convertValues(source["allVersions"], BinaryVersionInfo);
	        this.versions = this.convertValues(source["versions"], BinaryVersionInfo);
	        this.localVersions = this.convertValues(source["localVersions"], BinaryVersionInfo);
	        this.platform = source["platform"];
	        this.binaryPath = source["binaryPath"];
	        this.managedPath = source["managedPath"];
	        this.binarySource = source["binarySource"];
	        this.binaryReadOnly = source["binaryReadOnly"];
	        this.binaryShim = source["binaryShim"];
	        this.binaryError = source["binaryError"];
	        this.versionsDir = source["versionsDir"];
	        this.localExists = source["localExists"];
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
	export class BinaryStorageInfo {
	    localTotal: number;
	    cloudTotal: number;
	    localCount: number;
	    cloudCount: number;

	    static createFrom(source: any = {}) {
	        return new BinaryStorageInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.localTotal = source["localTotal"];
	        this.cloudTotal = source["cloudTotal"];
	        this.localCount = source["localCount"];
	        this.cloudCount = source["cloudCount"];
	    }
	}

	export class BinaryView {
	    encrypt: boolean;
	    chunkMode: string;
	    chunkSizeMB: number;
	    chunkThresholdMB: number;
	    autoUpload: boolean;

	    static createFrom(source: any = {}) {
	        return new BinaryView(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.encrypt = source["encrypt"];
	        this.chunkMode = source["chunkMode"];
	        this.chunkSizeMB = source["chunkSizeMB"];
	        this.chunkThresholdMB = source["chunkThresholdMB"];
	        this.autoUpload = source["autoUpload"];
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
	export class ClaudeBinaryInfo {
	    platform: string;
	    platformLabel: string;
	    localVersion: string;
	    remoteVersion: string;
	    installed: boolean;
	    status: string;
	    statusLabel: string;

	    static createFrom(source: any = {}) {
	        return new ClaudeBinaryInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.platform = source["platform"];
	        this.platformLabel = source["platformLabel"];
	        this.localVersion = source["localVersion"];
	        this.remoteVersion = source["remoteVersion"];
	        this.installed = source["installed"];
	        this.status = source["status"];
	        this.statusLabel = source["statusLabel"];
	    }
	}
	export class ClaudeBinaryResolution {
	    currentPath: string;
	    managedPath: string;
	    source: string;
	    version: string;
	    valid: boolean;
	    readOnly: boolean;
	    isShim: boolean;
	    error?: string;

	    static createFrom(source: any = {}) {
	        return new ClaudeBinaryResolution(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentPath = source["currentPath"];
	        this.managedPath = source["managedPath"];
	        this.source = source["source"];
	        this.version = source["version"];
	        this.valid = source["valid"];
	        this.readOnly = source["readOnly"];
	        this.isShim = source["isShim"];
	        this.error = source["error"];
	    }
	}
	export class ClaudeDirectoryInfo {
	    name: string;
	    path: string;
	    pattern: string;
	    excluded: boolean;

	    static createFrom(source: any = {}) {
	        return new ClaudeDirectoryInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.pattern = source["pattern"];
	        this.excluded = source["excluded"];
	    }
	}
	export class ClaudeFileInfo {
	    name: string;
	    path: string;
	    pattern: string;
	    excluded: boolean;

	    static createFrom(source: any = {}) {
	        return new ClaudeFileInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.pattern = source["pattern"];
	        this.excluded = source["excluded"];
	    }
	}
	export class ConfigStatus {
	    ok: boolean;
	    webdavConfigured: boolean;
	    passwordAvailable: boolean;
	    claudeDirExists: boolean;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new ConfigStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.webdavConfigured = source["webdavConfigured"];
	        this.passwordAvailable = source["passwordAvailable"];
	        this.claudeDirExists = source["claudeDirExists"];
	        this.message = source["message"];
	    }
	}
	export class SyncView {
	    snapshotLimit: number;
	    conflictStrategy: string;
	    mergeRetryMax: number;
	    autoSyncInterval: string;

	    static createFrom(source: any = {}) {
	        return new SyncView(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.snapshotLimit = source["snapshotLimit"];
	        this.conflictStrategy = source["conflictStrategy"];
	        this.mergeRetryMax = source["mergeRetryMax"];
	        this.autoSyncInterval = source["autoSyncInterval"];
	    }
	}
	export class EncryptionView {
	    enabled: boolean;

	    static createFrom(source: any = {}) {
	        return new EncryptionView(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	    }
	}
	export class DeviceView {
	    id: string;
	    name: string;

	    static createFrom(source: any = {}) {
	        return new DeviceView(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class WebDAVView {
	    url: string;
	    username: string;
	    root: string;
	    baseUrl: string;
	    headUrl: string;
	    hasPassword: boolean;

	    static createFrom(source: any = {}) {
	        return new WebDAVView(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.username = source["username"];
	        this.root = source["root"];
	        this.baseUrl = source["baseUrl"];
	        this.headUrl = source["headUrl"];
	        this.hasPassword = source["hasPassword"];
	    }
	}
	export class ConfigView {
	    webdav: WebDAVView;
	    device: DeviceView;
	    encryption: EncryptionView;
	    binary: BinaryView;
	    sync: SyncView;
	    exclude: string[];
	    claudeDir: string;
	    claudeDirRaw: string;
	    claudeDirDefault: string;
	    claudeJSONPath: string;
	    claudeJSONPathRaw: string;
	    claudeJSONPathDefault: string;
	    binDir: string;
	    binDirRaw: string;
	    versionsDir: string;
	    versionsDirRaw: string;
	    claudeBinaryPath: string;
	    claudeBinaryPathRaw: string;
	    claudeBinaryManagedPath: string;
	    claudeBinaryPlaceholderPath: string;
	    claudeBinarySource: string;
	    claudeBinaryVersion: string;
	    claudeBinaryValid: boolean;
	    claudeBinaryReadOnly: boolean;
	    claudeBinaryShim: boolean;
	    claudeBinaryError: string;

	    static createFrom(source: any = {}) {
	        return new ConfigView(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.webdav = this.convertValues(source["webdav"], WebDAVView);
	        this.device = this.convertValues(source["device"], DeviceView);
	        this.encryption = this.convertValues(source["encryption"], EncryptionView);
	        this.binary = this.convertValues(source["binary"], BinaryView);
	        this.sync = this.convertValues(source["sync"], SyncView);
	        this.exclude = source["exclude"];
	        this.claudeDir = source["claudeDir"];
	        this.claudeDirRaw = source["claudeDirRaw"];
	        this.claudeDirDefault = source["claudeDirDefault"];
	        this.claudeJSONPath = source["claudeJSONPath"];
	        this.claudeJSONPathRaw = source["claudeJSONPathRaw"];
	        this.claudeJSONPathDefault = source["claudeJSONPathDefault"];
	        this.binDir = source["binDir"];
	        this.binDirRaw = source["binDirRaw"];
	        this.versionsDir = source["versionsDir"];
	        this.versionsDirRaw = source["versionsDirRaw"];
	        this.claudeBinaryPath = source["claudeBinaryPath"];
	        this.claudeBinaryPathRaw = source["claudeBinaryPathRaw"];
	        this.claudeBinaryManagedPath = source["claudeBinaryManagedPath"];
	        this.claudeBinaryPlaceholderPath = source["claudeBinaryPlaceholderPath"];
	        this.claudeBinarySource = source["claudeBinarySource"];
	        this.claudeBinaryVersion = source["claudeBinaryVersion"];
	        this.claudeBinaryValid = source["claudeBinaryValid"];
	        this.claudeBinaryReadOnly = source["claudeBinaryReadOnly"];
	        this.claudeBinaryShim = source["claudeBinaryShim"];
	        this.claudeBinaryError = source["claudeBinaryError"];
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
	export class ConflictDetail {
	    path: string;
	    local: string;
	    remote: string;
	    localModified: string;
	    remoteModified: string;
	    recommended: string;
	    localExists: boolean;
	    remoteExists: boolean;

	    static createFrom(source: any = {}) {
	        return new ConflictDetail(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.local = source["local"];
	        this.remote = source["remote"];
	        this.localModified = source["localModified"];
	        this.remoteModified = source["remoteModified"];
	        this.recommended = source["recommended"];
	        this.localExists = source["localExists"];
	        this.remoteExists = source["remoteExists"];
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
	export class ConnectionTest {
	    success: boolean;
	    error?: string;
	    latency: number;

	    static createFrom(source: any = {}) {
	        return new ConnectionTest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.error = source["error"];
	        this.latency = source["latency"];
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
	export class SyncHealth {
	    status: string;
	    code: string;
	    message: string;
	    canRepair: boolean;
	    localHead?: string;
	    remoteHead?: string;

	    static createFrom(source: any = {}) {
	        return new SyncHealth(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.code = source["code"];
	        this.message = source["message"];
	        this.canRepair = source["canRepair"];
	        this.localHead = source["localHead"];
	        this.remoteHead = source["remoteHead"];
	    }
	}
	export class DashboardData {
	    syncStatus: string;
	    syncHealth: SyncHealth;
	    lastSync: string;
	    claudeVersion: string;
	    claudeLatest: boolean;
	    claudeBinary: ClaudeBinaryInfo;
	    configStatus: ConfigStatus;
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
	        this.syncHealth = this.convertValues(source["syncHealth"], SyncHealth);
	        this.lastSync = source["lastSync"];
	        this.claudeVersion = source["claudeVersion"];
	        this.claudeLatest = source["claudeLatest"];
	        this.claudeBinary = this.convertValues(source["claudeBinary"], ClaudeBinaryInfo);
	        this.configStatus = this.convertValues(source["configStatus"], ConfigStatus);
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


	export class DiffHunk {
	    oldStart: number;
	    oldCount: number;
	    newStart: number;
	    newCount: number;
	    lines: string[];

	    static createFrom(source: any = {}) {
	        return new DiffHunk(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.oldStart = source["oldStart"];
	        this.oldCount = source["oldCount"];
	        this.newStart = source["newStart"];
	        this.newCount = source["newCount"];
	        this.lines = source["lines"];
	    }
	}
	export class DiffResult {
	    path: string;
	    local?: string;
	    remote?: string;
	    hunks?: DiffHunk[];
	    status: string;
	    localNew?: boolean;
	    remoteNew?: boolean;

	    static createFrom(source: any = {}) {
	        return new DiffResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.local = source["local"];
	        this.remote = source["remote"];
	        this.hunks = this.convertValues(source["hunks"], DiffHunk);
	        this.status = source["status"];
	        this.localNew = source["localNew"];
	        this.remoteNew = source["remoteNew"];
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
	export class EncryptionPasswordPreview {
	    status: string;
	    message: string;
	    fingerprint: string;
	    matchesCurrent: boolean;

	    static createFrom(source: any = {}) {
	        return new EncryptionPasswordPreview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.message = source["message"];
	        this.fingerprint = source["fingerprint"];
	        this.matchesCurrent = source["matchesCurrent"];
	    }
	}
	export class EncryptionStatus {
	    enabled: boolean;
	    fingerprint: string;
	    hasKey: boolean;

	    static createFrom(source: any = {}) {
	        return new EncryptionStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.fingerprint = source["fingerprint"];
	        this.hasKey = source["hasKey"];
	    }
	}
	export class EncryptionVerifyResult {
	    status: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new EncryptionVerifyResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}

	export class FileDetail {
	    path: string;
	    size: number;
	    modified: string;
	    status: string;
	    content?: string;
	    hash?: string;

	    static createFrom(source: any = {}) {
	        return new FileDetail(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.size = source["size"];
	        this.modified = source["modified"];
	        this.status = source["status"];
	        this.content = source["content"];
	        this.hash = source["hash"];
	    }
	}
	export class FileEntry {
	    hash: string;
	    size: number;
	    modified: string;

	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash = source["hash"];
	        this.size = source["size"];
	        this.modified = source["modified"];
	    }
	}
	export class FileFailure {
	    path: string;
	    fullPath: string;
	    error: string;

	    static createFrom(source: any = {}) {
	        return new FileFailure(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.fullPath = source["fullPath"];
	        this.error = source["error"];
	    }
	}
	export class FileNode {
	    name: string;
	    path: string;
	    fullPath?: string;
	    isDir: boolean;
	    status: string;
	    size: number;
	    modified: string;
	    error?: string;
	    children?: FileNode[];
	    expanded?: boolean;

	    static createFrom(source: any = {}) {
	        return new FileNode(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.fullPath = source["fullPath"];
	        this.isDir = source["isDir"];
	        this.status = source["status"];
	        this.size = source["size"];
	        this.modified = source["modified"];
	        this.error = source["error"];
	        this.children = this.convertValues(source["children"], FileNode);
	        this.expanded = source["expanded"];
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
	export class FileTreeResult {
	    root?: FileNode;
	    total: number;
	    changed: number;
	    conflicts: number;
	    failed: number;
	    failures?: FileFailure[];
	    checking?: boolean;

	    static createFrom(source: any = {}) {
	        return new FileTreeResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = this.convertValues(source["root"], FileNode);
	        this.total = source["total"];
	        this.changed = source["changed"];
	        this.conflicts = source["conflicts"];
	        this.failed = source["failed"];
	        this.failures = this.convertValues(source["failures"], FileFailure);
	        this.checking = source["checking"];
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
	export class OrphanInfo {
	    remote: string;
	    discovered: string;

	    static createFrom(source: any = {}) {
	        return new OrphanInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.remote = source["remote"];
	        this.discovered = source["discovered"];
	    }
	}
	export class ProjectInfo {
	    name: string;
	    path: string;
	    remote: string;
	    remoteName: string;
	    lastSync: string;
	    mcpCount: number;
	    hasLocal: boolean;
	    hasRemote: boolean;
	    isOrphan: boolean;

	    static createFrom(source: any = {}) {
	        return new ProjectInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.remote = source["remote"];
	        this.remoteName = source["remoteName"];
	        this.lastSync = source["lastSync"];
	        this.mcpCount = source["mcpCount"];
	        this.hasLocal = source["hasLocal"];
	        this.hasRemote = source["hasRemote"];
	        this.isOrphan = source["isOrphan"];
	    }
	}
	export class ProjectListResult {
	    projects: ProjectInfo[];
	    orphans: OrphanInfo[];

	    static createFrom(source: any = {}) {
	        return new ProjectListResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projects = this.convertValues(source["projects"], ProjectInfo);
	        this.orphans = this.convertValues(source["orphans"], OrphanInfo);
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
	export class SnapshotDetail {
	    id: string;
	    timestamp: string;
	    device: string;
	    message: string;
	    parent: string;
	    files: Record<string, FileEntry>;
	    binary: Record<string, any>;

	    static createFrom(source: any = {}) {
	        return new SnapshotDetail(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.device = source["device"];
	        this.message = source["message"];
	        this.parent = source["parent"];
	        this.files = this.convertValues(source["files"], FileEntry, true);
	        this.binary = source["binary"];
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
	export class SnapshotEntry {
	    id: string;
	    shortId: string;
	    parent: string;
	    timestamp: string;
	    device: string;
	    message: string;
	    fileCount: number;

	    static createFrom(source: any = {}) {
	        return new SnapshotEntry(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.shortId = source["shortId"];
	        this.parent = source["parent"];
	        this.timestamp = source["timestamp"];
	        this.device = source["device"];
	        this.message = source["message"];
	        this.fileCount = source["fileCount"];
	    }
	}



}

