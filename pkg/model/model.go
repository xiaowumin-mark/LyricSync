package model

import "time"

const AppVersion = "0.1.0-dev"

type AppConfig struct {
	Server  ServerConfig  `json:"server"`
	Audio   AudioConfig   `json:"audio"`
	Lyrics  LyricsConfig  `json:"lyrics"`
	AMLL    AMLLConfig    `json:"amll"`
	AI      AIConfig      `json:"ai"`
	UI      UIConfig      `json:"ui"`
	Session SessionConfig `json:"session"`
}

type ServerConfig struct {
	Enabled bool   `json:"enabled"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
}

type AudioConfig struct {
	Enabled         bool   `json:"enabled"`
	Mode            string `json:"mode"`
	Format          string `json:"format"`
	SampleRate      int    `json:"sampleRate"`
	Channels        int    `json:"channels"`
	FrameDurationMs int    `json:"frameDurationMs"`
	IncludePCM      bool   `json:"includePcm"`
	IncludeFeatures bool   `json:"includeFeatures"`
}

type LyricsConfig struct {
	AutoSearch     bool     `json:"autoSearch"`
	CacheEnabled   bool     `json:"cacheEnabled"`
	CacheDirectory string   `json:"cacheDirectory"`
	LocalScanPaths []string `json:"localScanPaths"`
	Sources        []string `json:"sources"`
}

type AMLLConfig struct {
	Enabled     bool   `json:"enabled"`
	AutoConnect bool   `json:"autoConnect"`
	URL         string `json:"url"`
	SendAudio   bool   `json:"sendAudio"`
}

type AIConfig struct {
	Enabled bool   `json:"enabled"`
	BaseURL string `json:"baseUrl"`
	APIKey  string `json:"apiKey"`
	Model   string `json:"model"`
	Timeout int    `json:"timeoutSeconds"`
}

type UIConfig struct {
	MinimizeToTray bool `json:"minimizeToTray"`
	HideOnClose    bool `json:"hideOnClose"`
	StartHidden    bool `json:"startHidden"`
}

type SessionConfig struct {
	AutoSelect bool     `json:"autoSelect"`
	Preferred  []string `json:"preferred"`
	Blacklist  []string `json:"blacklist"`
	Whitelist  []string `json:"whitelist"`
}

type HealthResponse struct {
	OK       bool            `json:"ok"`
	Status   string          `json:"status"`
	Version  string          `json:"version"`
	Time     string          `json:"time"`
	Services []ServiceStatus `json:"services"`
}

type Track struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Album     string `json:"album"`
	SourceApp string `json:"sourceApp"`
	Artwork   string `json:"artwork"`
	Duration  int64  `json:"durationMs"`
}

type Playback struct {
	State      string  `json:"state"`
	Position   int64   `json:"positionMs"`
	Volume     float64 `json:"volume"`
	UpdatedAt  string  `json:"updatedAt"`
	CanControl bool    `json:"canControl"`
}

type LyricDocument struct {
	ID              string      `json:"id"`
	TrackID         string      `json:"trackId"`
	ProviderTrackID string      `json:"providerTrackId,omitempty"`
	Source          string      `json:"source"`
	Format          string      `json:"format"`
	Language        string      `json:"language"`
	Translated      bool        `json:"translated"`
	Lines           []LyricLine `json:"lines"`
	TTML            string      `json:"ttml,omitempty"`
	AMLXBase64      string      `json:"amlxBase64,omitempty"`
	LastUpdated     string      `json:"lastUpdated"`
}

type LyricLine struct {
	StartMs      int64  `json:"startMs"`
	EndMs        int64  `json:"endMs"`
	Text         string `json:"text"`
	Translation  string `json:"translation,omitempty"`
	Romanization string `json:"romanization,omitempty"`
}

type LyricSearchCandidate struct {
	ID              string `json:"id"`
	Source          string `json:"source"`
	ProviderTrackID string `json:"providerTrackId"`
	Title           string `json:"title"`
	Artist          string `json:"artist"`
	Album           string `json:"album"`
	DurationMs      int64  `json:"durationMs"`
	Score           int    `json:"score"`
	Rank            int    `json:"rank"`
}

type AITaskRequest struct {
	Task           string `json:"task"`
	TargetLanguage string `json:"targetLanguage,omitempty"`
	Tone           string `json:"tone,omitempty"`
}

type AITaskResult struct {
	Task           string        `json:"task"`
	TargetLanguage string        `json:"targetLanguage,omitempty"`
	LineCount      int           `json:"lineCount"`
	DurationMs     int64         `json:"durationMs"`
	UpdatedLyrics  LyricDocument `json:"updatedLyrics"`
}

type Session struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AppID     string `json:"appId"`
	Active    bool   `json:"active"`
	Available bool   `json:"available"`
	UpdatedAt string `json:"updatedAt"`
}

type WSClient struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RemoteAddr  string `json:"remoteAddr"`
	ConnectedAt string `json:"connectedAt"`
	LastSeenAt  string `json:"lastSeenAt"`
	LatencyMs   int64  `json:"latencyMs"`
}

type AMLLConnection struct {
	Enabled            bool   `json:"enabled"`
	URL                string `json:"url"`
	Status             string `json:"status"`
	Message            string `json:"message"`
	ConnectedAt        string `json:"connectedAt,omitempty"`
	LastMessageAt      string `json:"lastMessageAt,omitempty"`
	MessagesSent       uint64 `json:"messagesSent"`
	BinaryMessagesSent uint64 `json:"binaryMessagesSent"`
	BytesSent          uint64 `json:"bytesSent"`
}

type ServiceStatus struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	UpdatedAt string `json:"updatedAt"`
}

type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Source  string `json:"source"`
	Message string `json:"message"`
}

type WebSocketStats struct {
	Endpoint           string  `json:"endpoint"`
	ActiveClients      int     `json:"activeClients"`
	MessagesSent       uint64  `json:"messagesSent"`
	BinaryMessagesSent uint64  `json:"binaryMessagesSent"`
	BytesSent          uint64  `json:"bytesSent"`
	MessagesPerMinute  float64 `json:"messagesPerMinute"`
	LastMessageType    string  `json:"lastMessageType"`
	LastMessageAt      string  `json:"lastMessageAt"`
	UpdatedAt          string  `json:"updatedAt"`
}

type WebSocketMessageTrace struct {
	ID          string `json:"id"`
	Endpoint    string `json:"endpoint"`
	MessageType string `json:"messageType"`
	BytesSent   int    `json:"bytesSent"`
	Binary      bool   `json:"binary"`
	Time        string `json:"time"`
}

type WebSocketSeriesPoint struct {
	Time           string `json:"time"`
	Endpoint       string `json:"endpoint"`
	Messages       uint64 `json:"messages"`
	BinaryMessages uint64 `json:"binaryMessages"`
	BytesSent      uint64 `json:"bytesSent"`
}

type APIRequestTrace struct {
	ID         string `json:"id"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Status     int    `json:"status"`
	DurationMs int64  `json:"durationMs"`
	BytesSent  int    `json:"bytesSent"`
	RemoteAddr string `json:"remoteAddr"`
	Time       string `json:"time"`
}

