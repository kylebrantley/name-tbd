package watcher

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/kylebrantley/seer/internal/event"

	"github.com/rs/zerolog"
)

const tickerInterval = 500 * time.Millisecond

var defaultPathsToSkip = []string{"vendor"}

type Watcher struct {
	Events      chan *event.Batch
	batch       *event.Batch
	ctx         context.Context
	logger      *zerolog.Logger
	notifier    *fsnotify.Watcher
	pathsToSkip []string
	rootDir     string
	ticker      *time.Ticker
}

func New(ctx context.Context, logger *zerolog.Logger, rootDir string) *Watcher {
	return &Watcher{
		Events:      make(chan *event.Batch),
		batch:       event.NewBatch(),
		ctx:         ctx,
		logger:      logger,
		pathsToSkip: defaultPathsToSkip,
		rootDir:     rootDir,
	}
}

func (w *Watcher) Start() error {
	var err error

	w.notifier, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to initialize fsnotify: %w", err)
	}

	if err := filepath.Walk(w.rootDir, w.walk); err != nil {
		return fmt.Errorf("failed to traverse sub directories: %w", err)
	}

	w.ticker = time.NewTicker(tickerInterval)

	go w.watch()

	return nil
}

func (w *Watcher) Stop() error {
	if err := w.notifier.Close(); err != nil {
		return fmt.Errorf("failed to close notifier: %w", err)
	}

	w.ticker.Stop()
	close(w.Events)

	return nil
}

func (w *Watcher) Channel() chan *event.Batch {
	return w.Events
}

func (w *Watcher) watch() {
	w.logger.Info().Msgf("watching for changes across %d directories", len(w.notifier.WatchList()))

	for {
		select {
		case <-w.ctx.Done():
			// TODO: error message
			return
		case err, ok := <-w.notifier.Errors:
			if !ok {
				// TODO: error message
				return
			}

			w.logger.Error().Err(err).Msg("fsnotify error")
		case e, ok := <-w.notifier.Events:
			if !ok {
				// TODO: error message
				return
			}

			if err := w.handleEvent(e); err != nil {
				w.logger.Error().
					Err(err).
					Str("event", e.Op.String()).
					Str("file", e.Name).
					Msg("error handling event")
			}
		case _, ok := <-w.ticker.C:
			if !ok {
				// TODO: error message
				return
			}

			if len(w.batch.Events) == 0 {
				continue
			}

			batch := w.batch
			w.batch = event.NewBatch()

			w.logger.Info().Int("numberOfEvents", len(batch.Events)).Msg("publishing events")
			w.Events <- batch
		}
	}
}

func (w *Watcher) walk(path string, info fs.FileInfo, err error) error {
	if err != nil {
		return err
	}

	w.logger.Info().Str("directory", info.Name()).Msg("walking file path")

	if !info.IsDir() {
		return nil
	}

	if w.shouldSkip(filepath.Base(path)) {
		w.logger.Info().Str("directory", path).Msg("ignoring skip-able directory")
		return filepath.SkipDir
	}

	if err := w.notifier.Add(path); err != nil {
		return fmt.Errorf("failed to add path %s to fsnotify: %w", path, err)
	}

	w.logger.Info().Str("directory", path).Msg("directory added to notifier")

	return nil
}

func (w *Watcher) shouldSkip(path string) bool {
	if strings.HasPrefix(path, ".") || strings.HasSuffix(path, "~") {
		return true
	}

	for _, p := range w.pathsToSkip {
		if path == p {
			return true
		}
	}

	return false
}

func (w *Watcher) shouldSkipEvent(e fsnotify.Event) bool {
	if e.Op.Has(fsnotify.Chmod) {
		return true
	}

	return w.shouldSkip(filepath.Base(e.Name))
}

func (w *Watcher) handleEvent(e fsnotify.Event) error {
	w.logger.Info().
		Str("path", e.Name).
		Str("event", e.Op.String()).
		Msg("new event received")

	if w.shouldSkipEvent(e) {
		w.logger.Info().
			Str("path", e.Name).
			Str("event", e.Op.String()).
			Msg("skipping event")

		return nil
	}

	switch {
	case e.Op.Has(fsnotify.Write):
		return w.handleWriteEvent(e)
	case e.Op.Has(fsnotify.Create):
		return w.handleCreateEvent(e)
	case e.Op.Has(fsnotify.Remove):
		return w.handleRemoveEvent(e)
	default:
		w.logger.Info().
			Str("path", e.Name).
			Str("event", e.Op.String()).
			Msg("event not handled")
	}

	return nil
}

func (w *Watcher) handleWriteEvent(e fsnotify.Event) error {
	if isGoFile(e.Name) {
		w.logger.Info().
			Str("event", e.Op.String()).
			Str("file", e.Name).
			Msg("handling event")
		w.batch.Add(e.Name, event.Write)
	}

	return nil
}

func (w *Watcher) handleRemoveEvent(e fsnotify.Event) error {
	if isGoFile(e.Name) {
		w.logger.Info().Str("file", e.Name).Msg("file deleted")
		w.batch.Add(e.Name, event.Delete)
	}

	return nil
}

func (w *Watcher) handleCreateEvent(e fsnotify.Event) error {
	info, err := os.Stat(e.Name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			w.logger.Info().
				Str("file", e.Name).
				Msg("ignore event, file or directory does not exist")

			return nil
		}

		return fmt.Errorf("failed to fetch information directory or file: %w", err)
	}

	if info.IsDir() {
		if err := w.walk(e.Name, info, nil); err != nil {
			return fmt.Errorf("failed to walk new directory: %w", err)
		}
	}

	if isGoFile(e.Name) {
		w.logger.Info().Str("file", e.Name).Msg("file created, adding to batch")
		w.batch.Add(e.Name, event.Create)
	}

	return nil
}

func isGoFile(path string) bool {
	return strings.HasSuffix(path, ".go")
}
