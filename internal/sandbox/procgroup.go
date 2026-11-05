package sandbox

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// SecretEnvPrefixes and SecretEnvNames are removed from the environment
// handed to sandboxed processes.
var secretEnvNames = map[string]bool{"OPENROUTER_API_KEY": true}

const secretEnvPrefix = "LCME_"

// FilterEnv drops the server's secrets (OPENROUTER_API_KEY and every LCME_*
// variable) from env and appends the forced Go settings.
func FilterEnv(env []string) []string {
	out := make([]string, 0, len(env)+len(Env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if secretEnvNames[name] || strings.HasPrefix(name, secretEnvPrefix) {
			continue
		}
		out = append(out, kv)
	}
	return append(out, Env...)
}

// SandboxEnv is FilterEnv applied to the current process environment.
func SandboxEnv() []string { return FilterEnv(os.Environ()) }

// SetProcessGroup runs cmd in its own process group and makes cancellation
// kill the whole group. It needs no capabilities.
func SetProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		KillGroup(cmd)
		return nil
	}
	cmd.WaitDelay = time.Second
}

// KillGroup SIGKILLs every process left in cmd's process group.
func KillGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
