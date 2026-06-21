package core

import (
	"runtime"
	"sort"
	"sync"
	"time"

	"lyricsync/internal/perf"
	"lyricsync/pkg/model"
)

type State struct {
	mu              sync.RWMutex
	config          model.AppConfig
	track           model.Track
	playback        model.Playback
	lyrics          model.LyricDocument
	lyricCandidates []model.LyricSearchCandidate
	amll            model.AMLLConnection
	sessions        map[string]model.Session
	clients         map[string]model.WSClient
	services        map[string]model.ServiceStatus
	logs            []model.LogEntry
	audio           model.AudioFrame
	audioHistory    []model.AudioEnergyPoint
	wsStats         map[string]wsStatsState
	wsMessages      []model.WebSocketMessageTrace
	wsSeries        []model.WebSocketSeriesPoint
	apiRequests     []model.APIRequestTrace
	httpRequests    uint64
	httpErrors      uint64
	httpBytesSent   uint64
	startedAt       time.Time
	cpuSampler      *perf.CPUSampler
	subscribers     map[chan model.Event]struct{}
}

const maxAudioHistoryPoints = 600

type wsStatsState struct {
	stats       model.WebSocketStats
	windowStart time.Time
	windowCount uint64
}

func NewState(config model.AppConfig) *State {
	nowTime := time.Now()
	now := model.FormatTime(nowTime)
	return &State{
		config: config,
		track: model.Track{
			ID:        "idle",
			Title:     "等待播放",
			Artist:    "LyricSync",
			SourceApp: "System",
			Duration:  180000,
		},
		playback: model.Playback{
			State:      "stopped",
			UpdatedAt:  now,
			Volume:     1,
			CanControl: false,
		},
		amll: model.AMLLConnection{
			Enabled: config.AMLL.Enabled,
			URL:     config.AMLL.URL,
			Status:  "disconnected",
		},
		sessions:    map[string]model.Session{},
		clients:     map[string]model.WSClient{},
		services:    map[string]model.ServiceStatus{},
		wsStats:     map[string]wsStatsState{},
		startedAt:   nowTime,
		cpuSampler:  perf.NewCPUSampler(),
		subscribers: map[chan model.Event]struct{}{},
	}
}

func (s *State) Config() model.AppConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

func (s *State) Snapshot() model.AppSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]model.Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].Active != sessions[j].Active {
			return sessions[i].Active
		}
		return sessions[i].Name < sessions[j].Name
	})

	clients := make([]model.WSClient, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i].ConnectedAt < clients[j].ConnectedAt })

	services := make([]model.ServiceStatus, 0, len(s.services))
	for _, service := range s.services {
		services = append(services, service)
	}
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})

	logs := append([]model.LogEntry(nil), s.logs...)
	lyricCandidates := append([]model.LyricSearchCandidate(nil), s.lyricCandidates...)
	webSockets := s.webSocketStatsLocked(time.Now())
	wsMessages := append([]model.WebSocketMessageTrace(nil), s.wsMessages...)
	wsSeries := append([]model.WebSocketSeriesPoint(nil), s.wsSeries...)
	apiRequests := append([]model.APIRequestTrace(nil), s.apiRequests...)
	audioHistory := append([]model.AudioEnergyPoint(nil), s.audioHistory...)
	performance := s.performanceStatsLocked(webSockets, time.Now())

	return model.AppSnapshot{
		Version:         model.AppVersion,
		Config:          s.config,
		Track:           s.track,
		Playback:        s.playback,
		Lyrics:          s.lyrics,
		LyricCandidates: lyricCandidates,
		AMLL:            s.amll,
		Sessions:        sessions,
		Clients:         clients,
		Services:        services,
		Logs:            logs,
		Audio:           s.audio,
		Metrics: model.AppMetrics{
			WebSockets:        webSockets,
			WebSocketMessages: wsMessages,
			WebSocketSeries:   wsSeries,
			API:               apiRequests,
			AudioEnergy:       audioHistory,
			Performance:       performance,
		},
	}
}

func (s *State) AudioHistory() []model.AudioEnergyPoint {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]model.AudioEnergyPoint(nil), s.audioHistory...)
}

