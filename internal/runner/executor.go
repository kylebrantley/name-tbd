package runner

import (
	"context"
	"errors"
	"os/exec"
)

var runner = func(ctx context.Context, dir, name string, args ...string) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitError *exec.ExitError

		if errors.As(err, &exitError) {
			return output, cmd.ProcessState.ExitCode(), nil
		}

		return output, cmd.ProcessState.ExitCode(), err
	}

	return output, cmd.ProcessState.ExitCode(), nil
}
