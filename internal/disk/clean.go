package disk

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CleanOptions struct {
	ID   string
	Keep int
	Home string
	Yes  bool
}

func Clean(ctx context.Context, opt CleanOptions) (Result, error) {
	if opt.Keep < 0 {
		opt.Keep = 2
	}
	if opt.Keep > 3 {
		return Result{ID: opt.ID, Error: "keep must be 0, 1, 2, or 3"}, errf("keep must be 0, 1, 2, or 3")
	}
	if opt.Home == "" {
		if os.Geteuid() == 0 {
			return Result{ID: opt.ID, Error: "refusing to clean user caches as root"}, errf("refusing to clean user caches as root")
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return Result{}, err
		}
		opt.Home = home
	}
	if opt.ID == "all" {
		return cleanAll(ctx, opt)
	}
	return cleanOne(ctx, opt)
}

func cleanAll(ctx context.Context, opt CleanOptions) (Result, error) {
	ids := []string{"pacman", "yay", "journal", "coredumps", "caches"}
	out := Result{ID: "all", DryRun: !opt.Yes, Applied: opt.Yes}
	var failed bool
	for _, id := range ids {
		one := opt
		one.ID = id
		res, err := cleanOne(ctx, one)
		if err != nil && res.Error == "" {
			res.Error = err.Error()
		}
		if res.Error != "" {
			failed = true
		}
		out.Reclaimed += res.Reclaimed
		out.Results = append(out.Results, res)
		if res.Log != "" {
			out.Log += res.Log
			if !strings.HasSuffix(out.Log, "\n") {
				out.Log += "\n"
			}
		}
	}
	out.Detail = Format(out.Reclaimed)
	if failed {
		out.Error = "one or more cleaners failed"
		return out, errf(out.Error)
	}
	return out, nil
}

func cleanOne(ctx context.Context, opt CleanOptions) (Result, error) {
	switch opt.ID {
	case "pacman":
		return cleanPacman(ctx, opt)
	case "yay":
		return cleanYay(ctx, opt)
	case "journal":
		return cleanJournal(ctx, opt)
	case "coredumps":
		return cleanCoredumps(ctx, opt)
	case "caches":
		return cleanCaches(ctx, opt)
	case "gradle":
		return emptyHome(ctx, opt, filepath.Join(opt.Home, ".gradle", "caches"), "Gradle caches")
	case "pub":
		return emptyHome(ctx, opt, filepath.Join(opt.Home, ".pub-cache", "hosted"), "Pub hosted cache")
	case "cargo":
		return cleanCargo(ctx, opt)
	case "shaders":
		return cleanShaders(ctx, opt)
	case "flatpak":
		return cleanFlatpak(ctx, opt)
	case "orphans":
		return cleanOrphans(ctx, opt)
	default:
		return Result{ID: opt.ID, Error: "unknown cleaner"}, errf("unknown cleaner %s", opt.ID)
	}
}

func cleanPacman(ctx context.Context, opt CleanOptions) (Result, error) {
	if opt.Keep < 0 {
		opt.Keep = 2
	}
	res := Result{ID: "pacman", DryRun: !opt.Yes}
	c, cancel := withTimeout(ctx, 2*time.Minute)
	defer cancel()
	installed := sumPaccache(c, opt.Keep, false)
	uninstalled := sumPaccache(c, 0, true)
	res.Reclaimed = installed + uninstalled
	res.Detail = "keep " + itoa(opt.Keep) + ", free " + Format(res.Reclaimed)
	if !opt.Yes {
		return res, nil
	}
	log, err := paccacheApply(c, opt.Keep)
	res.Log = log
	if err != nil {
		res.Error = err.Error()
		res.Applied = false
		return res, err
	}
	res.Applied = true
	return res, nil
}