type AudioEnergyPoint struct {
	Sequence   uint64  `json:"sequence"`
	Timestamp  string  `json:"timestamp"`
	TrackID    string  `json:"trackId"`
	PositionMs int64   `json:"positionMs"`
	DurationMs int     `json:"durationMs"`
	RMS        float64 `json:"rms"`
	Peak       float64 `json:"peak"`
}

type PerformanceStats struct {
	UptimeSeconds       int64   `json:"uptimeSeconds"`
	CPUPercent          float64 `json:"cpuPercent"`
	MemoryAllocBytes    uint64  `json:"memoryAllocBytes"`
	MemorySysBytes      uint64  `json:"memorySysBytes"`
	Goroutines          int     `json:"goroutines"`
	HTTPRequests        uint64  `json:"httpRequests"`
	HTTPErrors          uint64  `json:"httpErrors"`
	HTTPBytesSent       uint64  `json:"httpBytesSent"`
	WebSocketBytesSent  uint64  `json:"webSocketBytesSent"`
	NetworkBytesSent    uint64  `json:"networkBytesSent"`
	WebSocketMessages   uint64  `json:"webSocketMessages"`
	WebSocketBinaryMsgs uint64  `json:"webSocketBinaryMessages"`
	UpdatedAt           string  `json:"updatedAt"`
}

