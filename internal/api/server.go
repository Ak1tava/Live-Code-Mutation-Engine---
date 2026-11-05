// Package api serves the role-specific HTTP and WebSocket interfaces.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"lcme/internal/audit"
	"lcme/internal/hints"
	"lcme/internal/project"
	"lcme/internal/sandbox"
	"lcme/internal/selector"
)

type PrepareFunc func(context.Context, *project.Project, func(selector.Progress)) (*selector.Plan, error)

type Deps struct {
	Store          *audit.Store
	Importer       *project.Importer
	Runner         sandbox.Runner
	Prepare        PrepareFunc
	Hinter         hints.Hinter
	Now            func() time.Time
	WorkDir        string
	PublicURL      string
	Static         fs.FS
	AllowGitImport bool
	PrepareTimeout time.Duration
}

type Server struct {
	d        Deps
	sessions sync.Map // session ID → *sessionRuntime
	handler  http.Handler
	workers  chan struct{}
	createMu sync.Mutex // serializes the retained-session limit with Store.Create
}

// mu serializes state changes, test captures and publication for one session.
type sessionRuntime struct {
	mu     sync.Mutex
	active bool
	peers  map[*peer]struct{}
}

func New(d Deps) *Server {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Hinter == nil {
		d.Hinter = hints.Template()
	}
	if d.WorkDir == "" {
		d.WorkDir = os.TempDir()
	}
	if d.PrepareTimeout <= 0 {
		d.PrepareTimeout = 3 * time.Minute
	}
	s := &Server{d: d, workers: make(chan struct{}, 2)}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/audits", s.create)
	mux.HandleFunc("GET /api/a/{tok}", s.authorize(audit.RoleAuditor, s.getView))
	mux.HandleFunc("POST /api/a/{tok}/flags/{flagId}", s.authorize(audit.RoleAuditor, s.reviewFlag))
	mux.HandleFunc("POST /api/a/{tok}/finish", s.authorize(audit.RoleAuditor, s.finish))
	mux.HandleFunc("GET /api/s/{tok}", s.authorize(audit.RoleStudent, s.getView))
	mux.HandleFunc("POST /api/s/{tok}/start", s.authorize(audit.RoleStudent, s.start))
	mux.HandleFunc("POST /api/s/{tok}/run", s.authorize(audit.RoleStudent, s.run))
	mux.HandleFunc("POST /api/s/{tok}/flags", s.authorize(audit.RoleStudent, s.addFlag))
	mux.HandleFunc("POST /api/s/{tok}/finish", s.authorize(audit.RoleStudent, s.finish))
	mux.HandleFunc("GET /ws/{tok}", s.authorize("", s.websocket))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") })
	mux.HandleFunc("/", s.static)
	s.handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
			if r.ContentLength > maxJSONBody {
				writeError(w, http.StatusRequestEntityTooLarge, "Запрос слишком большой.")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
		}
		mux.ServeHTTP(w, r)
	})
	return s
}

func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) runtime(id string) *sessionRuntime {
	if rt, ok := s.sessions.Load(id); ok {
		return rt.(*sessionRuntime)
	}
	rt, _ := s.sessions.LoadOrStore(id, &sessionRuntime{peers: make(map[*peer]struct{})})
	return rt.(*sessionRuntime)
}

type authorizedHandler func(http.ResponseWriter, *http.Request, string, audit.Role)

func (s *Server) authorize(want audit.Role, next authorizedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, role, ok := s.d.Store.ByToken(r.PathValue("tok"))
		if !ok {
			writeError(w, http.StatusNotFound, "Сессия не найдена.")
			return
		}
		if want != "" && role != want {
			writeError(w, http.StatusForbidden, "Недостаточно прав для этого действия.")
			return
		}
		next(w, r, id, role)
	}
}

func (s *Server) sourceAllowed(src project.Source) bool {
	return s.d.AllowGitImport || (src.GitURL == "" && src.Example == "textkit")
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var src project.Source
	if !readBody(w, r, &src) {
		return
	}
	src.GitURL = strings.TrimSpace(src.GitURL)
	src.Example = strings.TrimSpace(src.Example)
	if (src.GitURL == "") == (src.Example == "") {
		writeError(w, http.StatusBadRequest, "Укажите ссылку на репозиторий или выберите демо.")
		return
	}
	if !s.sourceAllowed(src) {
		writeSessionError(w, errSourceDisabled)
		return
	}
	session := audit.New(src, s.d.Now())
	if err := s.admitSession(session); err != nil {
		writeSessionError(w, err)
		return
	}
	go s.prepare(session.ID, src)
	writeJSON(w, http.StatusCreated, struct {
		AuditorURL string `json:"auditorUrl"`
	}{"/a/" + session.AuditorToken})
}

