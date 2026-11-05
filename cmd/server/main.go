package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"lcme/internal/api"
	"lcme/internal/audit"
	"lcme/internal/hints"
	"lcme/internal/project"
	"lcme/internal/sandbox"
	"lcme/internal/selector"
	"lcme/web"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	c, err := loadConfig()
	if err != nil {
		return err
	}
	var runner sandbox.Runner
	if c.mode == "local" {
		log.Print("ВНИМАНИЕ: local без изоляции — только встроенный textkit на loopback. Не публикуйте этот сервер.")
		runner = sandbox.NewLocal(c.runnerBin)
	} else {
		runner = sandbox.NewDocker(c.image, 180*time.Second)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	probe, done := context.WithTimeout(ctx, 15*time.Second)
	version, err := runner.GoVersion(probe)
	done()
	if err != nil {
		return err
	}
	store, err := audit.OpenStore(c.dataDir)
	if err != nil {
		return err
	}
	static, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return err
	}
	hinter := hints.Template()
	if key, model := os.Getenv("OPENROUTER_API_KEY"), os.Getenv("LCME_LLM_MODEL"); key != "" && model != "" {
		hinter = hints.Fallback(hints.OpenRouter(key, model, "", nil), hinter)
	}
	server := api.New(api.Deps{
		Store: store, Importer: &project.Importer{ExamplesDir: "examples", WorkDir: c.workDir, MaxGo: strings.TrimPrefix(version, "go")}, Runner: runner,
		Prepare: func(ctx context.Context, p *project.Project, cb func(selector.Progress)) (*selector.Plan, error) {
			return selector.Prepare(ctx, p.Files, p.GoFiles(), runner, c.workDir, selector.DefaultOptions, cb)
		},
		Hinter: hinter, WorkDir: c.workDir, PublicURL: c.publicURL, Static: static, AllowGitImport: c.mode == "docker",
	})
	handler := server.Handler()
	if c.mode == "local" {
		handler = localOnly(handler)
	}
	httpServer := &http.Server{Addr: c.addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	errs := make(chan error, 1)
	go func() { log.Printf("LCME: http://%s (runner=%s)", c.addr, c.mode); errs <- httpServer.ListenAndServe() }()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case err := <-errs:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		case <-ticker.C:
			server.Tick(ctx)
		case <-ctx.Done():
			shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			return httpServer.Shutdown(shutdown)
		}
	}
}
