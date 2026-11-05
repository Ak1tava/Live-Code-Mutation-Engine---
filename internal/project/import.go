// Package project безопасно импортирует git-репозиторий или демо-проект.
package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Source struct {
	GitURL  string `json:"gitUrl,omitempty"`
	Example string `json:"example,omitempty"`
}

type Project struct {
	Source      Source
	Module      string
	GoDirective string            // "1.22"
	Files       map[string][]byte // все обычные файлы, пути со слэшами, без .git
	SourceFiles []string          // не-тестовые .go, по алфавиту
	TestFiles   []string          // *_test.go, по алфавиту
}

// GoFiles возвращает только SourceFiles: вход для selector.Prepare.
func (p *Project) GoFiles() map[string][]byte {
	out := make(map[string][]byte, len(p.SourceFiles))
	for _, f := range p.SourceFiles {
		out[f] = p.Files[f]
	}
	return out
}

type Limits struct {
	MaxFiles     int
	MaxBytes     int64
	CloneTimeout time.Duration
}

var DefaultLimits = Limits{MaxFiles: 2000, MaxBytes: 20 << 20, CloneTimeout: 60 * time.Second}

type Cloner func(ctx context.Context, url, dst string) error

type Importer struct {
	ExamplesDir, WorkDir string
	Clone                Cloner
	Limits               Limits
	MaxGo                string // например "1.26"
}

var (
	ErrBadURL         = errors.New("project: bad url")
	ErrClone          = errors.New("project: clone failed")
	ErrNoGoMod        = errors.New("project: no go.mod")
	ErrExternalDeps   = errors.New("project: external dependencies")
	ErrNoTests        = errors.New("project: no tests")
	ErrTooLarge       = errors.New("project: too large")
	ErrGoTooNew       = errors.New("project: go version too new")
	ErrUnknownExample = errors.New("project: unknown example")
)

// UserMessage возвращает текст ошибки для интерфейса на русском.
func UserMessage(err error) string {
	switch {
	case errors.Is(err, ErrBadURL):
		return "Нужна ссылка на репозиторий вида https://… без логина и пароля."
	case errors.Is(err, ErrClone):
		return "Не удалось скачать репозиторий. Он может быть приватным или не существовать."
	case errors.Is(err, ErrNoGoMod):
		return "В корне репозитория нет файла go.mod."
	case errors.Is(err, ErrExternalDeps):
		return "Проекты с внешними зависимостями не поддерживаются: уберите require из go.mod."
	case errors.Is(err, ErrNoTests):
		return "В проекте нет тестов (файлов *_test.go)."
	case errors.Is(err, ErrTooLarge):
		return "Проект слишком большой для импорта."
	case errors.Is(err, ErrGoTooNew):
		return "Версия Go в go.mod новее, чем поддерживает сервер."
	case errors.Is(err, ErrUnknownExample):
		return "Такого демо-проекта нет."
	}
	return "Не удалось импортировать проект."
}

func (im *Importer) limits() Limits {
	l := im.Limits
	if l.MaxFiles <= 0 {
		l.MaxFiles = DefaultLimits.MaxFiles
	}
	if l.MaxBytes <= 0 {
		l.MaxBytes = DefaultLimits.MaxBytes
	}
	if l.CloneTimeout <= 0 {
		l.CloneTimeout = DefaultLimits.CloneTimeout
	}
	return l
}

