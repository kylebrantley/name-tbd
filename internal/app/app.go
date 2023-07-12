package app

import (
	"context"

	"github.com/kylebrantley/seer/internal/event"
	"github.com/kylebrantley/seer/internal/runner"

	"github.com/kylebrantley/seer/internal/watcher"

	"github.com/rs/zerolog"
)

type fileWatcher interface {
	Start() error
	Stop() error
	Channel() chan *event.Batch
}

type testRunner interface {
	Run(packages ...string) (runner.Result, error)
	FindPackage(file string) (string, error)
}

type App struct {
	ctx       context.Context
	ctxCancel context.CancelFunc
	logger    *zerolog.Logger
	watcher   fileWatcher
	runner    testRunner
	rootDir   string
}

func New(logger *zerolog.Logger) *App {
	ctx, ctxCancel := context.WithCancel(context.Background())

	return &App{
		ctx:       ctx,
		ctxCancel: ctxCancel,
		logger:    logger,
	}
}

func (a *App) Init(rootDir string) error {
	a.rootDir = rootDir

	return nil
}

func (a *App) Run() {
	a.logger.Info().Msg("starting the application...")

	a.watcher = watcher.New(a.ctx, a.logger, a.rootDir)
	a.runner = runner.New(a.ctx, a.logger, a.rootDir)

	a.startWatcher()

	for e := range a.watcher.Channel() {
		a.handleEvent(e)
	}
}

func (a *App) Stop() {
	a.ctxCancel()

	if err := a.watcher.Stop(); err != nil {
		a.logger.Error().Err(err).Msg("failed to stop the watcher")
	}
}

func (a *App) startWatcher() {
	a.logger.Info().Msg("starting watcher")

	if err := a.watcher.Start(); err != nil {
		a.logger.Fatal().Err(err).Msg("failed to start watcher")
	}
}

func (a *App) handleEvent(e *event.Batch) {
	packages := make([]string, 0)

	for _, path := range e.Paths() {
		pkg, err := a.runner.FindPackage(path)
		if err != nil {
			a.logger.Error().Err(err).Msg("failed to find package")
			continue
		}

		packages = append(packages, pkg)
	}

	a.runTests(packages...)
}

func (a *App) runTests(packages ...string) {
	// TODO: this, parse coverprofile
	results, err := a.runner.Run(packages...)
	if err != nil {
		a.logger.Error().
			Err(err).
			Msg("failed to execute tests")
	}

	defer func(results runner.Result) {
		err := results.Close()
		if err != nil {
			a.logger.Error().Err(err).Msg("failed to delete test results")
		}
	}(results)
}