func paccacheApply(ctx context.Context, keep int) (string, error) {
	if helperPath() != "" {
		if out, err := runRoot(ctx, "pacman", itoa(keep)); err == nil {
			return out, nil
		}
	}
	bin, err := systemBin("paccache")
	if err != nil {
		return "", err
	}
	if os.Geteuid() == 0 {
		first, err := run(ctx, bin, "-r", "-k", itoa(keep))
		if err != nil {
			return first, err
		}
		second, err := run(ctx, bin, "-r", "-u", "-k", "0")
		return first + second, err
	}
	first, err := run(ctx, "pkexec", bin, "-r", "-k", itoa(keep))
	if err != nil {
		return first, err
	}
	second, err := run(ctx, "pkexec", bin, "-r", "-u", "-k", "0")
	return first + second, err
}

func cleanYay(ctx context.Context, opt CleanOptions) (Result, error) {
	dir := filepath.Join(opt.Home, ".cache", "yay")
	files, err := PlanYay(dir, installedSet(ctx))
	if err != nil {
		return Result{ID: "yay", Error: err.Error()}, err
	}
	res := Result{ID: "yay", DryRun: !opt.Yes, Detail: "older tarballs only"}
	for _, f := range files {
		res.Reclaimed += f.Size
		if len(res.Files) < 40 {
			res.Files = append(res.Files, f.Path)
		}
	}
	if !opt.Yes {
		return res, nil
	}
	for _, f := range files {
		if err := RemoveWithin(dir, f.Path); err != nil {
			res.Error = err.Error()
			return res, err
		}
	}
	res.Applied = true
	return res, nil
}

func cleanJournal(ctx context.Context, opt CleanOptions) (Result, error) {
	res := Result{ID: "journal", DryRun: !opt.Yes, Detail: "vacuum to 300 MiB"}
	if !opt.Yes {
		return res, nil
	}
	c, cancel := withTimeout(ctx, time.Minute)
	defer cancel()
	if helperPath() != "" {
		out, err := runRoot(c, "journal")
		res.Log = out
		if err == nil {
			res.Applied = true
			return res, nil
		}
	}
	bin, err := systemBin("journalctl")
	if err != nil {
		return res, err
	}
	argv := []string{bin, "--vacuum-size=300M"}
	if os.Geteuid() != 0 {
		argv = append([]string{"pkexec"}, argv...)
	}
	out, err := run(c, argv[0], argv[1:]...)
	res.Log = out
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	res.Applied = true
	return res, nil
}

func cleanCoredumps(ctx context.Context, opt CleanOptions) (Result, error) {
	res := Result{ID: "coredumps", DryRun: !opt.Yes, Detail: "dumps older than 7 days"}
	if !opt.Yes {
		return res, nil
	}
	c, cancel := withTimeout(ctx, time.Minute)
	defer cancel()
	if helperPath() != "" {
		out, err := runRoot(c, "coredumps")
		res.Log = out
		if err == nil {
			res.Applied = true
			return res, nil
		}
	}
	bin, err := systemBin("find")
	if err != nil {
		return res, err
	}
	argv := []string{bin, "/var/lib/systemd/coredump", "-xdev", "-type", "f", "-mtime", "+7", "-print", "-delete"}
	if os.Geteuid() != 0 {
		argv = append([]string{"pkexec"}, argv...)
	}
	out, err := run(c, argv[0], argv[1:]...)
	res.Log = out
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	res.Applied = true
	return res, nil
}

func cleanCaches(ctx context.Context, opt CleanOptions) (Result, error) {
	res := Result{ID: "caches", DryRun: !opt.Yes}
	var roots []string
	for _, name := range cacheAllow {
		roots = append(roots, filepath.Join(opt.Home, ".cache", name))
	}
	roots = append(roots, filepath.Join(opt.Home, ".npm", "_cacache"))
	for _, dir := range roots {
		n := dirSize(ctx, dir)
		res.Reclaimed += n
		if n > 0 && len(res.Files) < 40 {
			res.Files = append(res.Files, dir)
		}
	}
	if !opt.Yes {
		return res, nil
	}
	for _, dir := range roots {
		if _, err := os.Lstat(dir); err != nil {
			continue
		}
		if err := EmptyDir(dir); err != nil {
			res.Error = err.Error()
			return res, err
		}
	}
	res.Applied = true
	return res, nil
}