// Import загружает демо-проект или клонирует репозиторий и проверяет его.
func (im *Importer) Import(ctx context.Context, src Source) (*Project, error) {
	lim := im.limits()
	var root string
	if src.Example != "" {
		dir, err := im.exampleDir(src.Example)
		if err != nil {
			return nil, err
		}
		root = dir
	} else {
		if err := checkURL(src.GitURL); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(im.WorkDir, 0o755); err != nil {
			return nil, err
		}
		tmp, err := os.MkdirTemp(im.WorkDir, "clone-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(tmp)
		clone := im.Clone
		if clone == nil {
			clone = GitClone
		}
		cctx, cancel := context.WithTimeout(ctx, lim.CloneTimeout)
		defer cancel()
		if err := clone(cctx, src.GitURL, tmp); err != nil {
			if errors.Is(err, ErrClone) {
				return nil, err
			}
			return nil, fmt.Errorf("%w: %v", ErrClone, err)
		}
		root = tmp
	}
	return im.load(root, src, lim)
}

func checkURL(raw string) error {
	if raw == "" || strings.HasPrefix(raw, "-") {
		return ErrBadURL
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ErrBadURL
	}
	return nil
}

// exampleDir принимает только имя существующего каталога прямо в ExamplesDir.
func (im *Importer) exampleDir(name string) (string, error) {
	if name == "." || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return "", ErrUnknownExample
	}
	dir := filepath.Join(im.ExamplesDir, name)
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() {
		return "", ErrUnknownExample
	}
	return dir, nil
}

func (im *Importer) load(root string, src Source, lim Limits) (*Project, error) {
	files := map[string][]byte{}
	var total int64
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		if d.Name() == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		if !fi.Mode().IsRegular() { // симлинки, сокеты, устройства
			return nil
		}
		if len(files)+1 > lim.MaxFiles || total+fi.Size() > lim.MaxBytes {
			return ErrTooLarge
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		total += int64(len(b))
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = b
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrTooLarge) {
			return nil, ErrTooLarge
		}
		return nil, fmt.Errorf("project: read tree: %w", err)
	}

	gomod, ok := files["go.mod"]
	if !ok {
		return nil, ErrNoGoMod
	}
	module, goVer, hasRequire := parseGoMod(string(gomod))
	if hasRequire {
		return nil, ErrExternalDeps
	}
	if im.MaxGo != "" && goVer != "" && versionLess(im.MaxGo, goVer) {
		return nil, ErrGoTooNew
	}

	p := &Project{Source: src, Module: module, GoDirective: goVer, Files: files}
	for name := range files {
		if !strings.HasSuffix(name, ".go") || !compiled(name) {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			p.TestFiles = append(p.TestFiles, name)
		} else {
			p.SourceFiles = append(p.SourceFiles, name)
		}
	}
	sort.Strings(p.SourceFiles)
	sort.Strings(p.TestFiles)
	if len(p.TestFiles) == 0 {
		return nil, ErrNoTests
	}
	return p, nil
}

// compiled сообщает, попадёт ли файл в сборку: Go игнорирует testdata, vendor
// и всё, что начинается с "." или "_".
func compiled(name string) bool {
	for _, part := range strings.Split(path.Clean(name), "/") {
		if part == "testdata" || part == "vendor" || strings.HasPrefix(part, ".") || strings.HasPrefix(part, "_") {
			return false
		}
	}
	return true
}

func parseGoMod(s string) (module, goVer string, hasRequire bool) {
	for _, line := range strings.Split(s, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		switch {
		case f[0] == "require" || f[0] == "require(":
			hasRequire = true
		case f[0] == "module" && module == "" && len(f) > 1:
			module = strings.Trim(f[1], `"`)
		case f[0] == "go" && goVer == "" && len(f) > 1:
			goVer = f[1]
		}
	}
	return
}

// versionLess сравнивает major.minor как числа; нечитаемая версия считается слишком новой.
func versionLess(a, b string) bool {
	am, an, ok1 := majorMinor(a)
	bm, bn, ok2 := majorMinor(b)
	if !ok1 || !ok2 {
		return true
	}
	return am < bm || (am == bm && an < bn)
}

func majorMinor(v string) (int, int, bool) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, 0, false
	}
	maj, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minor := parts[1]
	end := 0
	for end < len(minor) && minor[end] >= '0' && minor[end] <= '9' {
		end++
	}
	min, err := strconv.Atoi(minor[:end])
	if err != nil {
		return 0, 0, false
	}
	return maj, min, true
}
