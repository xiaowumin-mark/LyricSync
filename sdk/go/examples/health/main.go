package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"lyricsync/sdk/go/lyricsync"
)

func main() {
	baseURL := os.Getenv("LYRICSYNC_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:41917"
	}

	health, err := lyricsync.New(baseURL).Health(context.Background())
	if err != nil {
		panic(err)
	}
	data, _ := json.MarshalIndent(health, "", "  ")
	fmt.Println(string(data))
}
