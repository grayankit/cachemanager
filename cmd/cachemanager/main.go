package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"cachemanager/internal/disk"
)

const version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "--help":
		usage(stdout)
		return 0
	case "version":
		fmt.Fprintln(stdout, version)
		return 0
	case "brief":
		return cmdBrief(args[1:], stdout, stderr)
	case "disk":
		return cmdDisk(args[1:], stdout, stderr)
	case "clean":
		return cmdClean(args[1:], stdin, stdout, stderr)
	case "install-priv":
		fmt.Fprint(stdout, installText())
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		usage(stderr)
		return 2
	}
}

func cmdBrief(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("brief", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "df", "-B1", "-P", "/")
	raw, err := cmd.Output()
	out := string(raw)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fsys, err := disk.ParseDF(out)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *asJSON {
		return writeJSON(stdout, fsys)
	}
	fmt.Fprintln(stdout, fsys.Label)
	return 0
}

func cmdDisk(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("disk", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print json")
	quick := fs.Bool("quick", false, "skip the pacman candidate scan")
	keep := fs.Int("keep", 2, "installed pacman versions to keep")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *keep < 0 || *keep > 3 {
		fmt.Fprintln(stderr, "keep must be 0, 1, 2, or 3")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	rep, err := disk.Survey(ctx, disk.Options{Quick: *quick, Keep: *keep})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *asJSON {
		return writeJSON(stdout, rep)
	}
	writeText(stdout, rep)
	return 0
}

func cmdClean(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: cachemanager clean <id|all> [--yes] [--json] [--keep N]")
		return 2
	}
	id := args[0]
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "print json")
	yes := fs.Bool("yes", false, "delete without asking")
	keep := fs.Int("keep", -1, "installed pacman versions to keep")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *keep > 3 {
		fmt.Fprintln(stderr, "keep must be 0, 1, 2, or 3")
		return 2
	}
	opt := disk.CleanOptions{ID: id, Keep: *keep, Yes: *yes}
	if !*yes && !*asJSON && !isTerminal(stdin) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		preview, err := disk.Clean(ctx, opt)
		writeResult(stdout, preview)
		if err != nil {
			fmt.Fprintln(stderr, err)
		}
		fmt.Fprintln(stderr, "no terminal and no --yes; nothing deleted")
		return 2
	}
	if !*yes && !*asJSON && isTerminal(stdin) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		preview, err := disk.Clean(ctx, opt)
		if err != nil && preview.Error == "" {
			fmt.Fprintln(stderr, err)
			return 1
		}
		writeResult(stdout, preview)
		if preview.Error != "" {
			return 1
		}
		fmt.Fprint(stdout, "Delete these? [y/N] ")
		var answer string
		fmt.Fscanln(stdin, &answer)
		if strings.ToLower(strings.TrimSpace(answer)) != "y" && strings.ToLower(strings.TrimSpace(answer)) != "yes" {
			fmt.Fprintln(stdout, "cancelled")
			return 0
		}
		opt.Yes = true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	res, err := disk.Clean(ctx, opt)
	if *asJSON {
		code := writeJSON(stdout, res)
		if err != nil {
			return 1
		}
		return code
	}
	writeResult(stdout, res)
	if err != nil {
		if res.Error != "" {
			fmt.Fprintln(stderr, res.Error)
		} else {
			fmt.Fprintln(stderr, err)
		}
		return 1
	}
	if !res.Applied && !res.DryRun {
		fmt.Fprintln(stderr, "nothing deleted; pass --yes")
		return 2
	}
	return 0
}

func writeText(w io.Writer, rep disk.Report) {
	fmt.Fprintln(w, rep.Filesystem.Label)
	fmt.Fprintf(w, "\nSafe  %s    Expensive  %s    Left alone  %s\n",
		rep.Totals.SafeText, rep.Totals.ExpensiveText, rep.Totals.YoursText)
	last := ""
	for _, s := range rep.Stores {
		if s.Kind != last {
			fmt.Fprintf(w, "\n%s\n", title(s.Kind))
			last = s.Kind
		}
		right := s.ReclaimText
		if s.CanClean && s.Reclaimable > 0 {
			right = s.ReclaimText
		}
		fmt.Fprintf(w, "  %-22s %10s  %s\n", s.Label, right, s.Detail)
	}
	fmt.Fprintln(w, "\ncachemanager clean <id>     preview")
	fmt.Fprintln(w, "cachemanager clean all --yes   safe set only")
}

func writeResult(w io.Writer, res disk.Result) {
	if res.ID == "all" {
		for _, one := range res.Results {
			writeResult(w, one)
		}
		return
	}
	state := "preview"
	if res.Applied {
		state = "deleted"
	}
	if res.Error != "" {
		state = "failed"
	}
	fmt.Fprintf(w, "%s  %s  %s  %s\n", res.ID, state, disk.Format(res.Reclaimed), res.Detail)
	for _, f := range res.Files {
		fmt.Fprintf(w, "  %s\n", f)
	}
	if res.Log != "" {
		fmt.Fprintln(w, strings.TrimRight(res.Log, "\n"))
	}
	if res.Error != "" {
		fmt.Fprintln(w, res.Error)
	}
}

func title(kind string) string {
	switch kind {
	case "safe":
		return "Safe"
	case "expensive":
		return "Expensive — not included in clean all"
	default:
		return "Left alone"
	}
}

func writeJSON(w io.Writer, v any) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

func usage(w io.Writer) {
	fmt.Fprint(w, `cachemanager `+version+` — see what is using disk, then delete only what you name

  cachemanager disk              read-only report
  cachemanager disk --json
  cachemanager disk --quick
  cachemanager brief             filesystem line only
  cachemanager clean <id>        preview
  cachemanager clean <id> --yes  delete
  cachemanager clean all --yes   pacman, yay, journal, coredumps, allowlisted caches
  cachemanager install-priv      print the one-time root helper install

Safe cleaners keep a rollback copy: pacman keeps 2 versions, yay keeps the
newest tarball, the journal is vacuumed to 300 MiB. Gradle, pub, cargo,
shader caches, flatpak, and orphans are never part of "all".
Zen, Spotify, clipboard history, and Downloads are never deleted.

A settings page has no terminal, so it will not ask y/n. The DMS page arms
a button for five seconds and only then passes --yes.
`)
}

func installText() string {
	return `Root cleans (pacman, journal, coredumps) ask for your password through pkexec
until this helper is installed. It allows only those three operations.

From this repo:

  sudo ./scripts/install-priv.sh

After that, those three buttons do not prompt.
`
}