func (s *State) Subscribe(buffer int) (<-chan model.Event, func()) {
	if buffer < 1 {
		buffer = 1
	}
	ch := make(chan model.Event, buffer)

	s.mu.Lock()
	s.subscribers[ch] = struct{}{}
	s.mu.Unlock()

	cancel := func() {
		s.mu.Lock()
		if _, ok := s.subscribers[ch]; ok {
			delete(s.subscribers, ch)
			close(ch)
		}
		s.mu.Unlock()
	}

	return ch, cancel
}

func (s *State) SetConfig(config model.AppConfig) {
	s.mu.Lock()
	s.config = config
	s.amll.Enabled = config.AMLL.Enabled
	if config.AMLL.URL != "" {
		s.amll.URL = config.AMLL.URL
	}
	s.publishLocked("config_changed", config)
	s.mu.Unlock()
}

func (s *State) SetTrack(track model.Track) {
	s.mu.Lock()
	if s.track == track {
		s.mu.Unlock()
		return
	}
	s.track = track
	s.publishLocked("song_changed", track)
	s.mu.Unlock()
}

func (s *State) SetPlayback(playback model.Playback) {
	s.mu.Lock()
	if playback.Volume < 0 {
		playback.Volume = s.playback.Volume
	}
	s.playback = playback
	s.publishLocked("playback_changed", playback)
	s.mu.Unlock()
}

func (s *State) SetLyrics(lyrics model.LyricDocument) {
	s.mu.Lock()
	s.lyrics = lyrics
	s.publishLocked("lyric_changed", lyrics)
	s.mu.Unlock()
}

func (s *State) SetLyricCandidates(candidates []model.LyricSearchCandidate) {
	s.mu.Lock()
	s.lyricCandidates = append([]model.LyricSearchCandidate(nil), candidates...)
	s.publishLocked("lyric_candidates_changed", s.lyricCandidates)
	s.mu.Unlock()
}

func (s *State) SetAMLLConnection(connection model.AMLLConnection) {
	s.mu.Lock()
	s.amll = connection
	s.publishLocked("amll_connection_changed", connection)
	s.mu.Unlock()
}

func (s *State) AMLLConnection() model.AMLLConnection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.amll
}

func (s *State) SetSessions(sessions []model.Session) {
	s.mu.Lock()
	next := make(map[string]model.Session, len(sessions))
	for _, session := range sessions {
		next[session.ID] = session
	}
	s.sessions = next
	s.publishLocked("session_changed", sessions)
	s.mu.Unlock()
}

func (s *State) SetService(name, status, message string) {
	s.mu.Lock()
	service := model.ServiceStatus{
		Name:      name,
		Status:    status,
		Message:   message,
		UpdatedAt: model.Now(),
	}
	s.services[name] = service
	s.publishLocked("service_changed", service)
	s.mu.Unlock()
}

func (s *State) SetAudioFrame(frame model.AudioFrame) {
	s.mu.Lock()
	s.audio = frame
	s.addAudioHistoryLocked(frame)
	s.publishLocked("audio_frame", frame)
	s.mu.Unlock()
}

func (s *State) AddWebSocketClient(endpoint string) {
	s.mu.Lock()
	stat := s.ensureWebSocketStatsLocked(endpoint, time.Now())
	stat.stats.ActiveClients++
	stat.stats.UpdatedAt = model.Now()
	s.wsStats[endpoint] = stat
	s.publishLocked("metrics_changed", model.AppMetrics{WebSockets: s.webSocketStatsLocked(time.Now())})
	s.mu.Unlock()
}

func (s *State) RemoveWebSocketClient(endpoint string) {
	s.mu.Lock()
	stat := s.ensureWebSocketStatsLocked(endpoint, time.Now())
	if stat.stats.ActiveClients > 0 {
		stat.stats.ActiveClients--
	}
	stat.stats.UpdatedAt = model.Now()
	s.wsStats[endpoint] = stat
	s.publishLocked("metrics_changed", model.AppMetrics{WebSockets: s.webSocketStatsLocked(time.Now())})
	s.mu.Unlock()
}

