package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/xiaowumin-mark/LyricSync/internal/app"
	"github.com/xiaowumin-mark/LyricSync/internal/config"
	"github.com/xiaowumin-mark/LyricSync/internal/paths"
	"github.com/xiaowumin-mark/LyricSync/internal/song"
	"github.com/xiaowumin-mark/LyricSync/internal/state"
	"github.com/xiaowumin-mark/LyricSync/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("load config: %v", err)
		cfg = config.Default()
	}

	store := state.New(cfg)
	dbPath, err := paths.DatabasePath()
	if err != nil {
		log.Printf("song database path: %v", err)
		store.AddLog("song database path failed: " + err.Error())
	}
	var songs *song.Repository
	if dbPath != "" {
		songs, err = song.Open(dbPath)
		if err != nil {
			log.Printf("open song database: %v", err)
			store.AddLog("song database unavailable: " + err.Error())
		} else {
			defer songs.Close()
			store.AddLog("song database ready")
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	runtime := app.New(store, songs)
	if err := runtime.Start(ctx); err != nil {
		store.AddLog("runtime start failed: " + err.Error())
	}
	defer runtime.Stop()

	if err := ui.Run(ctx, store, runtime); err != nil {
		log.Fatal(err)
	}
}
