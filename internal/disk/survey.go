package disk

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	journalKeep = 300 << 20
	reviewMin   = 100 << 20
)

var cacheAllow = []string{
	"thumbnails",
	"pip",
	"pnpm",
	"yarn",
	"go-build",
	"uv",
	"vscode-cpptools",
}

type Options struct {
	Quick bool
	Keep  int
	Home  string
}

func Survey(ctx context.Context, opt Options) (Report, error) {
	if opt.Keep < 0 {
		opt.Keep = 2
	}
	if opt.Home == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Report{}, err
		}
		opt.Home = home
	}

	fsys, err := readFS(ctx)
	if err != nil {
		return Report{}, err
	}

	cacheRoot := filepath.Join(opt.Home, ".cache")
	cacheSizes := duChildren(ctx, cacheRoot)
	sys := duPaths(ctx, []string{
		"/var/cache/pacman/pkg",
		"/var/log/journal",
		"/var/lib/systemd/coredump",
		"/var/lib/flatpak",
		filepath.Join(opt.Home, ".gradle", "caches"),
		filepath.Join(opt.Home, ".pub-cache", "hosted"),
		filepath.Join(opt.Home, ".pub-cache", "git"),
		filepath.Join(opt.Home, ".cargo", "registry", "cache"),
		filepath.Join(opt.Home, ".cargo", "registry", "src"),
		filepath.Join(opt.Home, ".cargo", "git", "checkouts"),
		filepath.Join(opt.Home, ".npm", "_cacache"),
		filepath.Join(opt.Home, ".local", "share", "Trash"),
		filepath.Join(opt.Home, "Downloads"),
	})

	var stores []Store
	stores = append(stores, pacmanStore(ctx, opt, sys["/var/cache/pacman/pkg"]))
	stores = append(stores, yayStore(ctx, filepath.Join(cacheRoot, "yay"), cacheSizes[filepath.Join(cacheRoot, "yay")]))
	stores = append(stores, journalStore(sys["/var/log/journal"]))
	stores = append(stores, coredumpStore(ctx))
	stores = append(stores, allowCacheStore(opt.Home, cacheSizes, sys[filepath.Join(opt.Home, ".npm", "_cacache")]))
	stores = append(stores, gradleStore(sys[filepath.Join(opt.Home, ".gradle", "caches")]))
	stores = append(stores, pubStore(sys))
	stores = append(stores, cargoStore(opt.Home, sys))
	stores = append(stores, shaderStore(cacheSizes, cacheRoot))
	if s, ok := flatpakStore(sys["/var/lib/flatpak"]); ok {
		stores = append(stores, s)
	}
	if s, ok := orphanStore(ctx); ok {
		stores = append(stores, s)
	}
	stores = append(stores, yoursStores(opt.Home, cacheSizes, sys)...)
	stores = append(stores, reviewStores(cacheRoot, cacheSizes)...)

	rep := Report{
		Filesystem: fsys,
		Stores:     stores,
		RootHelper: helperPath() != "",
		ScannedAt:  time.Now().Format(time.RFC3339),
	}
	rep.Totals = totals(stores)
	return rep, nil
}

func readFS(ctx context.Context) (FS, error) {
	c, cancel := withTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := run(c, "df", "-B1", "-P", "/")
	if err != nil && out == "" {
		return FS{}, err
	}
	return ParseDF(out)
}

func duPaths(ctx context.Context, paths []string) map[string]int64 {
	var exist []string
	for _, p := range paths {
		if lst, err := os.Lstat(p); err == nil && lst.Mode()&os.ModeSymlink == 0 {
			exist = append(exist, p)
		}
	}
	if len(exist) == 0 {
		return map[string]int64{}
	}
	c, cancel := withTimeout(ctx, 45*time.Second)
	defer cancel()
	stdout, _, err := runOut(c, "du", append([]string{"-s", "-B1"}, exist...)...)
	if err != nil && stdout == "" {
		return map[string]int64{}
	}
	return ParseDU(stdout)
}

