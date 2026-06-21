package smtc

import (
	"strings"

	smtcsuite "github.com/xiaowumin-mark/smtc-suite-go/pkg/smtc"

	"lyricsync/pkg/model"
)

func chooseSession(sessions []smtcsuite.SessionInfo, currentID string, cfg model.SessionConfig) *smtcsuite.SessionInfo {
	allowed := make([]smtcsuite.SessionInfo, 0, len(sessions))
	for _, session := range sessions {
		if sessionAllowed(session, cfg) {
			allowed = append(allowed, session)
		}
	}
	if len(allowed) == 0 {
		return nil
	}

	if selected := matchSessionList(allowed, cfg.Preferred); selected != nil {
		return selected
	}
	if !cfg.AutoSelect {
		if selected := findSessionByID(allowed, currentID); selected != nil {
			return selected
		}
		return &allowed[0]
	}
	for index := range allowed {
		if allowed[index].PlaybackStatus == smtcsuite.PlaybackStatusPlaying {
			return &allowed[index]
		}
	}
	if selected := findSessionByID(allowed, currentID); selected != nil {
		return selected
	}
	return &allowed[0]
}

func sessionAllowed(session smtcsuite.SessionInfo, cfg model.SessionConfig) bool {
	if len(cfg.Whitelist) > 0 && !sessionMatchesAny(session, cfg.Whitelist) {
		return false
	}
	return !sessionMatchesAny(session, cfg.Blacklist)
}

func matchSessionList(sessions []smtcsuite.SessionInfo, patterns []string) *smtcsuite.SessionInfo {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		for index := range sessions {
			if sessionMatches(sessions[index], pattern) {
				return &sessions[index]
			}
		}
	}
	return nil
}

func findSessionByID(sessions []smtcsuite.SessionInfo, id string) *smtcsuite.SessionInfo {
	if id == "" {
		return nil
	}
	for index := range sessions {
		if sessions[index].SessionID == id || sessions[index].SourceAppUserModelID == id {
			return &sessions[index]
		}
	}
	return nil
}

func sessionMatchesAny(session smtcsuite.SessionInfo, patterns []string) bool {
	for _, pattern := range patterns {
		if sessionMatches(session, pattern) {
			return true
		}
	}
	return false
}

func sessionMatches(session smtcsuite.SessionInfo, pattern string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if pattern == "" {
		return false
	}
	fields := []string{
		session.SessionID,
		session.SourceAppUserModelID,
		session.MediaInfo.Title,
		session.MediaInfo.Artist,
		session.MediaInfo.AlbumArtist,
		session.MediaInfo.AlbumTitle,
		sessionDisplayName(session),
	}
	for _, field := range fields {
		value := strings.ToLower(strings.TrimSpace(field))
		if value == pattern || strings.Contains(value, pattern) {
			return true
		}
	}
	return false
}

func sessionDisplayName(session smtcsuite.SessionInfo) string {
	parts := []string{}
	if session.MediaInfo.Title != "" {
		parts = append(parts, session.MediaInfo.Title)
	}
	if session.MediaInfo.Artist != "" {
		parts = append(parts, session.MediaInfo.Artist)
	}
	if len(parts) > 0 {
		return strings.Join(parts, " - ")
	}
	return session.SourceAppUserModelID
}
