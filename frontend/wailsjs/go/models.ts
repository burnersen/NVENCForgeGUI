export namespace main {
	
	export class ConfigView {
	    found: boolean;
	    path: string;
	    note: string;
	    maxResolution: number;
	    maxBitrate1080p: number;
	    maxBitrateOriginal: number;
	    av1MaxBitrate1080p: number;
	    av1MaxBitrateOriginal: number;
	    targetCQ: number;
	    av1TargetCQ: number;
	    autoCQTargetVMAF: number;
	    autoCQ: boolean;
	    autoCQKnown: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConfigView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.found = source["found"];
	        this.path = source["path"];
	        this.note = source["note"];
	        this.maxResolution = source["maxResolution"];
	        this.maxBitrate1080p = source["maxBitrate1080p"];
	        this.maxBitrateOriginal = source["maxBitrateOriginal"];
	        this.av1MaxBitrate1080p = source["av1MaxBitrate1080p"];
	        this.av1MaxBitrateOriginal = source["av1MaxBitrateOriginal"];
	        this.targetCQ = source["targetCQ"];
	        this.av1TargetCQ = source["av1TargetCQ"];
	        this.autoCQTargetVMAF = source["autoCQTargetVMAF"];
	        this.autoCQ = source["autoCQ"];
	        this.autoCQKnown = source["autoCQKnown"];
	    }
	}
	export class ConverterStatus {
	    found: boolean;
	    path: string;
	    sizeBytes: number;
	    version: string;
	    eventChannel: boolean;
	    toolsDir: string;
	    ffmpegPresent: boolean;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new ConverterStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.found = source["found"];
	        this.path = source["path"];
	        this.sizeBytes = source["sizeBytes"];
	        this.version = source["version"];
	        this.eventChannel = source["eventChannel"];
	        this.toolsDir = source["toolsDir"];
	        this.ffmpegPresent = source["ffmpegPresent"];
	        this.note = source["note"];
	    }
	}
	export class DownloadResult {
	    status: ConverterStatus;
	    replaced: boolean;
	    tag: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = this.convertValues(source["status"], ConverterStatus);
	        this.replaced = source["replaced"];
	        this.tag = source["tag"];
	        this.message = source["message"];
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
	export class GPUInfo {
	    detected: boolean;
	    name: string;
	    driver: string;
	    memoryMB: number;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new GPUInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.detected = source["detected"];
	        this.name = source["name"];
	        this.driver = source["driver"];
	        this.memoryMB = source["memoryMB"];
	        this.note = source["note"];
	    }
	}
	export class QueueItem {
	    path: string;
	    name: string;
	    folder: string;
	    sizeMB: number;
	    missing: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QueueItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.folder = source["folder"];
	        this.sizeMB = source["sizeMB"];
	        this.missing = source["missing"];
	    }
	}
	export class RunRequest {
	    files: string[];
	    codec: string;
	    encoder: string;
	    container: string;
	    resolution: string;
	    audio: string;
	    bitDepth: string;
	    quality: string;
	    fixedCQ: number;
	    maxBitrate: number;
	    keepSource: boolean;
	    shutdown: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RunRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = source["files"];
	        this.codec = source["codec"];
	        this.encoder = source["encoder"];
	        this.container = source["container"];
	        this.resolution = source["resolution"];
	        this.audio = source["audio"];
	        this.bitDepth = source["bitDepth"];
	        this.quality = source["quality"];
	        this.fixedCQ = source["fixedCQ"];
	        this.maxBitrate = source["maxBitrate"];
	        this.keepSource = source["keepSource"];
	        this.shutdown = source["shutdown"];
	    }
	}
	export class StartupInfo {
	    gpu: GPUInfo;
	    converter: ConverterStatus;
	    config: ConfigView;
	
	    static createFrom(source: any = {}) {
	        return new StartupInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gpu = this.convertValues(source["gpu"], GPUInfo);
	        this.converter = this.convertValues(source["converter"], ConverterStatus);
	        this.config = this.convertValues(source["config"], ConfigView);
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