func duChildren(ctx context.Context, dir string) map[string]int64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return map[string]int64{}
	}
	var paths []string
	for _, entry := range entries {
		p := filepath.Join(dir, entry.Name())
		if lst, err := os.Lstat(p); err == nil && lst.Mode()&os.ModeSymlink == 0 {
			paths = append(paths, p)
		}
	}
	return duPaths(ctx, paths)
}

func pacmanStore(ctx context.Context, opt Options, size int64) Store {
	s := Store{
		ID: "pacman", Kind: "safe", Label: "Pacman cache",
		Path: "/var/cache/pacman/pkg", Size: size, Icon: "inventory_2", NeedsRoot: true,
	}
	if opt.Quick {
		s.Detail = "full scan measures what keep " + itoa(opt.Keep) + " would free"
		return finishStore(s)
	}
	c, cancel := withTimeout(ctx, 30*time.Second)
	defer cancel()
	installed, uninstalled := paccacheSaved(c, opt.Keep)
	s.Reclaimable = installed + uninstalled
	s.Detail = "keep " + itoa(opt.Keep) + " installed versions"
	if uninstalled > 0 {
		s.Detail += ", plus " + Format(uninstalled) + " from uninstalled packages"
	}
	s.CanClean = s.Reclaimable > 0
	return finishStore(s)
}

func paccacheSaved(ctx context.Context, keep int) (int64, int64) {
	return sumPaccache(ctx, keep, false), sumPaccache(ctx, 0, true)
}

func sumPaccache(ctx context.Context, keep int, uninstalled bool) int64 {
	args := []string{"-d", "-v", "-k", itoa(keep)}
	if uninstalled {
		args = append(args, "-u")
	}
	out, _, err := runOut(ctx, "paccache", args...)
	if err != nil && out == "" {
		return 0
	}
	var total int64
	for _, name := range ParsePaccache(out) {
		lst, err := os.Lstat(filepath.Join("/var/cache/pacman/pkg", name))
		if err != nil || !lst.Mode().IsRegular() {
			continue
		}
		total += lst.Size()
	}
	return total
}

func yayStore(ctx context.Context, dir string, size int64) Store {
	s := Store{
		ID: "yay", Kind: "safe", Label: "yay cache",
		Path: dir, Size: size, Icon: "construction",
	}
	installed := installedSet(ctx)
	files, err := PlanYay(dir, installed)
	if err != nil {
		s.Detail = err.Error()
		return finishStore(s)
	}
	for _, f := range files {
		s.Reclaimable += f.Size
	}
	s.Detail = "keeps the newest tarball of each installed package"
	if len(files) == 0 {
		s.Detail = "no older tarballs"
	}
	s.CanClean = s.Reclaimable > 0
	return finishStore(s)
}

func installedSet(ctx context.Context) map[string]bool {
	c, cancel := withTimeout(ctx, 10*time.Second)
	defer cancel()
	out, _, err := runOut(c, "pacman", "-Qq")
	set := map[string]bool{}
	if err != nil && out == "" {
		return set
	}
	for _, name := range strings.Fields(out) {
		if validName(name) {
			set[name] = true
		}
	}
	return set
}

func journalStore(size int64) Store {
	s := Store{
		ID: "journal", Kind: "safe", Label: "Systemd journal",
		Path: "/var/log/journal", Size: size, Icon: "description", NeedsRoot: true,
	}
	if size > journalKeep {
		s.Reclaimable = size - journalKeep
		s.CanClean = true
	}
	s.Detail = "vacuum to 300 MiB"
	return finishStore(s)
}

