package command

import (
	"context"
	"os/exec"
)

func BashCtx(ctx context.Context, cmd string) (int, []byte, error) {
	comm := exec.CommandContext(ctx, "/bin/bash", "-c", cmd)
	output, err := comm.Output()
	if err != nil {
		return comm.ProcessState.ExitCode(), nil, err
	}
	return comm.ProcessState.ExitCode(), output, nil
}

func Bash(cmd string) (int, []byte, error) {
	comm := exec.Command("/bin/bash", "-c", cmd)
	output, err := comm.Output()
	if err != nil {
		return comm.ProcessState.ExitCode(), nil, err
	}
	return comm.ProcessState.ExitCode(), output, nil
}
