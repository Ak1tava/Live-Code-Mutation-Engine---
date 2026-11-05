package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
)

type config struct{ addr, publicURL, dataDir, workDir, mode, image, runnerBin string }

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func loadConfig() (config, error) {
	c := config{mode: env("LCME_RUNNER", "docker"), publicURL: env("LCME_PUBLIC_URL", "http://localhost:8080"), dataDir: env("LCME_DATA_DIR", "./data"), workDir: env("LCME_WORK_DIR", "./work"), image: env("LCME_RUNNER_IMAGE", "lcme-runner:latest"), runnerBin: env("LCME_RUNNER_BIN", "./bin/lcme-runner")}
	if c.mode != "docker" && c.mode != "local" {
		return c, fmt.Errorf("LCME_RUNNER должен быть docker или local")
	}
	fallback := ":8080"
	if c.mode == "local" {
		fallback = "127.0.0.1:8080"
	}
	c.addr = env("LCME_ADDR", fallback)
	host, _, err := net.SplitHostPort(c.addr)
	if err != nil {
		return c, fmt.Errorf("некорректный LCME_ADDR: %w", err)
	}
	if c.mode == "local" {
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return c, fmt.Errorf("local разрешён только на loopback: задайте LCME_ADDR=127.0.0.1:8080")
		}
	}
	u, err := url.Parse(c.publicURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return c, fmt.Errorf("LCME_PUBLIC_URL должен быть http(s) адресом без учётных данных")
	}
	return c, nil
}

// localOnly prevents a non-local Host from reaching a development runner.
func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			http.Error(w, "Локальное демо доступно только через localhost.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
