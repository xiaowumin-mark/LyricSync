package lyric

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var timestampPattern = regexp.MustCompile(`^(((\d+):)?(\d+):)?((\d+)([.:](\d{1,3}))?)$`)

func ParseTimestamp(value string) (int64, error) {
	value = strings.TrimSpace(value)
	matches := timestampPattern.FindStringSubmatch(value)
	if matches == nil {
		return 0, fmt.Errorf("invalid lyric timestamp %q", value)
	}
	getInt := func(index int) int64 {
		if index >= len(matches) || matches[index] == "" {
			return 0
		}
		n, _ := strconv.ParseInt(matches[index], 10, 64)
		return n
	}
	hour := getInt(3)
	minute := getInt(4)
	second := getInt(6)
	frac := "0"
	if len(matches) > 8 && matches[8] != "" {
		frac = matches[8]
	}
	if len(frac) < 3 {
		frac += strings.Repeat("0", 3-len(frac))
	}
	millis, _ := strconv.ParseInt(frac, 10, 64)
	return (hour*3600+minute*60+second)*1000 + millis, nil
}

func FormatTimestamp(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	totalSeconds := ms / 1000
	millis := ms % 1000
	seconds := totalSeconds % 60
	totalMinutes := totalSeconds / 60
	minutes := totalMinutes % 60
	hours := totalMinutes / 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, millis)
	}
	return fmt.Sprintf("%02d:%02d.%03d", minutes, seconds, millis)
}

func roundMillis(value float64) int64 {
	if math.IsNaN(value) || value < 0 {
		return 0
	}
	return int64(math.Round(value))
}
