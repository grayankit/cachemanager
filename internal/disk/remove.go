package disk

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func RemoveWithin(root, target string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := ensureWithin(root, target); err != nil {
		return err
	}
	if target == root {
		return errf("refusing to delete %s", root)
	}
	lst, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if lst.Mode()&os.ModeSymlink != 0 {
		return os.Remove(target)
	}
	if !lst.IsDir() {
		return os.Remove(target)
	}
	return removeTree(root, target)
}

func EmptyDir(dir string) error {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	lst, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if lst.Mode()&os.ModeSymlink != 0 || !lst.IsDir() {
		return errf("refusing to empty %s", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := RemoveWithin(dir, filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func removeTree(root, target string) error {
	var dirs []string
	err := filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ensureWithin(root, path); err != nil {
			return err
		}
		if path == target {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return os.Remove(path)
		}
		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		return os.Remove(path)
	})
	if err != nil {
		return err
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Remove(dirs[i]); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return os.Remove(target)
}

func ensureWithin(root, path string) error {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if path == root {
		return nil
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return errf("%s is outside %s", path, root)
	}
	return nil
}
