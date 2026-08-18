//go:build linux

package processutils

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestParseProcStatReturnsStateField(t *testing.T) {
	name, parentPID, fields, ok := parseProcStat("1234 (Game.exe) Z 1000 1234 1234 0 -1 4194304 1 2 3 4 5 6 7 8 20 0 1 0 123456")
	if !ok {
		t.Fatal("expected proc stat to parse")
	}
	if name != "Game.exe" {
		t.Fatalf("expected process name, got %q", name)
	}
	if parentPID != 1000 {
		t.Fatalf("expected parent PID 1000, got %d", parentPID)
	}
	if len(fields) == 0 || fields[0] != "Z" {
		t.Fatalf("expected zombie state in first field, got %#v", fields)
	}
}

func TestLinuxProcessSnapshotIndexesDescendants(t *testing.T) {
	snapshot := newLinuxProcessSnapshot([]processSnapshotEntry{
		{Name: "launcher", PID: 100, ParentPID: 1, StartTicks: 10},
		{Name: "wrapper", PID: 200, ParentPID: 100, StartTicks: 20},
		{Name: "game", PID: 300, ParentPID: 200, StartTicks: 30},
		{Name: "unrelated", PID: 400, ParentPID: 1, StartTicks: 40},
	})

	descendants := snapshot.DescendantProcesses(100)
	if len(descendants) != 2 || descendants[0].PID != 300 || descendants[1].PID != 200 {
		t.Fatalf("unexpected descendants: %#v", descendants)
	}
	if !snapshot.ContainsPID(300) || snapshot.ContainsPID(999) {
		t.Fatal("unexpected snapshot PID membership")
	}
	if pid, ok := snapshot.ProcessPIDByName("GAME"); !ok || pid != 300 {
		t.Fatalf("unexpected name lookup: pid=%d ok=%v", pid, ok)
	}
}

func TestIsProcessDescendantWalksCurrentProcessParentChain(t *testing.T) {
	currentPID := uint32(os.Getpid())
	parentPID := uint32(os.Getppid())
	if parentPID == 0 {
		t.Skip("current process has no observable parent")
	}
	if !IsProcessDescendant(parentPID, currentPID) {
		t.Fatalf("expected PID %d to descend from %d", currentPID, parentPID)
	}
	if IsProcessDescendant(currentPID, parentPID) {
		t.Fatalf("did not expect PID %d to descend from %d", parentPID, currentPID)
	}
}

func TestLinuxProcessTrackerRetainsReparentedProcessAndRejectsPIDReuse(t *testing.T) {
	tracker := NewLinuxProcessTracker(100)
	initial := newLinuxProcessSnapshot([]processSnapshotEntry{
		{Name: "launcher", PID: 100, ParentPID: 1, StartTicks: 10},
		{Name: "game", PID: 200, ParentPID: 100, StartTicks: 20},
	})
	if processes := tracker.Observe(initial); len(processes) != 2 {
		t.Fatalf("expected root and child, got %#v", processes)
	}

	reparented := newLinuxProcessSnapshot([]processSnapshotEntry{
		{Name: "game", PID: 200, ParentPID: 1, StartTicks: 20},
	})
	processes := tracker.Observe(reparented)
	if len(processes) != 1 || processes[0].PID != 200 {
		t.Fatalf("expected reparented child to remain tracked, got %#v", processes)
	}

	reused := newLinuxProcessSnapshot([]processSnapshotEntry{
		{Name: "unrelated", PID: 200, ParentPID: 1, StartTicks: 99},
	})
	if processes := tracker.Observe(reused); len(processes) != 0 {
		t.Fatalf("expected reused PID to be rejected, got %#v", processes)
	}
}

func TestProcessMonitorSignalsProcessExitThroughPIDFD(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "exec sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start test process: %v", err)
	}
	reaped := false
	t.Cleanup(func() {
		if !reaped {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	monitor, exitChan, err := WaitForProcessExitAsync(uint32(cmd.Process.Pid))
	if err != nil {
		t.Skipf("pidfd process monitoring is unavailable: %v", err)
	}
	defer monitor.Stop()

	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill test process: %v", err)
	}
	_ = cmd.Wait()
	reaped = true

	select {
	case <-exitChan:
	case <-time.After(2 * time.Second):
		t.Fatal("pidfd monitor did not signal process exit")
	}
}

func TestProcessMonitorStopWakesPIDFDPoll(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "exec sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start test process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	monitor, exitChan, err := WaitForProcessExitAsync(uint32(cmd.Process.Pid))
	if err != nil {
		t.Skipf("pidfd process monitoring is unavailable: %v", err)
	}
	monitor.Stop()

	select {
	case <-exitChan:
	case <-time.After(2 * time.Second):
		t.Fatal("stopping process monitor did not wake pidfd poll")
	}
}