type AppMetrics struct {
	WebSockets        []WebSocketStats        `json:"webSockets"`
	WebSocketMessages []WebSocketMessageTrace `json:"webSocketMessages"`
	WebSocketSeries   []WebSocketSeriesPoint  `json:"webSocketSeries"`
	API               []APIRequestTrace       `json:"api"`
	AudioEnergy       []AudioEnergyPoint      `json:"audioEnergy"`
	Performance       PerformanceStats        `json:"performance"`
}

type AudioFrame struct {
	Sequence     uint64    `json:"sequence"`
	Timestamp    string    `json:"timestamp"`
	TrackID      string    `json:"trackId"`
	PositionMs   int64     `json:"positionMs"`
	SampleRate   int       `json:"sampleRate"`
	Channels     int       `json:"channels"`
	Format       string    `json:"format"`
	DurationMs   int       `json:"durationMs"`
	RMS          float64   `json:"rms"`
	Peak         float64   `json:"peak"`
	Spectrum     []float64 `json:"spectrum,omitempty"`
	PCMBase64    string    `json:"pcmBase64,omitempty"`
	PCM          []byte    `json:"-"`
	Provider     string    `json:"provider"`
	ProviderMode string    `json:"providerMode"`
}

type LyricCalibrationSuggestion struct {
	LineIndex           int     `json:"lineIndex"`
	LineText            string  `json:"lineText"`
	CurrentStartMs      int64   `json:"currentStartMs"`
	SuggestedPositionMs int64   `json:"suggestedPositionMs"`
	OffsetMs            int64   `json:"offsetMs"`
	Confidence          float64 `json:"confidence"`
	Reason              string  `json:"reason"`
}

type LyricCalibrationResult struct {
	Suggestion    LyricCalibrationSuggestion `json:"suggestion"`
	UpdatedLyrics LyricDocument              `json:"updatedLyrics"`
}

type AppSnapshot struct {
	Version         string                 `json:"version"`
	Config          AppConfig              `json:"config"`
	Track           Track                  `json:"track"`
	Playback        Playback               `json:"playback"`
	Lyrics          LyricDocument          `json:"lyrics"`
	LyricCandidates []LyricSearchCandidate `json:"lyricCandidates"`
	AMLL            AMLLConnection         `json:"amll"`
	Sessions        []Session              `json:"sessions"`
	Clients         []WSClient             `json:"clients"`
	Services        []ServiceStatus        `json:"services"`
	Logs            []LogEntry             `json:"logs"`
	Audio           AudioFrame             `json:"audio"`
	Metrics         AppMetrics             `json:"metrics"`
}

type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
	Time    string      `json:"time"`
}

func Now() string {
	return FormatTime(time.Now())
}

func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func DefaultConfig() AppConfig {
	return AppConfig{
		Server: ServerConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    41917,
		},
		Audio: AudioConfig{
			Enabled:         true,
			Mode:            "features",
			Format:          "f32le",
			SampleRate:      48000,
			Channels:        2,
			FrameDurationMs: 50,
			IncludePCM:      false,
			IncludeFeatures: true,
		},
		Lyrics: LyricsConfig{
			AutoSearch:   true,
			CacheEnabled: true,
			Sources:      []string{"amll-ttml-db", "qqmusic", "netease", "kugou", "local"},
		},
		AMLL: AMLLConfig{
			Enabled:     false,
			AutoConnect: false,
			URL:         "ws://127.0.0.1:11444",
			SendAudio:   true,
		},
		AI: AIConfig{
			Enabled: false,
			BaseURL: "https://api.openai.com/v1",
			Model:   "gpt-4.1-mini",
			Timeout: 60,
		},
		UI: UIConfig{
			MinimizeToTray: false,
			HideOnClose:    true,
			StartHidden:    false,
		},
		Session: SessionConfig{
			AutoSelect: true,
		},
	}
}