// Admission happens before persistence and before launching any background work.
func (s *Server) admitSession(session *audit.Session) error {
	if !s.acquireWorker() {
		return errOverloaded
	}
	s.createMu.Lock()
	defer s.createMu.Unlock()
	if len(s.d.Store.IDs()) >= 64 {
		s.releaseWorker()
		return errSessionCapacity
	}
	if err := s.d.Store.Create(session); err != nil {
		s.releaseWorker()
		return err
	}
	return nil
}

func (s *Server) acquireWorker() bool {
	select {
	case s.workers <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Server) releaseWorker() { <-s.workers }

func (s *Server) prepare(id string, src project.Source) {
	defer s.releaseWorker()
	ctx, cancel := context.WithTimeout(context.Background(), s.d.PrepareTimeout)
	defer cancel()
	p, err := s.d.Importer.Import(ctx, src)
	if ctx.Err() != nil {
		s.failPreparation(id, "Превышено время подготовки задания. Попробуйте проект меньшего размера.")
		return
	}
	if err != nil {
		s.failPreparation(id, project.UserMessage(err))
		return
	}
	plan, err := s.d.Prepare(ctx, p, func(p selector.Progress) {
		if err := s.change(id, func(v *audit.Session) error {
			v.Progress = p
			return nil
		}); err != nil {
			log.Printf("api: failed to save preparation progress: %v", err)
		}
	})
	switch {
	case ctx.Err() != nil:
		s.failPreparation(id, "Превышено время подготовки задания. Попробуйте проект меньшего размера.")
		return
	case errors.Is(err, selector.ErrBaselineRed):
		s.failPreparation(id, "Тесты исходного проекта должны пройти три раза подряд.")
		return
	case errors.Is(err, selector.ErrNoTriple):
		s.failPreparation(id, "Не удалось подобрать три независимые мутации.")
		return
	case err != nil:
		s.failPreparation(id, project.UserMessage(err))
		return
	}

	input := make([]hints.Input, 0, len(plan.Mutations))
	for _, m := range plan.Mutations {
		input = append(input, hints.Input{M: m, Snippet: hints.Snippet(p.Files[m.File], m.Line)})
	}
	hintCtx, cancelHints := context.WithTimeout(ctx, 20*time.Second)
	h, err := s.d.Hinter.Hints(hintCtx, input)
	cancelHints()
	if ctx.Err() != nil {
		s.failPreparation(id, "Превышено время подготовки задания. Попробуйте проект меньшего размера.")
		return
	}
	if err != nil {
		h, _ = hints.Template().Hints(ctx, input)
	}
	if err := s.change(id, func(v *audit.Session) error {
		v.SetReady(p, plan, h)
		return nil
	}); err != nil {
		s.failPreparation(id, "Не удалось сохранить подготовленное задание.")
	}
}

func (s *Server) failPreparation(id, message string) {
	if err := s.change(id, func(v *audit.Session) error {
		v.Fail(message)
		return nil
	}); err != nil {
		log.Printf("api: failed to save preparation error: %v", err)
	}
}

func (s *Server) change(id string, update func(*audit.Session) error) error {
	rt := s.runtime(id)
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if err := s.d.Store.Update(id, update); err != nil {
		return err
	}
	s.broadcastLocked(id, rt)
	return nil
}

func (s *Server) start(w http.ResponseWriter, _ *http.Request, id string, _ audit.Role) {
	err := s.change(id, func(v *audit.Session) error {
		if !s.sourceAllowed(v.Source) {
			return errSourceDisabled
		}
		return v.Start(s.d.Now())
	})
	if err != nil {
		writeSessionError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) addFlag(w http.ResponseWriter, r *http.Request, id string, _ audit.Role) {
	var body struct {
		File string `json:"file"`
		Line int    `json:"line"`
		Note string `json:"note"`
	}
	if !readBody(w, r, &body) {
		return
	}
	var flag audit.Flag
	err := s.change(id, func(v *audit.Session) error {
		now := s.d.Now()
		if err := checkOpen(v, now); err != nil {
			return err
		}
		content, ok := v.Files[body.File]
		if !ok {
			content, ok = v.Tests[body.File]
		}
		if !ok {
			return audit.ErrUnknownFile
		}
		if body.Line < 1 || body.Line > 1+bytes.Count(content, []byte("\n")) {
			return errLine
		}
		var err error
		flag, err = v.AddFlag(body.File, body.Line, body.Note, now)
		return err
	})
	if err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, studentFlagOf(flag))
}

func (s *Server) reviewFlag(w http.ResponseWriter, r *http.Request, id string, _ audit.Role) {
	var body struct {
		MutationID string `json:"mutationId"`
		Accepted   *bool  `json:"accepted"`
	}
	if !readBody(w, r, &body) {
		return
	}
	if body.MutationID == "" || body.Accepted == nil {
		writeError(w, http.StatusBadRequest, "Укажите мутацию и решение по отметке.")
		return
	}
	err := s.change(id, func(v *audit.Session) error {
		if err := checkOpen(v, s.d.Now()); err != nil {
			return err
		}
		return v.ReviewFlag(r.PathValue("flagId"), body.MutationID, *body.Accepted)
	})
	if err != nil {
		writeSessionError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func checkOpen(v *audit.Session, now time.Time) error {
	if v.State != audit.StateRunning || !v.FinishRequestedAt.IsZero() {
		return audit.ErrBadState
	}
	if v.Expired(now) {
		return audit.ErrExpired
	}
	return nil
}

const maxJSONBody = 16 << 10

func readBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	err := d.Decode(dst)
	if err == nil {
		var extra json.RawMessage
		err = d.Decode(&extra)
		if err == io.EOF {
			return true
		}
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "Запрос слишком большой.")
	} else {
		writeError(w, http.StatusBadRequest, "Неверный формат запроса.")
	}
	return false
}

var (
	errSourceDisabled  = errors.New("api: source disabled")
	errBusy            = errors.New("api: run already in progress")
	errOverloaded      = errors.New("api: all workers busy")
	errSessionCapacity = errors.New("api: retained session limit reached")
	errLine            = errors.New("api: invalid line")
)

func sessionError(err error) (int, string) {
	switch {
	case errors.Is(err, errSourceDisabled):
		return http.StatusForbidden, "В этом режиме доступен только демо-проект textkit."
	case errors.Is(err, errBusy):
		return http.StatusConflict, "Тесты уже выполняются."
	case errors.Is(err, errOverloaded):
		return http.StatusTooManyRequests, "Сервер занят подготовкой и проверкой заданий. Повторите попытку позже."
	case errors.Is(err, errSessionCapacity):
		return http.StatusServiceUnavailable, "Достигнут предел сохранённых сессий. Обратитесь к организатору демо."
	case errors.Is(err, audit.ErrExpired):
		return http.StatusConflict, "Время сессии вышло."
	case errors.Is(err, audit.ErrBadState):
		return http.StatusConflict, "Действие недоступно на этом этапе сессии."
	case errors.Is(err, audit.ErrReadOnly):
		return http.StatusForbidden, "Файл доступен только для чтения."
	case errors.Is(err, audit.ErrTooBig):
		return http.StatusRequestEntityTooLarge, "Файл больше 256 КиБ."
	case errors.Is(err, audit.ErrUnknownFile):
		return http.StatusBadRequest, "Такого файла нет в задании."
	case errors.Is(err, audit.ErrUnknownFlag), errors.Is(err, audit.ErrUnknownMutation):
		return http.StatusBadRequest, "Не найдена отметка или мутация."
	case errors.Is(err, audit.ErrInvalidEvent):
		return http.StatusBadRequest, "Недопустимое событие."
	case errors.Is(err, errLine):
		return http.StatusBadRequest, "Строка вне границ файла."
	default:
		log.Printf("api: session operation failed: %v", err)
		return http.StatusInternalServerError, "Не удалось сохранить изменение."
	}
}

func writeSessionError(w http.ResponseWriter, err error) {
	status, message := sessionError(err)
	writeError(w, status, message)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, struct {
		Error string `json:"error"`
	}{message})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if s.d.Static == nil || (r.Method != http.MethodGet && r.Method != http.MethodHead) ||
		name == "api" || strings.HasPrefix(name, "api/") || name == "ws" || strings.HasPrefix(name, "ws/") {
		http.NotFound(w, r)
		return
	}
	if info, err := fs.Stat(s.d.Static, name); err == nil && !info.IsDir() && name != "index.html" {
		http.FileServerFS(s.d.Static).ServeHTTP(w, r)
		return
	}
	b, err := fs.ReadFile(s.d.Static, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(b))
}
