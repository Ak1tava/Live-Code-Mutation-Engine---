package project

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func importArchive(t *testing.T, entries ...tar.Header) []byte {
	t.Helper()
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	for _, h := range entries {
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if h.Size != 0 {
			if _, err := io.CopyN(tw, zeroReader{}, h.Size); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

func TestImportArchiveRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../escaped", "/tmp/escaped", "a/../../escaped", `a\escaped`, "a:escaped", "./../escaped"} {
		b := importArchive(t, tar.Header{Name: name, Mode: 0644, Typeflag: tar.TypeReg, Size: 1})
		if err := extractImport(bytes.NewReader(b), t.TempDir(), DefaultLimits); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
}

func TestImportArchiveSkipsLinksAndGit(t *testing.T) {
	b := importArchive(t,
		tar.Header{Name: "./", Typeflag: tar.TypeDir},
		tar.Header{Name: "./.git/config", Typeflag: tar.TypeReg, Size: 2},
		tar.Header{Name: "nested/.git/config", Typeflag: tar.TypeReg, Size: 2},
		tar.Header{Name: "symlink", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
		tar.Header{Name: "hardlink", Typeflag: tar.TypeLink, Linkname: "sub/a.go"},
		tar.Header{Name: "sub/a.go", Typeflag: tar.TypeReg, Size: 3},
	)
	dst := t.TempDir()
	if err := extractImport(bytes.NewReader(b), dst, DefaultLimits); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".git", "nested", "symlink", "hardlink"} {
		if _, err := os.Lstat(filepath.Join(dst, name)); !os.IsNotExist(err) {
			t.Errorf("unexpected %s: %v", name, err)
		}
	}
	if got, err := os.ReadFile(filepath.Join(dst, "sub/a.go")); err != nil || len(got) != 3 {
		t.Fatalf("regular file: %q %v", got, err)
	}
}

func TestImportArchiveCannotFollowExistingSymlinkOutsideRoot(t *testing.T) {
	dst, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dst, "sub")); err != nil {
		t.Fatal(err)
	}
	b := importArchive(t, tar.Header{Name: "sub/escaped", Typeflag: tar.TypeReg, Size: 1})
	if err := extractImport(bytes.NewReader(b), dst, DefaultLimits); err == nil {
		t.Fatal("followed existing outside symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, "escaped")); !os.IsNotExist(err) {
		t.Fatalf("outside file created: %v", err)
	}
}

func TestImportArchiveEnforcesLimitsBeforeFileWrite(t *testing.T) {
	b := importArchive(t, tar.Header{Name: "a", Typeflag: tar.TypeReg, Size: 4}, tar.Header{Name: "b", Typeflag: tar.TypeReg, Size: 4})
	for _, lim := range []Limits{{MaxFiles: 1, MaxBytes: 100}, {MaxFiles: 10, MaxBytes: 5}} {
		dst := t.TempDir()
		if err := extractImport(bytes.NewReader(b), dst, lim); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("limit: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dst, "b")); !os.IsNotExist(err) {
			t.Fatalf("excess file created: %v", err)
		}
	}
	// A declared huge size is rejected without receiving or allocating its body.
	var header bytes.Buffer
	tw := tar.NewWriter(&header)
	if err := tw.WriteHeader(&tar.Header{Name: "huge", Typeflag: tar.TypeReg, Size: 1 << 40}); err != nil {
		t.Fatal(err)
	}
	if err := extractImport(&header, t.TempDir(), DefaultLimits); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("huge header: %v", err)
	}
}

func TestImportArchiveBoundsTrailer(t *testing.T) {
	b := importArchive(t)
	input := io.MultiReader(bytes.NewReader(b), io.LimitReader(zeroReader{}, 40<<20))
	if err := extractImport(input, t.TempDir(), DefaultLimits); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("trailer: %v", err)
	}
}

func TestImportArchiveBoundsPathDepth(t *testing.T) {
	b := importArchive(t, tar.Header{Name: strings.Repeat("nested/", 128) + "a.go", Typeflag: tar.TypeReg, Size: 1})
	if err := extractImport(bytes.NewReader(b), t.TempDir(), DefaultLimits); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("excessive path depth: %v", err)
	}
}
