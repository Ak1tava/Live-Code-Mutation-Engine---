package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"lcme/internal/audit"
	"lcme/internal/sandbox"
)

type runJob struct {
	id      string
	number  int
	at      time.Time
	final   bool
	allowed bool
	files   map[string][]byte
}

func (s *Server) run(w http.ResponseWriter, _ *http.Request, id string, _ audit.Role) {
	rt := s.runtime(id)
	rt.mu.Lock()
	err := s.startRunLocked(context.Background(), id, rt, false)
	if err == nil {
		s.broadcastLocked(id, rt)
	}
	rt.mu.Unlock()
	if err != nil {
		writeSessionError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) finish(w http.ResponseWriter, _ *http.Request, id string, _ audit.Role) {
	rt := s.runtime(id)
	rt.mu.Lock()
	changed, err := s.requestFinishLocked(context.Background(), id, rt, false)
	if changed {
		s.broadcastLocked(id, rt)
	}
	rt.mu.Unlock()
	if err != nil {
		writeSessionError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// Tick queues final runs for expired sessions and resumes persisted finish requests.
func (s *Server) Tick(ctx context.Context) {
	for _, id := range s.d.Store.IDs() {
		if ctx.Err() != nil {
			return
		}
		rt := s.runtime(id)
		rt.mu.Lock()
		changed, err := s.requestFinishLocked(ctx, id, rt, true)
		if changed {
			s.broadcastLocked(id, rt)
		}
		rt.mu.Unlock()
		if err != nil {
			log.Printf("api: could not queue final run: %v", err)
		}
	}
}

func (s *Server) requestFinishLocked(ctx context.Context, id string, rt *sessionRuntime, dueOnly bool) (bool, error) {
	now := s.d.Now()
	var state audit.State
	var deadline, requested time.Time
	var expired bool
	s.d.Store.View(id, func(v *audit.Session) {
		state, deadline, requested = v.State, v.Deadline, v.FinishRequestedAt
		expired = v.Expired(now)
	})
	if state == audit.StateFinished || (dueOnly && (state != audit.StateRunning || !expired && requested.IsZero())) {
		return false, nil
	}
	if state != audit.StateRunning {
		return false, audit.ErrBadState
	}

	changed := requested.IsZero()
	if changed {
		if expired {
			now = deadline
		}
		if err := s.d.Store.Update(id, func(v *audit.Session) error {
			v.FinishRequestedAt = now
			return nil
		}); err != nil {
			return false, err
		}
	}
	if rt.active {
		return changed, nil
	}
	err := s.startRunLocked(ctx, id, rt, true)
	if errors.Is(err, errOverloaded) {
		// The persisted request freezes edits; Tick retries without a waiting goroutine.
		return changed, nil
	}
	return changed || err == nil, err
}

func (s *Server) startRunLocked(ctx context.Context, id string, rt *sessionRuntime, final bool) error {
	if rt.active {
		return errBusy
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	job := runJob{id: id, at: s.d.Now(), final: final}
	var err error
	s.d.Store.View(id, func(v *audit.Session) {
		job.allowed = s.sourceAllowed(v.Source)
		if final {
			job.at = v.FinishRequestedAt
		} else {
			if err = checkOpen(v, job.at); err != nil {
				return
			}
			if !job.allowed {
				err = errSourceDisabled
				return
			}
		}
		if !s.acquireWorker() {
			err = errOverloaded
			return
		}
		job.number = len(v.Runs) + 1
		job.files = v.RunFiles()
		for path, content := range job.files {
			job.files[path] = append([]byte(nil), content...)
		}
	})
	if err != nil {
		return err
	}
	rt.active = true
	go s.execute(ctx, rt, job)
	return nil
}

func (s *Server) execute(ctx context.Context, rt *sessionRuntime, job runJob) {
	result := s.runTests(ctx, job)
	s.releaseWorker()

	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.active = false
	if ctx.Err() == nil {
		err := s.d.Store.Update(job.id, func(v *audit.Session) error {
			v.AddRun(result, job.final, job.at)
			if job.final {
				v.Finish(job.at)
			}
			return nil
		})
		if err != nil {
			log.Printf("api: could not save run: %v", err)
			s.broadcastErrorLocked(rt, "Не удалось сохранить результаты тестов.")
		}
		if !job.final {
			// A deadline or manual finish can arrive during an ordinary run.
			// Keep the session frozen and start the final run once this run ends.
			if _, err := s.requestFinishLocked(ctx, job.id, rt, true); err != nil {
				log.Printf("api: could not queue final run: %v", err)
			}
		}
	}
	s.broadcastLocked(job.id, rt)
}

func (s *Server) runTests(parent context.Context, job runJob) sandbox.VariantResult {
	const variant = "v"
	started := time.Now()
	failure := func(message string) sandbox.VariantResult {
		return sandbox.VariantResult{ID: variant, BuildError: message, DurationMs: time.Since(started).Milliseconds()}
	}
	if !job.allowed {
		return failure("В этом режиме доступен только демо-проект textkit.")
	}
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	dir := filepath.Join(s.d.WorkDir, job.id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("api: could not create run directory: %v", err)
		return failure("Не удалось подготовить файлы для тестов.")
	}
	defer os.Remove(dir) // Only succeeds once the session's batches have been removed.
	batch, err := os.MkdirTemp(dir, "run-"+strconv.Itoa(job.number)+"-")
	if err != nil {
		log.Printf("api: could not create run batch: %v", err)
		return failure("Не удалось подготовить файлы для тестов.")
	}
	defer os.RemoveAll(batch)
	if err := sandbox.WriteVariant(batch, variant, job.files); err != nil {
		log.Printf("api: could not write test variant: %v", err)
		return failure("Не удалось подготовить файлы для тестов.")
	}
	if s.d.Runner == nil {
		return failure("Среда запуска тестов недоступна.")
	}
	results, err := s.d.Runner.Run(ctx, sandbox.Batch{Dir: batch, IDs: []string{variant}})
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return failure("Превышено время выполнения тестов.")
	}
	if err != nil {
		return failure("Не удалось запустить тесты.")
	}
	for _, result := range results {
		if result.ID == variant {
			return result
		}
	}
	return failure("Среда запуска не вернула результат тестов.")
}
