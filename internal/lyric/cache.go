package lyric

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"lyricsync/pkg/model"
)

type Cache struct {
	dir string
}

func NewCache(dir string) (*Cache, error) {
	if strings.TrimSpace(dir) == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(base, "LyricSync", "lyrics")
	}
	return &Cache{dir: dir}, nil
}

func CacheKey(track model.Track) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(track.Title)),
		strings.ToLower(strings.TrimSpace(track.Artist)),
		strings.ToLower(strings.TrimSpace(track.Album)),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func (c *Cache) Load(key string) (model.LyricDocument, bool, error) {
	path := c.path(key)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return model.LyricDocument{}, false, nil
	}
	if err != nil {
		return model.LyricDocument{}, false, err
	}
	var doc model.LyricDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return model.LyricDocument{}, false, err
	}
	return doc, true, nil
}

func (c *Cache) Save(key string, doc model.LyricDocument) error {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path(key), data, 0o600)
}

func (c *Cache) Dir() string {
	return c.dir
}

func (c *Cache) path(key string) string {
	return filepath.Join(c.dir, key+".json")
}
