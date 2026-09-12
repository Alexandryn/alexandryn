//go:build linux

package supervisor

import (
	"os/exec"
	"syscall"
)

// SpawnWithOrphanPrevention starts cmd with SysProcAttr.Pdeathsig set to
// SIGKILL: if this process dies for any reason — including SIGKILL, which
// bypasses every graceful-shutdown path — the kernel signals cmd to die too,
// with no cooperation needed from cmd's own source. This requires no modification
// of PostgreSQL's own binary, ensuring orphan prevention for third-party processes.
func SpawnWithOrphanPrevention(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
	return cmd.Start()
}
