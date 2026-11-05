package sandbox

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Batch is a directory holding one full project copy per variant: Dir/<id>/.
type Batch struct {
	Dir string
	IDs []string
}

// Runner executes the tests of every variant in a batch.
type Runner interface {
	Run(ctx context.Context, b Batch) ([]VariantResult, error)
	// GoVersion returns the toolchain version used, e.g. "go1.26.2".
	GoVersion(ctx context.Context) (string, error)
}

// WriteVariant writes files (slash-separated relative paths) under
// batchDir/id with directory mode 0755 and file mode 0644, so a container
// running as an unprivileged user can read them.
func WriteVariant(batchDir, id string, files map[string][]byte) error {
	if id == "" || id != path.Base(id) || id == "." || id == ".." {
		return fmt.Errorf("invalid variant id %q", id)
	}
	root := filepath.Join(batchDir, id)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	// The batch dir itself must be traversable by uid 65534 whatever the umask.
	if err := os.Chmod(batchDir, 0o755); err != nil {
		return err
	}
	for name, data := range files {
		clean := path.Clean(name)
		if path.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("invalid file path %q", name)
		}
		full := filepath.Join(root, filepath.FromSlash(clean))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return err
		}
		if err := os.Chmod(full, 0o644); err != nil {
			return err
		}
	}
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.Chmod(p, 0o755)
		}
		return nil
	})
}
