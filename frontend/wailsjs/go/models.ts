export namespace model {
	
	export class AIConfig {
	    enabled: boolean;
	    baseUrl: string;
	    apiKey: string;
	    model: string;
	    timeoutSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new AIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.baseUrl = source["baseUrl"];
	        this.apiKey = source["apiKey"];
	        this.model = source["model"];
	        this.timeoutSeconds = source["timeoutSeconds"];
	    }
	}
	export class AITaskRequest {
	    task: string;
	    targetLanguage?: string;
	    tone?: string;
	
	    static createFrom(source: any = {}) {
	        return new AITaskRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.task = source["task"];
	        this.targetLanguage = source["targetLanguage"];
	        this.tone = source["tone"];
	    }
	}
	export class LyricLine {
	    startMs: number;
	    endMs: number;
	    text: string;
	    translation?: string;
	    romanization?: string;
	
	    static createFrom(source: any = {}) {
	        return new LyricLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.startMs = source["startMs"];
	        this.endMs = source["endMs"];
	        this.text = source["text"];
	        this.translation = source["translation"];
	        this.romanization = source["romanization"];
	    }
	}
	export class LyricDocument {
	    id: string;
	    trackId: string;
	    providerTrackId?: string;
	    source: string;
	    format: string;
	    language: string;
	    translated: boolean;
	    lines: LyricLine[];
	    ttml?: string;
	    amlxBase64?: string;
	    lastUpdated: string;
	
	    static createFrom(source: any = {}) {
	        return new LyricDocument(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.trackId = source["trackId"];
	        this.providerTrackId = source["providerTrackId"];
	        this.source = source["source"];
	        this.format = source["format"];
	        this.language = source["language"];
	        this.translated = source["translated"];
	        this.lines = this.convertValues(source["lines"], LyricLine);
	        this.ttml = source["ttml"];
	        this.amlxBase64 = source["amlxBase64"];
	        this.lastUpdated = source["lastUpdated"];
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
	export class AITaskResult {
	    task: string;
	    targetLanguage?: string;
	    lineCount: number;
	    durationMs: number;
	    updatedLyrics: LyricDocument;
	
	    static createFrom(source: any = {}) {
	        return new AITaskResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.task = source["task"];
	        this.targetLanguage = source["targetLanguage"];
	        this.lineCount = source["lineCount"];
	        this.durationMs = source["durationMs"];
	        this.updatedLyrics = this.convertValues(source["updatedLyrics"], LyricDocument);
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
	export class AMLLConfig {
	    enabled: boolean;
	    autoConnect: boolean;
	    url: string;
	    sendAudio: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AMLLConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.autoConnect = source["autoConnect"];
	        this.url = source["url"];
	        this.sendAudio = source["sendAudio"];
	    }
	}
	export class AMLLConnection {
	    enabled: boolean;
	    url: string;
	    status: string;
	    message: string;
	    connectedAt?: string;
	    lastMessageAt?: string;
	    messagesSent: number;
	    binaryMessagesSent: number;
	    bytesSent: number;
	
	    static createFrom(source: any = {}) {
	        return new AMLLConnection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.url = source["url"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.connectedAt = source["connectedAt"];
	        this.lastMessageAt = source["lastMessageAt"];
	        this.messagesSent = source["messagesSent"];
	        this.binaryMessagesSent = source["binaryMessagesSent"];
	        this.bytesSent = source["bytesSent"];
	    }
	}
	export class APIRequestTrace {
	    id: string;
	    method: string;
	    path: string;
	    status: number;
	    durationMs: number;
	    bytesSent: number;
	    remoteAddr: string;
	    time: string;
	
	    static createFrom(source: any = {}) {
	        return new APIRequestTrace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.method = source["method"];
	        this.path = source["path"];
	        this.status = source["status"];
	        this.durationMs = source["durationMs"];
	        this.bytesSent = source["bytesSent"];
	        this.remoteAddr = source["remoteAddr"];
	        this.time = source["time"];
	    }
	}
	export class SessionConfig {
	    autoSelect: boolean;
	    preferred: string[];
	    blacklist: string[];
	    whitelist: string[];
	
	    static createFrom(source: any = {}) {
	        return new SessionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.autoSelect = source["autoSelect"];
	        this.preferred = source["preferred"];
	        this.blacklist = source["blacklist"];
	        this.whitelist = source["whitelist"];
	    }
	}
	export class UIConfig {
	    minimizeToTray: boolean;
	    hideOnClose: boolean;
	    startHidden: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.minimizeToTray = source["minimizeToTray"];
	        this.hideOnClose = source["hideOnClose"];
	        this.startHidden = source["startHidden"];
	    }
	}
	export class LyricsConfig {
	    autoSearch: boolean;
	    cacheEnabled: boolean;
	    cacheDirectory: string;
	    localScanPaths: string[];
	    sources: string[];
	
	    static createFrom(source: any = {}) {
	        return new LyricsConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.autoSearch = source["autoSearch"];
	        this.cacheEnabled = source["cacheEnabled"];
	        this.cacheDirectory = source["cacheDirectory"];
	        this.localScanPaths = source["localScanPaths"];
	        this.sources = source["sources"];
	    }
	}
	export class AudioConfig {
	    enabled: boolean;
	    mode: string;
	    format: string;
	    sampleRate: number;
	    channels: number;
	    frameDurationMs: number;
	    includePcm: boolean;
	    includeFeatures: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AudioConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.mode = source["mode"];
	        this.format = source["format"];
	        this.sampleRate = source["sampleRate"];
	        this.channels = source["channels"];
	        this.frameDurationMs = source["frameDurationMs"];
	        this.includePcm = source["includePcm"];
	        this.includeFeatures = source["includeFeatures"];
	    }
	}
	export class ServerConfig {
	    enabled: boolean;
	    host: string;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new ServerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.host = source["host"];
	        this.port = source["port"];
	    }
	}
	export class AppConfig {
	    server: ServerConfig;
	    audio: AudioConfig;
	    lyrics: LyricsConfig;
	    amll: AMLLConfig;
	    ai: AIConfig;
	    ui: UIConfig;
	    session: SessionConfig;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.server = this.convertValues(source["server"], ServerConfig);
	        this.audio = this.convertValues(source["audio"], AudioConfig);
	        this.lyrics = this.convertValues(source["lyrics"], LyricsConfig);
	        this.amll = this.convertValues(source["amll"], AMLLConfig);
	        this.ai = this.convertValues(source["ai"], AIConfig);
	        this.ui = this.convertValues(source["ui"], UIConfig);
	        this.session = this.convertValues(source["session"], SessionConfig);
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
	export class PerformanceStats {
	    uptimeSeconds: number;
	    cpuPercent: number;
	    memoryAllocBytes: number;
	    memorySysBytes: number;
	    goroutines: number;
	    httpRequests: number;
	    httpErrors: number;
	    httpBytesSent: number;
	    webSocketBytesSent: number;
	    networkBytesSent: number;
	    webSocketMessages: number;
	    webSocketBinaryMessages: number;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new PerformanceStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uptimeSeconds = source["uptimeSeconds"];
	        this.cpuPercent = source["cpuPercent"];
	        this.memoryAllocBytes = source["memoryAllocBytes"];
	        this.memorySysBytes = source["memorySysBytes"];
	        this.goroutines = source["goroutines"];
	        this.httpRequests = source["httpRequests"];
	        this.httpErrors = source["httpErrors"];
	        this.httpBytesSent = source["httpBytesSent"];
	        this.webSocketBytesSent = source["webSocketBytesSent"];
	        this.networkBytesSent = source["networkBytesSent"];
	        this.webSocketMessages = source["webSocketMessages"];
	        this.webSocketBinaryMessages = source["webSocketBinaryMessages"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class AudioEnergyPoint {
	    sequence: number;
	    timestamp: string;
	    trackId: string;
	    positionMs: number;
	    durationMs: number;
	    rms: number;
	    peak: number;
	
	    static createFrom(source: any = {}) {
	        return new AudioEnergyPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sequence = source["sequence"];
	        this.timestamp = source["timestamp"];
	        this.trackId = source["trackId"];
	        this.positionMs = source["positionMs"];
	        this.durationMs = source["durationMs"];
	        this.rms = source["rms"];
	        this.peak = source["peak"];
	    }
	}
	export class WebSocketSeriesPoint {
	    time: string;
	    endpoint: string;
	    messages: number;
	    binaryMessages: number;
	    bytesSent: number;
	
	    static createFrom(source: any = {}) {
	        return new WebSocketSeriesPoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.endpoint = source["endpoint"];
	        this.messages = source["messages"];
	        this.binaryMessages = source["binaryMessages"];
	        this.bytesSent = source["bytesSent"];
	    }
	}
	export class WebSocketMessageTrace {
	    id: string;
	    endpoint: string;
	    messageType: string;
	    bytesSent: number;
	    binary: boolean;
	    time: string;
	
	    static createFrom(source: any = {}) {
	        return new WebSocketMessageTrace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.endpoint = source["endpoint"];
	        this.messageType = source["messageType"];
	        this.bytesSent = source["bytesSent"];
	        this.binary = source["binary"];
	        this.time = source["time"];
	    }
	}
	export class WebSocketStats {
	    endpoint: string;
	    activeClients: number;
	    messagesSent: number;
	    binaryMessagesSent: number;
	    bytesSent: number;
	    messagesPerMinute: number;
	    lastMessageType: string;
	    lastMessageAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new WebSocketStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = source["endpoint"];
	        this.activeClients = source["activeClients"];
	        this.messagesSent = source["messagesSent"];
	        this.binaryMessagesSent = source["binaryMessagesSent"];
	        this.bytesSent = source["bytesSent"];
	        this.messagesPerMinute = source["messagesPerMinute"];
	        this.lastMessageType = source["lastMessageType"];
	        this.lastMessageAt = source["lastMessageAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class AppMetrics {
	    webSockets: WebSocketStats[];
	    webSocketMessages: WebSocketMessageTrace[];
	    webSocketSeries: WebSocketSeriesPoint[];
	    api: APIRequestTrace[];
	    audioEnergy: AudioEnergyPoint[];
	    performance: PerformanceStats;
	
	    static createFrom(source: any = {}) {
	        return new AppMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.webSockets = this.convertValues(source["webSockets"], WebSocketStats);
	        this.webSocketMessages = this.convertValues(source["webSocketMessages"], WebSocketMessageTrace);
	        this.webSocketSeries = this.convertValues(source["webSocketSeries"], WebSocketSeriesPoint);
	        this.api = this.convertValues(source["api"], APIRequestTrace);
	        this.audioEnergy = this.convertValues(source["audioEnergy"], AudioEnergyPoint);
	        this.performance = this.convertValues(source["performance"], PerformanceStats);
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
	export class AudioFrame {
	    sequence: number;
	    timestamp: string;
	    trackId: string;
	    positionMs: number;
	    sampleRate: number;
	    channels: number;
	    format: string;
	    durationMs: number;
	    rms: number;
	    peak: number;
	    spectrum?: number[];
	    pcmBase64?: string;
	    provider: string;
	    providerMode: string;
	
	    static createFrom(source: any = {}) {
	        return new AudioFrame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sequence = source["sequence"];
	        this.timestamp = source["timestamp"];
	        this.trackId = source["trackId"];
	        this.positionMs = source["positionMs"];
	        this.sampleRate = source["sampleRate"];
	        this.channels = source["channels"];
	        this.format = source["format"];
	        this.durationMs = source["durationMs"];
	        this.rms = source["rms"];
	        this.peak = source["peak"];
	        this.spectrum = source["spectrum"];
	        this.pcmBase64 = source["pcmBase64"];
	        this.provider = source["provider"];
	        this.providerMode = source["providerMode"];
	    }
	}
	export class LogEntry {
	    time: string;
	    level: string;
	    source: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.level = source["level"];
	        this.source = source["source"];
	        this.message = source["message"];
	    }
	}
	export class ServiceStatus {
	    name: string;
	    status: string;
	    message: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ServiceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class WSClient {
	    id: string;
	    name: string;
	    remoteAddr: string;
	    connectedAt: string;
	    lastSeenAt: string;
	    latencyMs: number;
	
	    static createFrom(source: any = {}) {
	        return new WSClient(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.remoteAddr = source["remoteAddr"];
	        this.connectedAt = source["connectedAt"];
	        this.lastSeenAt = source["lastSeenAt"];
	        this.latencyMs = source["latencyMs"];
	    }
	}
	export class Session {
	    id: string;
	    name: string;
	    appId: string;
	    active: boolean;
	    available: boolean;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.appId = source["appId"];
	        this.active = source["active"];
	        this.available = source["available"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class LyricSearchCandidate {
	    id: string;
	    source: string;
	    providerTrackId: string;
	    title: string;
	    artist: string;
	    album: string;
	    durationMs: number;
	    score: number;
	    rank: number;
	
	    static createFrom(source: any = {}) {
	        return new LyricSearchCandidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.providerTrackId = source["providerTrackId"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.durationMs = source["durationMs"];
	        this.score = source["score"];
	        this.rank = source["rank"];
	    }
	}
	export class Playback {
	    state: string;
	    positionMs: number;
	    volume: number;
	    updatedAt: string;
	    canControl: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Playback(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.positionMs = source["positionMs"];
	        this.volume = source["volume"];
	        this.updatedAt = source["updatedAt"];
	        this.canControl = source["canControl"];
	    }
	}
	export class Track {
	    id: string;
	    title: string;
	    artist: string;
	    album: string;
	    sourceApp: string;
	    artwork: string;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Track(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.sourceApp = source["sourceApp"];
	        this.artwork = source["artwork"];
	        this.durationMs = source["durationMs"];
	    }
	}
	export class AppSnapshot {
	    version: string;
	    config: AppConfig;
	    track: Track;
	    playback: Playback;
	    lyrics: LyricDocument;
	    lyricCandidates: LyricSearchCandidate[];
	    amll: AMLLConnection;
	    sessions: Session[];
	    clients: WSClient[];
	    services: ServiceStatus[];
	    logs: LogEntry[];
	    audio: AudioFrame;
	    metrics: AppMetrics;
	
	    static createFrom(source: any = {}) {
	        return new AppSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.config = this.convertValues(source["config"], AppConfig);
	        this.track = this.convertValues(source["track"], Track);
	        this.playback = this.convertValues(source["playback"], Playback);
	        this.lyrics = this.convertValues(source["lyrics"], LyricDocument);
	        this.lyricCandidates = this.convertValues(source["lyricCandidates"], LyricSearchCandidate);
	        this.amll = this.convertValues(source["amll"], AMLLConnection);
	        this.sessions = this.convertValues(source["sessions"], Session);
	        this.clients = this.convertValues(source["clients"], WSClient);
	        this.services = this.convertValues(source["services"], ServiceStatus);
	        this.logs = this.convertValues(source["logs"], LogEntry);
	        this.audio = this.convertValues(source["audio"], AudioFrame);
	        this.metrics = this.convertValues(source["metrics"], AppMetrics);
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
	
	
	
	
	export class LyricCalibrationSuggestion {
	    lineIndex: number;
	    lineText: string;
	    currentStartMs: number;
	    suggestedPositionMs: number;
	    offsetMs: number;
	    confidence: number;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new LyricCalibrationSuggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lineIndex = source["lineIndex"];
	        this.lineText = source["lineText"];
	        this.currentStartMs = source["currentStartMs"];
	        this.suggestedPositionMs = source["suggestedPositionMs"];
	        this.offsetMs = source["offsetMs"];
	        this.confidence = source["confidence"];
	        this.reason = source["reason"];
	    }
	}
	export class LyricCalibrationResult {
	    suggestion: LyricCalibrationSuggestion;
	    updatedLyrics: LyricDocument;
	
	    static createFrom(source: any = {}) {
	        return new LyricCalibrationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.suggestion = this.convertValues(source["suggestion"], LyricCalibrationSuggestion);
	        this.updatedLyrics = this.convertValues(source["updatedLyrics"], LyricDocument);
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

