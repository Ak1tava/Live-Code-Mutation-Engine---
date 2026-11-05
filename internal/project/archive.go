package project

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
)

// No extraction tool runs on the host. Only bounded regular files are copied;
// Root prevents symlink races escaping dst, even for a pre-existing destination.
func extractImport(src io.Reader, dst string, lim Limits) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(dst)
	if err != nil {
		return err
	}
	defer root.Close()
	// Metadata/empty entries and tar trailers must be bounded as well as payload.
	input := &io.LimitedReader{R: src, N: 32<<20 + 1}
	tr := tar.NewReader(input)
	var total int64
	files, entries := 0, 0
	directories := make(map[string]bool)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		entries++
		if entries > 4096 {
			return ErrTooLarge
		}
		if (h.Name == "." || h.Name == "./") && h.Typeflag == tar.TypeDir {
			continue
		}
		name := strings.TrimPrefix(h.Name, "./")
		name = strings.TrimSuffix(name, "/")
		if len(name) > 4096 || !fs.ValidPath(name) || strings.ContainsAny(name, `\:`) {
			return fmt.Errorf("invalid archive path")
		}
		if strings.Count(name, "/") >= 128 {
			return ErrTooLarge
		}
		git := false
		for _, part := range strings.Split(name, "/") {
			git = git || part == ".git"
		}
		if git || h.Typeflag != tar.TypeReg {
			continue // Never create links, devices, or even attacker-supplied dirs.
		}
		files++
		if files > min(lim.MaxFiles, 2000) || h.Size < 0 || h.Size > min(lim.MaxBytes, 20<<20)-total {
			return ErrTooLarge
		}
		if parent := path.Dir(name); parent != "." {
			var current string
			for _, part := range strings.Split(parent, "/") {
				current = path.Join(current, part)
				if directories[current] {
					continue
				}
				if len(directories) == 4096 {
					return ErrTooLarge
				}
				if err := root.Mkdir(current, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
					return err
				}
				directories[current] = true
			}
		}
		f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		n, copyErr := io.CopyN(f, tr, h.Size)
		closeErr := f.Close()
		if copyErr != nil || closeErr != nil {
			return errors.Join(copyErr, closeErr)
		}
		total += n
	}
	// Consume stdout before cmd.Wait without allowing a malicious trailer to grow
	// memory or keep docker's pipe blocked indefinitely (ctx supplies the timeout).
	_, err = io.Copy(io.Discard, input)
	if input.N == 0 {
		return ErrTooLarge
	}
	return err
}
