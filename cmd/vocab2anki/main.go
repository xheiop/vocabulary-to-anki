// Command vocab2anki is a local daemon that turns words selected in a browser
// (via a userscript) or on iOS (via a Shortcut writing to iCloud Drive) into
// richly-formatted Anki notes: an LLM-generated definition and examples plus a
// dictionary IPA and pronunciation audio, added through AnkiConnect.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/xheiop/vocab2anki/internal/anki"
	"github.com/xheiop/vocab2anki/internal/audio"
	"github.com/xheiop/vocab2anki/internal/config"
	"github.com/xheiop/vocab2anki/internal/enrich"
	"github.com/xheiop/vocab2anki/internal/pending"
	"github.com/xheiop/vocab2anki/internal/process"
	"github.com/xheiop/vocab2anki/internal/queue"
	"github.com/xheiop/vocab2anki/internal/server"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config file")
	flag.Parse()

	log.SetFlags(log.LstdFlags)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Collaborators.
	ankiClient := anki.New(cfg.Anki.URL, cfg.Anki.Deck, cfg.Anki.Model)
	enrichSvc, err := enrich.New(enrich.Options{
		Provider:  cfg.Claude.Provider,
		Model:     cfg.Claude.Model,
		MaxTokens: cfg.Claude.MaxTokens,
		APIKey:    cfg.AnthropicAPIKey,
		CLIPath:   cfg.Claude.CLIPath,
	})
	if err != nil {
		log.Fatalf("init enrich: %v", err)
	}
	audioSvc, err := audio.New(cfg.Audio.Dir)
	if err != nil {
		log.Fatalf("init audio: %v", err)
	}
	pendingStore, err := pending.New(cfg.Pending.File)
	if err != nil {
		log.Fatalf("init pending store: %v", err)
	}
	proc := process.New(ankiClient, enrichSvc, audioSvc, pendingStore)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	jobs := make(chan process.Request, 128)

	// Worker: process one word at a time (serial keeps API usage predictable).
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case req := <-jobs:
				proc.Handle(ctx, req)
			}
		}
	}()

	// Retry the pending queue on a ticker so words added while Anki was closed
	// land once it reopens (the processor creates the deck/model lazily).
	go func() {
		interval := time.Duration(cfg.Pending.RetryInterval) * time.Second
		if interval <= 0 {
			interval = 60 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				proc.RetryPending(ctx)
			}
		}
	}()

	// iCloud file queue (iOS intake).
	watcher := queue.New(cfg.Queue.File, jobs)
	go func() {
		if err := watcher.Run(ctx); err != nil {
			log.Printf("queue watcher stopped: %v", err)
		}
	}()

	// HTTP intake (browser userscript).
	srv := &http.Server{
		Addr:    cfg.Server.Listen,
		Handler: server.New(jobs).Handler(),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("vocab2anki listening on http://%s", cfg.Server.Listen)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server: %v", err)
	}
	log.Print("shutting down")
}
