package disk

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseDF(t *testing.T) {
	out := "Filesystem 1B-blocks Used Available Use% Mounted on\n/dev/nvme0n1p8 1000 800 200 80% /\n"
	fs, err := ParseDF(out)
	if err != nil {
		t.Fatal(err)
	}
	if fs.Size != 1000 || fs.Used != 800 || fs.Avail != 200 || fs.UsePercent != 80 || fs.Mount != "/" {
		t.Fatalf("%+v", fs)
	}
	if fs.FreeLabel != "200 B free" {
		t.Fatalf("label %q", fs.FreeLabel)
	}
}

func TestParsePaccacheRejectsPaths(t *testing.T) {
	out := "==> Candidate packages:\nfoo-1.pkg.tar.zst\n../etc/passwd\nbar-1.pkg.tar.zst.sig\n"
	names := ParsePaccache(out)
	if len(names) != 2 || names[0] != "foo-1.pkg.tar.zst" || names[1] != "bar-1.pkg.tar.zst.sig" {
		t.Fatalf("%v", names)
	}
}

func TestPlanYayKeepsNewest(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "visual-studio-code-bin")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "visual-studio-code-bin-1.pkg.tar.zst")
	neu := filepath.Join(dir, "visual-studio-code-bin-2.pkg.tar.zst")
	src := filepath.Join(dir, "src")
	writeFile(t, old, "old")
	writeFile(t, neu, "new")
	writeFile(t, old+".sig", "sig")
	if err := os.Mkdir(src, 0o755); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret")
	writeFile(t, outside, "nope")
	if err := os.Symlink(outside, filepath.Join(dir, "link.pkg.tar.zst")); err != nil {
		t.Fatal(err)
	}

	files, err := PlanYay(root, map[string]bool{"visual-studio-code-bin": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %#v", files)
	}
	for _, f := range files {
		if f.Path == neu || f.Path == src {
			t.Fatalf("would delete %s", f.Path)
		}
	}
}

func TestPlanYaySkipsUninstalled(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "gone-bin")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "gone-bin-1.pkg.tar.zst"), "a")
	writeFile(t, filepath.Join(dir, "gone-bin-2.pkg.tar.zst"), "b")
	files, err := PlanYay(root, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("uninstalled dir must be left alone, got %#v", files)
	}
}

func TestRemoveWithinDoesNotFollowSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret")
	writeFile(t, secret, "keep")
	link := filepath.Join(root, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := RemoveWithin(root, link); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(secret); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "real")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(nested, "a"), "a")
	if err := os.Symlink(secret, filepath.Join(nested, "out")); err != nil {
		t.Fatal(err)
	}
	if err := EmptyDir(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(secret); err != nil {
		t.Fatalf("outside file deleted: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatal(err)
	}
}

func TestValidName(t *testing.T) {
	if !validName("visual-studio-code-bin") || validName("../etc") || validName("a/b") || validName("") {
		t.Fatal("name check")
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
