package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kylebrantley/seer/internal/app"

	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(
		zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		},
	).With().Timestamp().Caller().Logger()

	application := app.New(&logger)

	workingDir, err := os.Getwd()
	if err != nil {
		logger.Error().Err(err).Msg("error getting working directory")
		os.Exit(1)
	}

	err = application.Init(workingDir)
	if err != nil {
		logger.Error().Err(err).Msg("initializing the application")
		os.Exit(1)
	}

	// Create a channel to watch for a shutdown signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		logger.Info().Msg("shut down message received")
		application.Stop()
		os.Exit(1)
	}()

	application.Run()
}
