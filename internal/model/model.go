package model

import "time"

const Version = "0.2.0-full"

type Config struct {
	AMLL   AMLLConfig   `json:"amll"`
	Media  MediaConfig  `json:"media"`
	Lyrics LyricsConfig `json:"lyrics"`
	AI     AIConfig     `json:"ai"`
	App    AppConfig    `json:"app"`
	TTMLDB TTMLDBConfig `json:"ttmlDb"`
}

type AMLLConfig struct {
	URL         string `json:"url"`
	AutoConnect bool   `json:"autoConnect"`
	SendAudio   bool   `json:"sendAudio"`
}

type MediaConfig struct {
	AutoSelect        bool     `json:"autoSelect"`
	SelectedSessionID string   `json:"selectedSessionId,omitempty"`
	Preferred         []string `json:"preferred"`
	Blacklist         []string `json:"blacklist"`
	Whitelist         []string `json:"whitelist"`
}

const (
	LyricSourceTTMLDB  = "ttml-db"
	LyricSourceQQ      = "qq"
	LyricSourceKugou   = "kugou"
	LyricSourceNetease = "netease"

	CloseBehaviorExit = "exit"
	CloseBehaviorTray = "tray"

	LyricsCleanNone     = "none"
	LyricsCleanSoftware = "software"
	LyricsCleanAI       = "ai"
)

type LyricsConfig struct {
	SearchPriority  []string `json:"searchPriority"`
	CleanStrategy   string   `json:"cleanStrategy"`
	AITranslate     bool     `json:"aiTranslate"`
	AITransliterate bool     `json:"aiTransliterate"`
}

type AIConfig struct {
	BaseURL      string `json:"baseUrl"`
	APIKey       string `json:"apiKey,omitempty"`
	Model        string `json:"model"`
	DeepThinking bool   `json:"deepThinking"`
}

type AppConfig struct {
	CloseBehavior string `json:"closeBehavior"`
}

type TTMLDBConfig struct {
	AutoUpdateIndex     bool   `json:"autoUpdateIndex"`
	UpdateIntervalHours int    `json:"updateIntervalHours"`
	IndexURL            string `json:"indexUrl"`
	LastUpdatedAt       string `json:"lastUpdatedAt,omitempty"`
}

type Track struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Artist        string `json:"artist"`
	Album         string `json:"album"`
	SourceApp     string `json:"sourceApp"`
	Duration      int64  `json:"durationMs"`
	CoverMimeType string `json:"coverMimeType,omitempty"`
	CoverHash     string `json:"coverHash,omitempty"`
	CoverData     []byte `json:"-"`
}

type Playback struct {
	State      string  `json:"state"`
	Position   int64   `json:"positionMs"`
	Volume     float64 `json:"volume"`
	CanControl bool    `json:"canControl"`
	UpdatedAt  string  `json:"updatedAt"`
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
	PCM          []byte    `json:"-"`
	ProviderMode string    `json:"providerMode"`
}

type Session struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AppID     string `json:"appId"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Album     string `json:"album"`
	State     string `json:"state"`
	Position  int64  `json:"positionMs"`
	Duration  int64  `json:"durationMs"`
	Active    bool   `json:"active"`
	UpdatedAt string `json:"updatedAt"`
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

type Snapshot struct {
	Version   string         `json:"version"`
	Config    Config         `json:"config"`
	Track     Track          `json:"track"`
	Playback  Playback       `json:"playback"`
	Audio     AudioFrame     `json:"audio"`
	Sessions  []Session      `json:"sessions"`
	AMLL      AMLLConnection `json:"amll"`
	Logs      []string       `json:"logs"`
	UpdatedAt string         `json:"updatedAt"`
}

type Event struct {
	Type    string
	Payload any
	Time    string
}

func Now() string {
	return FormatTime(time.Now())
}

func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