func (s *State) AddWebSocketSend(endpoint, messageType string, bytesSent int, binary bool) {
	s.mu.Lock()
	now := time.Now()
	stat := s.ensureWebSocketStatsLocked(endpoint, now)
	if now.Sub(stat.windowStart) >= time.Minute {
		stat.windowStart = now
		stat.windowCount = 0
	}
	stat.stats.MessagesSent++
	stat.windowCount++
	if binary {
		stat.stats.BinaryMessagesSent++
	}
	if bytesSent > 0 {
		stat.stats.BytesSent += uint64(bytesSent)
	}
	stat.stats.MessagesPerMinute = float64(stat.windowCount) / maxFloat64(now.Sub(stat.windowStart).Minutes(), 1.0/60.0)
	stat.stats.LastMessageType = messageType
	stat.stats.LastMessageAt = model.FormatTime(now)
	stat.stats.UpdatedAt = model.FormatTime(now)
	s.wsStats[endpoint] = stat
	s.addWebSocketMessageLocked(endpoint, messageType, bytesSent, binary, now)
	s.addWebSocketSeriesLocked(endpoint, bytesSent, binary, now)
	s.mu.Unlock()
}

func (s *State) AddAPIRequest(trace model.APIRequestTrace) {
	s.mu.Lock()
	if trace.ID == "" {
		trace.ID = model.Now()
	}
	if trace.Time == "" {
		trace.Time = model.Now()
	}
	s.httpRequests++
	if trace.Status >= 500 {
		s.httpErrors++
	}
	if trace.BytesSent > 0 {
		s.httpBytesSent += uint64(trace.BytesSent)
	}
	s.apiRequests = append(s.apiRequests, trace)
	if len(s.apiRequests) > 200 {
		s.apiRequests = append([]model.APIRequestTrace(nil), s.apiRequests[len(s.apiRequests)-200:]...)
	}
	s.mu.Unlock()
}

func (s *State) AddClient(client model.WSClient) {
	s.mu.Lock()
	s.clients[client.ID] = client
	s.publishLocked("client_changed", s.clientListLocked())
	s.mu.Unlock()
}

func (s *State) UpdateClient(client model.WSClient) {
	s.mu.Lock()
	if _, ok := s.clients[client.ID]; ok {
		s.clients[client.ID] = client
		s.publishLocked("client_changed", s.clientListLocked())
	}
	s.mu.Unlock()
}

func (s *State) RemoveClient(id string) {
	s.mu.Lock()
	delete(s.clients, id)
	s.publishLocked("client_changed", s.clientListLocked())
	s.mu.Unlock()
}

func (s *State) AddLog(level, source, message string) {
	s.mu.Lock()
	entry := model.LogEntry{
		Time:    model.Now(),
		Level:   level,
		Source:  source,
		Message: message,
	}
	s.logs = append(s.logs, entry)
	if len(s.logs) > 300 {
		s.logs = append([]model.LogEntry(nil), s.logs[len(s.logs)-300:]...)
	}
	s.publishLocked("log_added", entry)
	s.mu.Unlock()
}

func (s *State) publishLocked(eventType string, payload interface{}) {
	event := model.Event{
		Type:    eventType,
		Payload: payload,
		Time:    model.Now(),
	}
	for ch := range s.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *State) clientListLocked() []model.WSClient {
	clients := make([]model.WSClient, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i].ConnectedAt < clients[j].ConnectedAt })
	return clients
}

func (s *State) ensureWebSocketStatsLocked(endpoint string, now time.Time) wsStatsState {
	if endpoint == "" {
		endpoint = "unknown"
	}
	stat, ok := s.wsStats[endpoint]
	if !ok {
		stat = wsStatsState{
			stats: model.WebSocketStats{
				Endpoint:  endpoint,
				UpdatedAt: model.FormatTime(now),
			},
			windowStart: now,
		}
	}
	if stat.windowStart.IsZero() {
		stat.windowStart = now
	}
	return stat
}

func (s *State) webSocketStatsLocked(now time.Time) []model.WebSocketStats {
	stats := make([]model.WebSocketStats, 0, len(s.wsStats))
	for _, stat := range s.wsStats {
		if !stat.windowStart.IsZero() && now.Sub(stat.windowStart) < time.Minute {
			stat.stats.MessagesPerMinute = float64(stat.windowCount) / maxFloat64(now.Sub(stat.windowStart).Minutes(), 1.0/60.0)
		} else {
			stat.stats.MessagesPerMinute = 0
		}
		stats = append(stats, stat.stats)
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].Endpoint < stats[j].Endpoint })
	return stats
}

