package gitexec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
)

func WriteAdvertisedRefs(
	ctx context.Context,
	w io.Writer,
	service Service,
	repoPath string,
	logger *slog.Logger,
) error {
	header := fmt.Sprintf("# service=%s\n", service)
	if _, err := fmt.Fprintf(w, "%04x%s0000", len(header)+4, header); err != nil {
		return err
	}

	cmd := exec.CommandContext(
		ctx,
		"git",
		service.ShortName(),
		"--stateless-rpc",
		"--advertise-refs",
		repoPath,
	)

	var stderr bytes.Buffer
	cmd.Stdout = w
	cmd.Stderr = &stderr

	err := cmd.Run()
	if stderr.Len() > 0 {
		logger.Debug("git advertise stderr", "service", service, "stderr", stderr.String())
	}
	return err
}

func RunRPC(
	ctx context.Context,
	stdin io.Reader,
	stdout io.Writer,
	service Service,
	repoPath string,
	logger *slog.Logger,
) error {
	cmd := exec.CommandContext(
		ctx,
		"git",
		service.ShortName(),
		"--stateless-rpc",
		repoPath,
	)

	var stderr bytes.Buffer
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if stderr.Len() > 0 {
		logger.Debug("git rpc stderr", "service", service, "stderr", stderr.String())
	}
	return err
}