func coredumpStore(ctx context.Context) Store {
	s := Store{
		ID: "coredumps", Kind: "safe", Label: "Coredumps",
		Path: "/var/lib/systemd/coredump", Icon: "bug_report", NeedsRoot: true,
		Detail: "dumps older than 7 days",
	}
	c, cancel := withTimeout(ctx, 10*time.Second)
	defer cancel()
	entries, err := os.ReadDir(s.Path)
	if err != nil {
		s.Detail = "not readable"
		return finishStore(s)
	}
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	var total int64
	for _, entry := range entries {
		if c.Err() != nil {
			break
		}
		path := filepath.Join(s.Path, entry.Name())
		lst, err := os.Lstat(path)
		if err != nil || !lst.Mode().IsRegular() {
			continue
		}
		total += lst.Size()
		if lst.ModTime().Before(cutoff) {
			s.Reclaimable += lst.Size()
		}
	}
	s.Size = total
	s.CanClean = s.Reclaimable > 0
	return finishStore(s)
}

func allowCacheStore(home string, cacheSizes map[string]int64, npm int64) Store {
	s := Store{
		ID: "caches", Kind: "safe", Label: "Rebuildable caches",
		Icon: "folder", Detail: "pip, pnpm, yarn, go, uv, thumbnails, vscode, npm",
	}
	var parts []string
	for _, name := range cacheAllow {
		p := filepath.Join(home, ".cache", name)
		n := cacheSizes[p]
		if n <= 0 {
			continue
		}
		s.Size += n
		s.Reclaimable += n
		parts = append(parts, name+" "+Format(n))
	}
	if npm > 0 {
		s.Size += npm
		s.Reclaimable += npm
		parts = append(parts, "npm "+Format(npm))
	}
	if len(parts) > 0 {
		s.Detail = strings.Join(parts, ", ")
	}
	s.CanClean = s.Reclaimable > 0
	return finishStore(s)
}

func gradleStore(size int64) Store {
	s := Store{
		ID: "gradle", Kind: "expensive", Label: "Gradle caches",
		Icon: "android", Size: size, Reclaimable: size,
		Detail:   "only ~/.gradle/caches; JDKs and wrappers stay",
		CanClean: size > 0,
	}
	return finishStore(s)
}

func pubStore(sys map[string]int64) Store {
	hosted := int64(0)
	git := int64(0)
	for p, n := range sys {
		if strings.HasSuffix(p, "/.pub-cache/hosted") {
			hosted = n
		}
		if strings.HasSuffix(p, "/.pub-cache/git") {
			git = n
		}
	}
	s := Store{
		ID: "pub", Kind: "expensive", Label: "Pub cache",
		Icon: "flutter", Size: hosted + git, Reclaimable: hosted,
		Detail:   "clears hosted packages, leaves git checkouts (" + Format(git) + ")",
		CanClean: hosted > 0,
	}
	return finishStore(s)
}

func cargoStore(home string, sys map[string]int64) Store {
	var size int64
	for _, rel := range []string{
		filepath.Join(".cargo", "registry", "cache"),
		filepath.Join(".cargo", "registry", "src"),
		filepath.Join(".cargo", "git", "checkouts"),
	} {
		size += sys[filepath.Join(home, rel)]
	}
	return finishStore(Store{
		ID: "cargo", Kind: "expensive", Label: "Cargo registry",
		Icon: "inventory", Size: size, Reclaimable: size,
		Detail:   "registry cache, extracted crates, and git checkouts; ~/.cargo/bin stays",
		CanClean: size > 0,
	})
}

func shaderStore(cacheSizes map[string]int64, cacheRoot string) Store {
	nvidia := cacheSizes[filepath.Join(cacheRoot, "nvidia")]
	mesa := cacheSizes[filepath.Join(cacheRoot, "mesa_shader_cache")]
	s := Store{
		ID: "shaders", Kind: "expensive", Label: "Shader caches",
		Icon: "gradient", Size: nvidia + mesa, Reclaimable: nvidia + mesa,
		Detail:   "NVIDIA and Mesa; the next launch stutters while they rebuild",
		CanClean: nvidia+mesa > 0,
	}
	return finishStore(s)
}

