package runner

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

type Package struct {
	Name     string
	Pass     bool
	Passed   int              // number of tests within the package that passed
	Failed   int              // number of tests within the package that failed
	Skipped  int              // number of tests within the package that were skipped
	Tests    map[string]Event // TODO: maybe refactor this to a slice
	Coverage float64
	Elapsed  float64
}

type Event struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Output  string    `json:"Output"`
	Elapsed float64   `json:"Elapsed"`
}

var coverageRegex = regexp.MustCompile(`([0-9]*\.?[0-9]*)\s*%`)

func (r *Runner) parse(input []byte) (map[string]*Package, error) {
	scanner := bufio.NewScanner(bytes.NewReader(input))
	scanner.Split(bufio.ScanLines)

	packages := make(map[string]*Package)

	for scanner.Scan() {
		line := scanner.Bytes()
		event := Event{}

		if err := json.Unmarshal(line, &event); err != nil {
			r.logger.Error().
				Err(err).
				Msg("error decoding event")

			continue
		}

		if event.Package == "" {
			r.logger.Info().
				Interface("event", event).
				Msg("skipping event")

			continue
		}

		if err := r.handleEvent(event, packages); err != nil {
			r.logger.Error().
				Err(err).
				Msg("failed to handle event")

			continue
		}
	}

	return packages, nil
}

func (r *Runner) handleEvent(e Event, packages map[string]*Package) error {
	if e.Test != "" {
		// TODO: parsing table tests is a little weird?
		return r.handleTestEvent(e, packages)
	}

	return r.handlePackageEvent(e, packages)
}

func (r *Runner) handlePackageEvent(e Event, packages map[string]*Package) error {
	pkg, exists := packages[e.Package]
	if !exists {
		r.logger.Info().
			Str("package", e.Package).
			Msg("handling new package")

		pkg = &Package{
			Name:  e.Package,
			Tests: make(map[string]Event),
		}

		packages[e.Package] = pkg
	}

	switch e.Action {
	case "pass":
		pkg.Pass = true
		pkg.Elapsed = e.Elapsed
	case "fail":
		pkg.Pass = false
		pkg.Elapsed = e.Elapsed
	case "output":
		coverage := coverageRegex.FindStringSubmatch(e.Output)
		if len(coverage) >= 2 {
			parsedCoverage, err := strconv.ParseFloat(coverage[1], 64)
			if err != nil {
				return fmt.Errorf("failed to convert %s to float: %w", coverage[1], err)
			}

			packages[e.Package].Coverage = parsedCoverage
		}
	default:
		r.logger.Info().
			Interface("event", e).
			Msg("unhandled package event")
	}

	return nil
}

func (r *Runner) handleTestEvent(e Event, packages map[string]*Package) error {
	pkg, exists := packages[e.Package]
	if !exists {
		return fmt.Errorf("package does not exist: %s", e.Package)
	}

	// TODO: does this make sense
	_, exists = pkg.Tests[e.Test]
	if !exists {
		pkg.Tests[e.Test] = e
	}

	switch e.Action {
	case "pass":
		pkg.Passed++
		pkg.Elapsed = e.Elapsed
	case "fail":
		pkg.Failed++
		pkg.Elapsed = e.Elapsed
	case "skip":
		pkg.Skipped++
		// TODO: verify if skipped tests have elapsed
		pkg.Elapsed = e.Elapsed
	default:
		r.logger.Info().
			Interface("event", e).
			Msg("unhandled test event")
	}

	return nil
}
