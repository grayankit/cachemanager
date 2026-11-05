package disk

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

func runOut(ctx context.Context, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func withTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, d)
}

func systemBin(name string) (string, error) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(resolved, "/usr/bin/") && !strings.HasPrefix(resolved, "/usr/local/bin/") {
		return "", errf("%s is not a system binary (%s)", name, resolved)
	}
	return resolved, nil
}

func helperPath() string {
	const p = "/usr/local/libexec/cachemanager-priv"
	lst, err := os.Lstat(p)
	if err != nil || lst.Mode()&os.ModeSymlink != 0 || lst.Mode().Perm()&0o022 != 0 {
		return ""
	}
	return p
}

func runRoot(ctx context.Context, sub string, extra ...string) (string, error) {
	if os.Geteuid() == 0 {
		if helper := helperPath(); helper != "" {
			return run(ctx, helper, append([]string{sub}, extra...)...)
		}
		return "", errf("root helper is not installed")
	}
	if helper := helperPath(); helper != "" {
		args := append([]string{"-n", helper, sub}, extra...)
		out, err := run(ctx, "sudo", args...)
		if err == nil {
			return out, nil
		}
	}
	return "", errf("root helper is not installed")
}