func emptyHome(ctx context.Context, opt CleanOptions, dir, label string) (Result, error) {
	res := Result{ID: opt.ID, DryRun: !opt.Yes, Detail: label, Reclaimed: dirSize(ctx, dir)}
	if !opt.Yes {
		return res, nil
	}
	if err := EmptyDir(dir); err != nil {
		res.Error = err.Error()
		return res, err
	}
	res.Applied = true
	return res, nil
}

func cleanCargo(ctx context.Context, opt CleanOptions) (Result, error) {
	dirs := []string{
		filepath.Join(opt.Home, ".cargo", "registry", "cache"),
		filepath.Join(opt.Home, ".cargo", "registry", "src"),
		filepath.Join(opt.Home, ".cargo", "git", "checkouts"),
	}
	res := Result{ID: "cargo", DryRun: !opt.Yes, Detail: "registry and git checkouts"}
	for _, dir := range dirs {
		res.Reclaimed += dirSize(ctx, dir)
	}
	if !opt.Yes {
		return res, nil
	}
	for _, dir := range dirs {
		if _, err := os.Lstat(dir); err != nil {
			continue
		}
		if err := EmptyDir(dir); err != nil {
			res.Error = err.Error()
			return res, err
		}
	}
	res.Applied = true
	return res, nil
}

func cleanShaders(ctx context.Context, opt CleanOptions) (Result, error) {
	dirs := []string{
		filepath.Join(opt.Home, ".cache", "nvidia"),
		filepath.Join(opt.Home, ".cache", "mesa_shader_cache"),
	}
	res := Result{ID: "shaders", DryRun: !opt.Yes, Detail: "shader caches"}
	for _, dir := range dirs {
		res.Reclaimed += dirSize(ctx, dir)
	}
	if !opt.Yes {
		return res, nil
	}
	for _, dir := range dirs {
		if _, err := os.Lstat(dir); err != nil {
			continue
		}
		if err := EmptyDir(dir); err != nil {
			res.Error = err.Error()
			return res, err
		}
	}
	res.Applied = true
	return res, nil
}

func cleanFlatpak(ctx context.Context, opt CleanOptions) (Result, error) {
	res := Result{ID: "flatpak", DryRun: !opt.Yes, Detail: "unused runtimes only"}
	if !opt.Yes {
		return res, nil
	}
	bin, err := systemBin("flatpak")
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	c, cancel := withTimeout(ctx, 2*time.Minute)
	defer cancel()
	out, err := run(c, "pkexec", bin, "uninstall", "--unused", "--assumeyes")
	res.Log = out
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	res.Applied = true
	return res, nil
}

func cleanOrphans(ctx context.Context, opt CleanOptions) (Result, error) {
	c, cancel := withTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := run(c, "pacman", "-Qdtq")
	res := Result{ID: "orphans", DryRun: !opt.Yes}
	if err != nil && strings.TrimSpace(out) == "" {
		res.Detail = "no orphans"
		return res, nil
	}
	var names []string
	for _, name := range strings.Fields(out) {
		if !validName(name) {
			res.Error = "refusing unusual package name"
			return res, errf(res.Error)
		}
		names = append(names, name)
	}
	res.Detail = strings.Join(names, ", ")
	res.Files = names
	if len(names) == 0 || !opt.Yes {
		return res, nil
	}
	bin, err := systemBin("pacman")
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	args := append([]string{bin, "-Rns", "--noconfirm"}, names...)
	apply, cancelApply := withTimeout(ctx, 2*time.Minute)
	defer cancelApply()
	log, err := run(apply, "pkexec", args...)
	res.Log = log
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	res.Applied = true
	return res, nil
}

func dirSize(ctx context.Context, path string) int64 {
	if lst, err := os.Lstat(path); err != nil || lst.Mode()&os.ModeSymlink != 0 {
		return 0
	}
	sizes := duPaths(ctx, []string{path})
	return sizes[path]
}
