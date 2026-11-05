package disk

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type YayFile struct {
	Path string
	Size int64
}

func PlanYay(cacheDir string, installed map[string]bool) ([]YayFile, error) {
	info, err := os.Lstat(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, errf("%s is not a real directory", cacheDir)
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return nil, err
	}

	var doomed []YayFile
	for _, entry := range entries {
		name := entry.Name()
		if name == "completion.cache" || name == "vcs.json" || !validName(name) {
			continue
		}
		dir := filepath.Join(cacheDir, name)
		lst, err := os.Lstat(dir)
		if err != nil || lst.Mode()&os.ModeSymlink != 0 || !lst.IsDir() {
			continue
		}
		if !installed[name] {
			continue
		}
		old, err := oldTarballs(dir)
		if err != nil {
			return nil, err
		}
		doomed = append(doomed, old...)
	}
	sort.Slice(doomed, func(i, j int) bool { return doomed[i].Path < doomed[j].Path })
	return doomed, nil
}

func oldTarballs(dir string) ([]YayFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	type cand struct {
		path string
		size int64
		mod  time.Time
		name string
	}
	var tarballs []cand
	for _, entry := range entries {
		name := entry.Name()
		if !strings.Contains(name, ".pkg.tar.") || strings.HasSuffix(name, ".sig") {
			continue
		}
		if strings.Contains(name, "/") || strings.Contains(name, "..") {
			continue
		}
		path := filepath.Join(dir, name)
		lst, err := os.Lstat(path)
		if err != nil || !lst.Mode().IsRegular() {
			continue
		}
		tarballs = append(tarballs, cand{path: path, size: lst.Size(), mod: lst.ModTime(), name: name})
	}
	if len(tarballs) < 2 {
		return nil, nil
	}
	sort.Slice(tarballs, func(i, j int) bool {
		if !tarballs[i].mod.Equal(tarballs[j].mod) {
			return tarballs[i].mod.After(tarballs[j].mod)
		}
		return tarballs[i].name > tarballs[j].name
	})
	var doomed []YayFile
	for _, t := range tarballs[1:] {
		doomed = append(doomed, YayFile{Path: t.path, Size: t.size})
		sig := filepath.Join(dir, t.name+".sig")
		if lst, err := os.Lstat(sig); err == nil && lst.Mode().IsRegular() {
			doomed = append(doomed, YayFile{Path: sig, Size: lst.Size()})
		}
	}
	return doomed, nil
}