func flatpakStore(size int64) (Store, bool) {
	if _, err := os.Lstat("/var/lib/flatpak"); err != nil {
		return Store{}, false
	}
	s := finishStore(Store{
		ID: "flatpak", Kind: "expensive", Label: "Flatpak unused",
		Path: "/var/lib/flatpak", Size: size, Icon: "apps",
		Detail:   "removes unused runtimes only, not installed apps",
		CanClean: true, NeedsRoot: true,
	})
	s.ReclaimText = "unused only"
	return s, true
}

func orphanStore(ctx context.Context) (Store, bool) {
	c, cancel := withTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := run(c, "pacman", "-Qdtq")
	if err != nil && strings.TrimSpace(out) == "" {
		return Store{}, false
	}
	var names []string
	for _, name := range strings.Fields(out) {
		if validName(name) {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return Store{}, false
	}
	s := finishStore(Store{
		ID: "orphans", Kind: "expensive", Label: "Orphan packages",
		Icon: "delete", Detail: strings.Join(names, ", "),
		CanClean: true, NeedsRoot: true,
	})
	s.ReclaimText = itoa(len(names)) + " packages"
	return s, true
}

func yoursStores(home string, cacheSizes, sys map[string]int64) []Store {
	type item struct {
		id, label, icon, path string
	}
	items := []item{
		{"zen", "Zen", "public", filepath.Join(home, ".cache", "zen")},
		{"spotify", "Spotify", "music_note", filepath.Join(home, ".cache", "spotify")},
		{"google", "Google", "language", filepath.Join(home, ".cache", "Google")},
		{"cliphist", "Clipboard history", "content_paste", filepath.Join(home, ".cache", "cliphist")},
		{"trash", "Trash", "delete", filepath.Join(home, ".local", "share", "Trash")},
		{"downloads", "Downloads", "download", filepath.Join(home, "Downloads")},
	}
	var stores []Store
	for _, it := range items {
		n := cacheSizes[it.path]
		if n == 0 {
			n = sys[it.path]
		}
		if n <= 0 {
			continue
		}
		stores = append(stores, finishStore(Store{
			ID: it.id, Kind: "yours", Label: it.label, Path: it.path,
			Size: n, Icon: it.icon, Detail: "left alone",
		}))
	}
	return stores
}

func reviewStores(cacheRoot string, cacheSizes map[string]int64) []Store {
	known := map[string]bool{
		"yay": true, "zen": true, "spotify": true, "Google": true, "cliphist": true,
		"nvidia": true, "mesa_shader_cache": true, "nix": true,
	}
	for _, name := range cacheAllow {
		known[name] = true
	}
	type pair struct {
		name string
		size int64
	}
	var big []pair
	for path, size := range cacheSizes {
		if size < reviewMin || filepath.Dir(path) != cacheRoot {
			continue
		}
		name := filepath.Base(path)
		if known[name] {
			continue
		}
		big = append(big, pair{name, size})
	}
	sort.Slice(big, func(i, j int) bool { return big[i].size > big[j].size })
	if len(big) > 8 {
		big = big[:8]
	}
	var stores []Store
	for _, p := range big {
		stores = append(stores, finishStore(Store{
			ID: "review-" + p.name, Kind: "yours", Label: p.name,
			Path: filepath.Join(cacheRoot, p.name), Size: p.size,
			Icon: "help", Detail: "not on the allowlist",
		}))
	}
	return stores
}

func totals(stores []Store) Totals {
	var t Totals
	for _, s := range stores {
		switch s.Kind {
		case "safe":
			t.Safe += s.Reclaimable
		case "expensive":
			t.Expensive += s.Reclaimable
		default:
			t.Yours += s.Size
		}
	}
	t.SafeText = Format(t.Safe)
	t.ExpensiveText = Format(t.Expensive)
	t.YoursText = Format(t.Yours)
	return t
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
