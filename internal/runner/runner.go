package runner

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/rs/zerolog"
)

const (
	defaultBin          = "go"
	defaultTempDir      = "tmp"
	coverDirPermissions = 0o755
)

type Result struct {
	Pass     bool
	Start    time.Time
	End      time.Time
	ExitCode int
	Duration time.Duration
	// TODO: should this be a map? Probably not
	Packages map[string]*Package
	dir      string
	id       uuid.UUID
}

func (r *Result) Close() error {
	if r.dir == "" {
		return nil
	}

	if err := os.RemoveAll(r.dir); err != nil {
		return fmt.Errorf("failed to delete temp dir %q: %w", r.dir, err)
	}

	r.dir = ""

	return nil
}

type Executor func(ctx context.Context, dir, name string, args ...string) ([]byte, int, error)

type Runner struct {
	bin         string
	ctx         context.Context
	logger      *zerolog.Logger
	rootDir     string
	tempDir     string
	executor    Executor
	tempDirOnce sync.Once
}

func New(ctx context.Context, logger *zerolog.Logger, rootDir string) *Runner {
	return &Runner{
		bin:      defaultBin,
		ctx:      ctx,
		executor: runner,
		logger:   logger,
		rootDir:  rootDir,
		tempDir:  defaultTempDir,
	}
}

func (r *Runner) Run(packages ...string) (Result, error) {
	result := &Result{
		Pass:  true,
		Start: time.Now(),
		id:    uuid.New(),
	}

	var err error
	if result.dir, err = r.makeTempDir(result.id); err != nil {
		return Result{}, err
	}

	args := []string{
		"test",
		"-json",
		"-cover",
		"-coverprofile",
		path.Join(result.dir, "coverprofile.out"),
	}

	args = append(args, packages...)

	r.logger.Info().
		Strs("args", args).
		Msg("executing 'go test'")

	// TODO: a non-zero exit code is returned when a test scenario fails should figure out what is needed
	// to detect a real failure vs a failed test case
	var output []byte
	output, result.ExitCode, err = r.executor(r.ctx, r.rootDir, r.bin, args...)
	result.Duration = time.Since(result.Start)

	r.logger.Info().
		Dur("duration", result.Duration).
		Int("exitCode", result.ExitCode).
		Msg("finished executing test run")

	if err != nil {
		return Result{}, fmt.Errorf("failed to execute tests: %w", err)
	}

	// if result.ExitCode != 0 {
	// 	return Result{}, fmt.Errorf("failed to execute tests: exitCode =  %d", result.ExitCode)
	// }

	// TODO: figure this out
	result.Packages, err = r.parse(output)
	if err != nil {
		return Result{}, fmt.Errorf("failed to parse test results: %w", err)
	}

	return *result, nil
}

func (r *Runner) FindPackage(file string) (string, error) {
	args := []string{"list", "-find", "-f", "{{.ImportPath}}", filepath.Dir(file)}

	r.logger.Info().
		Strs("args", args).
		Msg("executing 'go list'")

	output, exitCode, err := r.executor(r.ctx, r.rootDir, r.bin, args...)
	if err != nil {
		return "", fmt.Errorf("failed to find package: %w", err)
	}

	if exitCode != 0 {
		return "", fmt.Errorf("failed to find package, exitCode: %d", exitCode)
	}

	return strings.TrimSpace(string(output)), nil
}

func (r *Runner) makeTempDir(id uuid.UUID) (string, error) {
	r.tempDirOnce.Do(func() {
		if r.tempDir == "" {
			var err error

			r.tempDir, err = os.MkdirTemp("", defaultTempDir)
			if err != nil {
				panic(fmt.Errorf("'failed to create tmp directory': %w", err))
			}
		}
	})

	name := filepath.Join(r.tempDir, id.String())

	if err := os.MkdirAll(name, coverDirPermissions); err != nil {
		return "", fmt.Errorf("failed to create temp dir %q: %w", name, err)
	}

	return name, nil
}
