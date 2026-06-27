package paths

import (
	"os"
	"path/filepath"
)

const appDirName = "LyricSync"

func DataDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(base, appDirName), nil
}

func DatabasePath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lyricsync.db"), nil
}