func (s *State) addWebSocketMessageLocked(endpoint, messageType string, bytesSent int, binary bool, now time.Time) {
	trace := model.WebSocketMessageTrace{
		ID:          endpoint + "-" + model.FormatTime(now),
		Endpoint:    endpoint,
		MessageType: messageType,
		BytesSent:   bytesSent,
		Binary:      binary,
		Time:        model.FormatTime(now),
	}
	s.wsMessages = append(s.wsMessages, trace)
	if len(s.wsMessages) > 300 {
		s.wsMessages = append([]model.WebSocketMessageTrace(nil), s.wsMessages[len(s.wsMessages)-300:]...)
	}
}

func (s *State) addWebSocketSeriesLocked(endpoint string, bytesSent int, binary bool, now time.Time) {
	bucket := now.Truncate(5 * time.Second)
	bucketTime := model.FormatTime(bucket)
	for index := len(s.wsSeries) - 1; index >= 0; index-- {
		point := s.wsSeries[index]
		if point.Time == bucketTime && point.Endpoint == endpoint {
			s.wsSeries[index].Messages++
			if binary {
				s.wsSeries[index].BinaryMessages++
			}
			if bytesSent > 0 {
				s.wsSeries[index].BytesSent += uint64(bytesSent)
			}
			return
		}
		if len(s.wsSeries)-index > 240 {
			break
		}
	}

	point := model.WebSocketSeriesPoint{
		Time:     bucketTime,
		Endpoint: endpoint,
		Messages: 1,
	}
	if binary {
		point.BinaryMessages = 1
	}
	if bytesSent > 0 {
		point.BytesSent = uint64(bytesSent)
	}
	s.wsSeries = append(s.wsSeries, point)
	if len(s.wsSeries) > 240 {
		s.wsSeries = append([]model.WebSocketSeriesPoint(nil), s.wsSeries[len(s.wsSeries)-240:]...)
	}
}

func (s *State) addAudioHistoryLocked(frame model.AudioFrame) {
	timestamp := frame.Timestamp
	if timestamp == "" {
		timestamp = model.Now()
	}
	point := model.AudioEnergyPoint{
		Sequence:   frame.Sequence,
		Timestamp:  timestamp,
		TrackID:    frame.TrackID,
		PositionMs: frame.PositionMs,
		DurationMs: frame.DurationMs,
		RMS:        frame.RMS,
		Peak:       frame.Peak,
	}
	s.audioHistory = append(s.audioHistory, point)
	if len(s.audioHistory) > maxAudioHistoryPoints {
		s.audioHistory = append([]model.AudioEnergyPoint(nil), s.audioHistory[len(s.audioHistory)-maxAudioHistoryPoints:]...)
	}
}

func (s *State) performanceStatsLocked(webSockets []model.WebSocketStats, now time.Time) model.PerformanceStats {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	var wsBytes uint64
	var wsMessages uint64
	var wsBinary uint64
	for _, stats := range webSockets {
		wsBytes += stats.BytesSent
		wsMessages += stats.MessagesSent
		wsBinary += stats.BinaryMessagesSent
	}

	uptime := now.Sub(s.startedAt).Seconds()
	if uptime < 0 {
		uptime = 0
	}
	return model.PerformanceStats{
		UptimeSeconds:       int64(uptime),
		CPUPercent:          s.cpuSampler.Percent(now),
		MemoryAllocBytes:    mem.Alloc,
		MemorySysBytes:      mem.Sys,
		Goroutines:          runtime.NumGoroutine(),
		HTTPRequests:        s.httpRequests,
		HTTPErrors:          s.httpErrors,
		HTTPBytesSent:       s.httpBytesSent,
		WebSocketBytesSent:  wsBytes,
		NetworkBytesSent:    s.httpBytesSent + wsBytes,
		WebSocketMessages:   wsMessages,
		WebSocketBinaryMsgs: wsBinary,
		UpdatedAt:           model.FormatTime(now),
	}
}

func maxFloat64(value, minimum float64) float64 {
	if value < minimum {
		return minimum
	}
	return value
}
