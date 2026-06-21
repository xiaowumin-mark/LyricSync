package lyric

import (
	"fmt"
	"math"
	"sort"

	"lyricsync/pkg/model"
)

const (
	calibrationPrimaryBeforeMs  int64 = 2500
	calibrationPrimaryAfterMs   int64 = 750
	calibrationFallbackBeforeMs int64 = 5000
	calibrationFallbackAfterMs  int64 = 1500
)

type scoredEnergyPoint struct {
	point     model.AudioEnergyPoint
	energy    float64
	onset     float64
	proximity float64
	score     float64
}

func SuggestAlignment(doc model.LyricDocument, history []model.AudioEnergyPoint, lineIndex int, playbackPositionMs int64, trackID string) (model.LyricCalibrationSuggestion, error) {
	if len(doc.Lines) == 0 {
		return model.LyricCalibrationSuggestion{}, fmt.Errorf("lyric: no current lyrics to calibrate")
	}
	if playbackPositionMs < 0 {
		playbackPositionMs = 0
	}
	if lineIndex < 0 {
		lineIndex = nearestLineIndex(doc.Lines, playbackPositionMs)
	}
	if lineIndex < 0 || lineIndex >= len(doc.Lines) {
		return model.LyricCalibrationSuggestion{}, fmt.Errorf("lyric: line index %d out of range", lineIndex)
	}

	candidates := calibrationCandidates(history, trackID, playbackPositionMs, calibrationPrimaryBeforeMs, calibrationPrimaryAfterMs)
	if len(candidates) < 3 {
		candidates = calibrationCandidates(history, trackID, playbackPositionMs, calibrationFallbackBeforeMs, calibrationFallbackAfterMs)
	}
	if len(candidates) == 0 {
		return model.LyricCalibrationSuggestion{}, fmt.Errorf("lyric: insufficient audio history for calibration")
	}

	best, ok := bestEnergyPoint(candidates, playbackPositionMs)
	if !ok || best.energy <= 0 {
		return model.LyricCalibrationSuggestion{}, fmt.Errorf("lyric: audio history has no usable energy")
	}

	line := doc.Lines[lineIndex]
	offsetMs := best.point.PositionMs - line.StartMs
	distanceMs := absInt64(best.point.PositionMs - playbackPositionMs)
	confidence := 0.20 + best.energy*0.45 + best.onset*1.30 + best.proximity*0.20
	if best.onset < 0.02 {
		confidence = math.Min(confidence, 0.62)
	}

	return model.LyricCalibrationSuggestion{
		LineIndex:           lineIndex,
		LineText:            line.Text,
		CurrentStartMs:      line.StartMs,
		SuggestedPositionMs: best.point.PositionMs,
		OffsetMs:            offsetMs,
		Confidence:          roundFloat(clampFloat(confidence, 0.05, 0.95), 2),
		Reason:              fmt.Sprintf("energy=%.2f onset=%.2f distance=%dms", best.energy, best.onset, distanceMs),
	}, nil
}

func calibrationCandidates(history []model.AudioEnergyPoint, trackID string, centerMs, beforeMs, afterMs int64) []model.AudioEnergyPoint {
	startMs := centerMs - beforeMs
	if startMs < 0 {
		startMs = 0
	}
	endMs := centerMs + afterMs
	candidates := make([]model.AudioEnergyPoint, 0, len(history))
	for _, point := range history {
		if trackID != "" && point.TrackID != "" && point.TrackID != trackID {
			continue
		}
		if point.PositionMs < startMs || point.PositionMs > endMs {
			continue
		}
		candidates = append(candidates, point)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].PositionMs == candidates[j].PositionMs {
			return candidates[i].Sequence < candidates[j].Sequence
		}
		return candidates[i].PositionMs < candidates[j].PositionMs
	})
	return candidates
}

func bestEnergyPoint(points []model.AudioEnergyPoint, centerMs int64) (scoredEnergyPoint, bool) {
	if len(points) == 0 {
		return scoredEnergyPoint{}, false
	}

	var totalEnergy float64
	for _, point := range points {
		totalEnergy += pointEnergy(point)
	}
	meanEnergy := totalEnergy / float64(len(points))

	var best scoredEnergyPoint
	for index, point := range points {
		energy := pointEnergy(point)
		previous := previousEnergy(points, index, meanEnergy)
		onset := energy - previous
		if onset < 0 {
			onset = 0
		}
		distance := math.Abs(float64(point.PositionMs - centerMs))
		proximityWindow := float64(calibrationPrimaryBeforeMs + calibrationPrimaryAfterMs)
		proximity := 1 - math.Min(distance/proximityWindow, 1)
		score := energy*0.58 + onset*1.40 + proximity*0.15
		scored := scoredEnergyPoint{
			point:     point,
			energy:    energy,
			onset:     onset,
			proximity: proximity,
			score:     score,
		}
		if index == 0 || scored.score > best.score {
			best = scored
		}
	}
	return best, true
}

func previousEnergy(points []model.AudioEnergyPoint, index int, fallback float64) float64 {
	start := index - 4
	if start < 0 {
		start = 0
	}
	var total float64
	var count int
	for i := start; i < index; i++ {
		total += pointEnergy(points[i])
		count++
	}
	if count == 0 {
		return fallback
	}
	return total / float64(count)
}

func pointEnergy(point model.AudioEnergyPoint) float64 {
	return clampFloat(point.RMS*0.65+point.Peak*0.35, 0, 1)
}

func nearestLineIndex(lines []model.LyricLine, positionMs int64) int {
	if len(lines) == 0 {
		return -1
	}
	for index, line := range lines {
		if positionMs >= line.StartMs && positionMs < line.EndMs {
			return index
		}
	}
	nearest := 0
	nearestDistance := absInt64(lines[0].StartMs - positionMs)
	for index := 1; index < len(lines); index++ {
		distance := absInt64(lines[index].StartMs - positionMs)
		if distance < nearestDistance {
			nearest = index
			nearestDistance = distance
		}
	}
	return nearest
}

func clampFloat(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func roundFloat(value float64, places int) float64 {
	scale := math.Pow(10, float64(places))
	return math.Round(value*scale) / scale
}
