//go:build linux

package processutils

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type processSnapshotEntry struct {
	Name       string
	PID        uint32
	ParentPID  uint32
	StartTicks uint64
}

// LinuxProcessSnapshot captures the lightweight process table once and lazily
// loads expensive per-process metadata only when a detector needs it.
type LinuxProcessSnapshot struct {
	entries  []processSnapshotEntry
	byPID    map[uint32]processSnapshotEntry
	children map[uint32][]uint32
	details  map[uint32]ProcessDetails
}

// LinuxProcessTracker retains process identities already observed in a game
// session, so reparenting does not remove them from the tracked membership.
type LinuxProcessTracker struct {
	rootPID        uint32
	rootStartTicks uint64
	rootObserved   bool
	known          map[uint32]uint64
}

// ProcessDetails exposes Linux-only process metadata used by launcher
// detection. It intentionally keeps ProcessInfo unchanged for the UI-facing
// process picker while allowing Linux detectors to inspect /proc paths.
type ProcessDetails struct {
	ProcessInfo
	ParentPID        uint32
	ExecutablePath   string
	CurrentDirectory string
	CommandLine      []string
}

func StartProcess(file string, args []string, dir string) (*StartedProcess, error) {
	return StartProcessWithEnv(file, args, dir, nil)
}

func StartProcessWithEnv(file string, args []string, dir string, env []string) (*StartedProcess, error) {
	file = strings.TrimSpace(file)
	if file == "" {
		return nil, fmt.Errorf("executable path is empty")
	}

	cmd := exec.Command(file, args...)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	pid := uint32(cmd.Process.Pid)
	exitChan := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exitChan)
	}()

	return &StartedProcess{PID: pid, ExitChan: exitChan}, nil
}

func StartProcessElevated(file string, args []string, dir string) (*StartedProcess, error) {
	return StartProcess(file, args, dir)
}

func StartProcessHidden(file string, args []string, dir string) (*StartedProcess, error) {
	return StartProcess(file, args, dir)
}

func StartProcessElevatedHidden(file string, args []string, dir string) (*StartedProcess, error) {
	return StartProcessElevated(file, args, dir)
}

func CloseProcessHandle(processHandle uintptr) error {
	return nil
}

func CheckIfProcessRunning(processName string) (bool, error) {
	_, err := GetProcessPIDByName(processName)
	if err != nil {
		if strings.Contains(err.Error(), "process not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func GetRunningProcesses() ([]ProcessInfo, error) {
	snapshot, err := CaptureLinuxProcessSnapshot()
	if err != nil {
		return nil, err
	}

	systemProcesses := map[string]bool{
		"bash": true, "cat": true, "dbus-daemon": true, "gnome-shell": true,
		"init": true, "kthreadd": true, "login": true, "sh": true,
		"sshd": true, "steam": true, "steamwebhelper": true, "systemd": true,
		"systemd-journal": true, "systemd-logind": true, "xwayland": true,
	}

	processMap := make(map[string]ProcessInfo)
	for _, entry := range snapshot.entries {
		nameLower := strings.ToLower(strings.TrimSpace(entry.Name))
		if nameLower == "" || systemProcesses[nameLower] || entry.PID == 0 {
			continue
		}
		if _, exists := processMap[nameLower]; !exists {
			processMap[nameLower] = ProcessInfo{Name: entry.Name, PID: entry.PID}
		}
	}

	processes := make([]ProcessInfo, 0, len(processMap))
	for _, proc := range processMap {
		processes = append(processes, proc)
	}
	sortProcesses(processes)
	return processes, nil
}

func GetProcessPIDByName(processName string) (uint32, error) {
	targetName := strings.TrimSpace(processName)
	if targetName == "" {
		return 0, fmt.Errorf("process name is empty")
	}

	snapshot, err := CaptureLinuxProcessSnapshot()
	if err != nil {
		return 0, err
	}
	for _, entry := range snapshot.entries {
		if strings.EqualFold(entry.Name, targetName) {
			return entry.PID, nil
		}
	}
	return 0, fmt.Errorf("process not found: %s", processName)
}

func IsProcessPresentByPID(pid uint32) bool {
	if pid == 0 {
		return false
	}
	if isZombieProcess(pid) {
		return false
	}
	err := syscall.Kill(int(pid), 0)
	return err == nil || err == syscall.EPERM
}

func isZombieProcess(pid uint32) bool {
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "stat"))
	if err != nil {
		return false
	}
	_, _, fields, ok := parseProcStat(string(stat))
	return ok && len(fields) > 0 && fields[0] == "Z"
}

func GetDescendantProcesses(parentPID uint32) ([]ProcessInfo, error) {
	snapshot, err := CaptureLinuxProcessSnapshot()
	if err != nil {
		return nil, err
	}
	return snapshot.DescendantProcesses(parentPID), nil
}

func GetDescendantProcessDetails(parentPID uint32) ([]ProcessDetails, error) {
	snapshot, err := CaptureLinuxProcessSnapshot()
	if err != nil {
		return nil, err
	}
	return snapshot.DescendantProcessDetails(parentPID), nil
}

// CaptureLinuxProcessSnapshot reads /proc/*/stat once and builds indexes used
// by descendant and directory queries in the same detection cycle.
func CaptureLinuxProcessSnapshot() (*LinuxProcessSnapshot, error) {
	entries, err := readProcessSnapshotEntries()
	if err != nil {
		return nil, err
	}
	return newLinuxProcessSnapshot(entries), nil
}

func newLinuxProcessSnapshot(entries []processSnapshotEntry) *LinuxProcessSnapshot {
	snapshot := &LinuxProcessSnapshot{
		entries:  entries,
		byPID:    make(map[uint32]processSnapshotEntry, len(entries)),
		children: make(map[uint32][]uint32),
		details:  make(map[uint32]ProcessDetails),
	}
	for _, entry := range entries {
		snapshot.byPID[entry.PID] = entry
		snapshot.children[entry.ParentPID] = append(snapshot.children[entry.ParentPID], entry.PID)
	}
	return snapshot
}

func (s *LinuxProcessSnapshot) ProcessPIDByName(processName string) (uint32, bool) {
	if s == nil {
		return 0, false
	}
	targetName := strings.TrimSpace(processName)
	for _, entry := range s.entries {
		if strings.EqualFold(entry.Name, targetName) {
			return entry.PID, true
		}
	}
	return 0, false
}

func (s *LinuxProcessSnapshot) ContainsPID(pid uint32) bool {
	if s == nil || pid == 0 {
		return false
	}
	_, ok := s.byPID[pid]
	return ok
}

func NewLinuxProcessTracker(rootPID uint32) *LinuxProcessTracker {
	return &LinuxProcessTracker{
		rootPID: rootPID,
		known:   make(map[uint32]uint64),
	}
}

func (t *LinuxProcessTracker) Observe(snapshot *LinuxProcessSnapshot) []ProcessInfo {
	if t == nil || snapshot == nil {
		return nil
	}

	alive := make(map[uint32]uint64, len(t.known)+1)
	for pid, startTicks := range t.known {
		if entry, ok := snapshot.byPID[pid]; ok && entry.StartTicks == startTicks {
			alive[pid] = startTicks
		}
	}
	if root, ok := snapshot.byPID[t.rootPID]; ok {
		if !t.rootObserved {
			t.rootObserved = true
			t.rootStartTicks = root.StartTicks
		}
		if root.StartTicks == t.rootStartTicks {
			alive[root.PID] = root.StartTicks
		}
	}

	queue := make([]uint32, 0, len(alive))
	for pid := range alive {
		queue = append(queue, pid)
	}
	seen := make(map[uint32]bool, len(alive))
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		if seen[pid] {
			continue
		}
		seen[pid] = true
		for _, childPID := range snapshot.children[pid] {
			entry, ok := snapshot.byPID[childPID]
			if !ok {
				continue
			}
			alive[childPID] = entry.StartTicks
			queue = append(queue, childPID)
		}
	}

	t.known = alive
	return snapshot.processesForIdentities(alive)
}

func (t *LinuxProcessTracker) Remember(snapshot *LinuxProcessSnapshot, processes []ProcessInfo) {
	if t == nil || snapshot == nil {
		return
	}
	for _, process := range processes {
		if entry, ok := snapshot.byPID[process.PID]; ok {
			t.known[process.PID] = entry.StartTicks
		}
	}
}

func (t *LinuxProcessTracker) RootPresent(snapshot *LinuxProcessSnapshot) bool {
	if t == nil || snapshot == nil || !t.rootObserved {
		return false
	}
	entry, ok := snapshot.byPID[t.rootPID]
	return ok && entry.StartTicks == t.rootStartTicks
}

func (s *LinuxProcessSnapshot) processesForIdentities(identities map[uint32]uint64) []ProcessInfo {
	processes := make([]ProcessInfo, 0, len(identities))
	for pid, startTicks := range identities {
		entry, ok := s.byPID[pid]
		if !ok || entry.StartTicks != startTicks {
			continue
		}
		processes = append(processes, ProcessInfo{Name: entry.Name, PID: entry.PID})
	}
	sortProcesses(processes)
	return processes
}

func (s *LinuxProcessSnapshot) DescendantProcesses(parentPID uint32) []ProcessInfo {
	details := s.DescendantProcessDetails(parentPID)
	processes := make([]ProcessInfo, 0, len(details))
	for _, detail := range details {
		processes = append(processes, detail.ProcessInfo)
	}
	return processes
}

func (s *LinuxProcessSnapshot) DescendantProcessDetails(parentPID uint32) []ProcessDetails {
	if s == nil || parentPID == 0 {
		return nil
	}

	seen := map[uint32]bool{parentPID: true}
	queue := []uint32{parentPID}
	descendants := make([]ProcessDetails, 0)

	for len(queue) > 0 {
		currentPID := queue[0]
		queue = queue[1:]

		for _, childPID := range s.children[currentPID] {
			if seen[childPID] {
				continue
			}
			seen[childPID] = true
			queue = append(queue, childPID)
			if detail, ok := s.processDetails(childPID); ok {
				descendants = append(descendants, detail)
			}
		}
	}

	sortProcessDetails(descendants)
	return descendants
}

// IsProcessDescendant walks from the candidate towards PID 1. This avoids a
// full process-table scan for the once-per-second foreground check.
func IsProcessDescendant(rootPID uint32, candidatePID uint32) bool {
	if rootPID == 0 || candidatePID == 0 {
		return false
	}

	seen := make(map[uint32]bool)
	currentPID := candidatePID
	for currentPID != 0 && !seen[currentPID] {
		if currentPID == rootPID {
			return true
		}
		seen[currentPID] = true

		stat, err := os.ReadFile(filepath.Join("/proc", strconv.FormatUint(uint64(currentPID), 10), "stat"))
		if err != nil {
			return false
		}
		_, parentPID, fields, ok := parseProcStat(string(stat))
		if !ok || len(fields) == 0 || fields[0] == "Z" || parentPID == currentPID {
			return false
		}
		currentPID = parentPID
	}
	return false
}

func GetProcessCreationTime(pid uint32) (time.Time, error) {
	if pid == 0 {
		return time.Time{}, fmt.Errorf("invalid pid: 0")
	}

	stat, err := os.ReadFile(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "stat"))
	if err != nil {
		return time.Time{}, fmt.Errorf("read process stat: %w", err)
	}

	_, _, fields, ok := parseProcStat(string(stat))
	if !ok || len(fields) < 20 {
		return time.Time{}, fmt.Errorf("parse process stat for pid %d", pid)
	}

	startTicks, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse process start time: %w", err)
	}
	bootTime, err := linuxBootTime()
	if err != nil {
		return time.Time{}, err
	}
	return bootTime.Add(time.Duration(startTicks) * time.Second / 100), nil
}

func GetProcessesByExecutableDir(rootDir string) ([]ProcessInfo, error) {
	details, err := GetProcessDetailsByExecutableDir(rootDir)
	if err != nil {
		return nil, err
	}

	processes := make([]ProcessInfo, 0, len(details))
	for _, detail := range details {
		processes = append(processes, detail.ProcessInfo)
	}
	return processes, nil
}

func GetProcessDetailsByExecutableDir(rootDir string) ([]ProcessDetails, error) {
	if strings.TrimSpace(rootDir) == "" {
		return nil, nil
	}
	normalizedRoot, err := filepath.Abs(filepath.Clean(strings.TrimSpace(rootDir)))
	if err != nil {
		return nil, fmt.Errorf("normalize executable dir: %w", err)
	}

	snapshot, err := CaptureLinuxProcessSnapshot()
	if err != nil {
		return nil, err
	}
	return snapshot.ProcessDetailsByExecutableDir(normalizedRoot)
}

func (s *LinuxProcessSnapshot) ProcessesByExecutableDir(rootDir string) ([]ProcessInfo, error) {
	details, err := s.ProcessDetailsByExecutableDir(rootDir)
	if err != nil {
		return nil, err
	}
	processes := make([]ProcessInfo, 0, len(details))
	for _, detail := range details {
		processes = append(processes, detail.ProcessInfo)
	}
	return processes, nil
}

func (s *LinuxProcessSnapshot) ProcessDetailsByExecutableDir(rootDir string) ([]ProcessDetails, error) {
	if s == nil || strings.TrimSpace(rootDir) == "" {
		return nil, nil
	}
	canonicalRoot, err := canonicalizeLinuxPath(rootDir)
	if err != nil {
		return nil, fmt.Errorf("normalize executable dir: %w", err)
	}

	processes := make([]ProcessDetails, 0)
	for _, entry := range s.entries {
		detail, ok := s.processDetails(entry.PID)
		if !ok || !processDetailsMatchesRoot(detail, canonicalRoot) {
			continue
		}
		processes = append(processes, detail)
	}

	sortProcessDetails(processes)
	return processes, nil
}

func (s *LinuxProcessSnapshot) processDetails(pid uint32) (ProcessDetails, bool) {
	if detail, ok := s.details[pid]; ok {
		return detail, true
	}
	entry, ok := s.byPID[pid]
	if !ok {
		return ProcessDetails{}, false
	}
	detail := processDetailsFromEntry(entry)
	s.details[pid] = detail
	return detail, true
}

func FilterProcessesWithVisibleWindows(processes []ProcessInfo) []ProcessInfo {
	return processes
}

func HasVisibleTopLevelWindow(pid uint32) bool {
	return IsProcessPresentByPID(pid)
}

func IsProcessRunningByPID(pid uint32, ctx context.Context) bool {
	return IsProcessPresentByPID(pid)
}

type ProcessMonitor struct {
	mu       sync.Mutex
	stopOnce sync.Once
	exitChan chan struct{}
	pidfd    int
	wakeFD   int
	startErr error
}

type SnapshotProcessMonitor struct {
	stopOnce sync.Once
	stopChan chan struct{}
	exitChan chan struct{}
}

func NewProcessMonitor(pid uint32) *ProcessMonitor {
	monitor := &ProcessMonitor{
		exitChan: make(chan struct{}),
		pidfd:    -1,
		wakeFD:   -1,
	}
	if pid == 0 {
		monitor.startErr = fmt.Errorf("process id is zero")
		return monitor
	}

	pidfd, err := unix.PidfdOpen(int(pid), 0)
	if err != nil {
		if errors.Is(err, unix.ESRCH) {
			close(monitor.exitChan)
			return monitor
		}
		monitor.startErr = fmt.Errorf("open pidfd for process %d: %w", pid, err)
		return monitor
	}
	monitor.pidfd = pidfd

	wakeFD, err := unix.Eventfd(0, unix.EFD_CLOEXEC|unix.EFD_NONBLOCK)
	if err != nil {
		_ = unix.Close(pidfd)
		monitor.pidfd = -1
		monitor.startErr = fmt.Errorf("create process monitor wake event: %w", err)
		return monitor
	}
	monitor.wakeFD = wakeFD

	go monitor.wait()
	return monitor
}

func (pm *ProcessMonitor) wait() {
	defer close(pm.exitChan)
	defer pm.closeDescriptors()

	fds := []unix.PollFd{
		{Fd: int32(pm.pidfd), Events: unix.POLLIN},
		{Fd: int32(pm.wakeFD), Events: unix.POLLIN},
	}

	for {
		_, err := unix.Poll(fds, -1)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil || fds[0].Revents != 0 || fds[1].Revents != 0 {
			return
		}
	}
}

func (pm *ProcessMonitor) Start() (<-chan struct{}, error) {
	if pm == nil {
		return nil, fmt.Errorf("process monitor is nil")
	}
	if pm.startErr != nil {
		return nil, pm.startErr
	}
	return pm.exitChan, nil
}

func (pm *ProcessMonitor) Stop() {
	if pm == nil {
		return
	}
	pm.stopOnce.Do(func() {
		pm.mu.Lock()
		defer pm.mu.Unlock()
		if pm.wakeFD >= 0 {
			_, _ = unix.Write(pm.wakeFD, []byte{1, 0, 0, 0, 0, 0, 0, 0})
		}
	})
}

func (pm *ProcessMonitor) closeDescriptors() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.pidfd >= 0 {
		_ = unix.Close(pm.pidfd)
		pm.pidfd = -1
	}
	if pm.wakeFD >= 0 {
		_ = unix.Close(pm.wakeFD)
		pm.wakeFD = -1
	}
}

func (pm *ProcessMonitor) WaitForProcessExit(timeout time.Duration) bool {
	exitChan, err := pm.Start()
	if err != nil {
		return true
	}
	if timeout == 0 {
		<-exitChan
		return true
	}
	select {
	case <-exitChan:
		return true
	case <-time.After(timeout):
		pm.Stop()
		return false
	}
}

func WaitForProcessExitAsync(pid uint32) (*ProcessMonitor, <-chan struct{}, error) {
	monitor := NewProcessMonitor(pid)
	exitChan, err := monitor.Start()
	if err != nil {
		return nil, nil, err
	}
	return monitor, exitChan, nil
}

func WaitForProcessHandleExitAsync(pid uint32, processHandle uintptr) (*ProcessMonitor, <-chan struct{}, error) {
	return WaitForProcessExitAsync(pid)
}

func NewSnapshotProcessMonitor(pid uint32) *SnapshotProcessMonitor {
	monitor := &SnapshotProcessMonitor{
		stopChan: make(chan struct{}),
		exitChan: make(chan struct{}),
	}
	go func() {
		defer close(monitor.exitChan)

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if !IsProcessPresentByPID(pid) {
					return
				}
			case <-monitor.stopChan:
				return
			}
		}
	}()
	return monitor
}

func (m *SnapshotProcessMonitor) Stop() {
	if m == nil {
		return
	}
	m.stopOnce.Do(func() {
		close(m.stopChan)
	})
}

func (m *SnapshotProcessMonitor) ExitChan() <-chan struct{} {
	if m == nil {
		exitChan := make(chan struct{})
		close(exitChan)
		return exitChan
	}
	return m.exitChan
}

func WaitForProcessExitBySnapshotAsync(pid uint32) (*SnapshotProcessMonitor, <-chan struct{}) {
	monitor := NewSnapshotProcessMonitor(pid)
	return monitor, monitor.ExitChan()
}

func getProcessSnapshotEntries() ([]processSnapshotEntry, error) {
	snapshot, err := CaptureLinuxProcessSnapshot()
	if err != nil {
		return nil, err
	}
	return snapshot.entries, nil
}

func readProcessSnapshotEntries() ([]processSnapshotEntry, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("enumerate processes: %w", err)
	}

	processes := make([]processSnapshotEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid64, err := strconv.ParseUint(entry.Name(), 10, 32)
		if err != nil || pid64 == 0 {
			continue
		}

		statPath := filepath.Join("/proc", entry.Name(), "stat")
		stat, err := os.ReadFile(statPath)
		if err != nil {
			continue
		}
		name, parentPID, fields, ok := parseProcStat(string(stat))
		if !ok {
			continue
		}
		if len(fields) > 0 && fields[0] == "Z" {
			continue
		}
		if name == "" {
			if imagePath, ok := queryProcessImagePath(uint32(pid64)); ok {
				name = filepath.Base(imagePath)
			}
		}
		if name == "" {
			continue
		}

		startTicks := uint64(0)
		if len(fields) > 19 {
			startTicks, _ = strconv.ParseUint(fields[19], 10, 64)
		}
		processes = append(processes, processSnapshotEntry{
			Name:       name,
			PID:        uint32(pid64),
			ParentPID:  parentPID,
			StartTicks: startTicks,
		})
	}
	return processes, nil
}

func parseProcStat(stat string) (string, uint32, []string, bool) {
	stat = strings.TrimSpace(stat)
	open := strings.Index(stat, "(")
	close := strings.LastIndex(stat, ")")
	if open < 0 || close <= open || close+2 >= len(stat) {
		return "", 0, nil, false
	}

	name := strings.TrimSpace(stat[open+1 : close])
	fields := strings.Fields(strings.TrimSpace(stat[close+1:]))
	if len(fields) < 2 {
		return "", 0, nil, false
	}
	ppid, err := strconv.ParseUint(fields[1], 10, 32)
	if err != nil {
		return "", 0, nil, false
	}
	return name, uint32(ppid), fields, true
}

func queryProcessImagePath(pid uint32) (string, bool) {
	path, err := os.Readlink(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "exe"))
	if err != nil || strings.TrimSpace(path) == "" {
		return "", false
	}
	return strings.TrimSuffix(path, " (deleted)"), true
}

func queryProcessCurrentDirectory(pid uint32) string {
	path, err := os.Readlink(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "cwd"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(path)
}

func readProcessCommandLine(pid uint32) []string {
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "cmdline"))
	if err != nil || len(raw) == 0 {
		return nil
	}
	parts := strings.Split(string(raw), "\x00")
	args := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			args = append(args, part)
		}
	}
	return args
}

func GetProcessCommandInfo(pid uint32) (ProcessCommandInfo, error) {
	if pid == 0 {
		return ProcessCommandInfo{}, fmt.Errorf("process id is zero")
	}

	info := ProcessCommandInfo{
		Arguments:   readProcessCommandLine(pid),
		Environment: readProcessEnvironment(pid),
	}
	if imagePath, ok := queryProcessImagePath(pid); ok {
		info.ExecutablePath = imagePath
	}
	if len(info.Arguments) == 0 && info.ExecutablePath == "" {
		return ProcessCommandInfo{}, fmt.Errorf("read process command info for pid %d", pid)
	}
	return info, nil
}

func readProcessEnvironment(pid uint32) map[string]string {
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.FormatUint(uint64(pid), 10), "environ"))
	if err != nil || len(raw) == 0 {
		return nil
	}
	environment := make(map[string]string)
	for _, part := range strings.Split(string(raw), "\x00") {
		key, value, ok := strings.Cut(part, "=")
		if ok && key != "" {
			environment[key] = value
		}
	}
	return environment
}

func processDetailsFromEntry(entry processSnapshotEntry) ProcessDetails {
	detail := ProcessDetails{
		ProcessInfo: ProcessInfo{
			Name: entry.Name,
			PID:  entry.PID,
		},
		ParentPID:        entry.ParentPID,
		CurrentDirectory: queryProcessCurrentDirectory(entry.PID),
		CommandLine:      readProcessCommandLine(entry.PID),
	}
	if imagePath, ok := queryProcessImagePath(entry.PID); ok {
		detail.ExecutablePath = imagePath
	}
	return detail
}

func processMatchesRoot(pid uint32, rootDir string) bool {
	canonicalRoot, err := canonicalizeLinuxPath(rootDir)
	if err != nil {
		return false
	}
	if imagePath, ok := queryProcessImagePath(pid); ok && isProcPathUnderCanonicalDir(imagePath, canonicalRoot) {
		return true
	}

	pidText := strconv.FormatUint(uint64(pid), 10)
	if cwdPath, err := os.Readlink(filepath.Join("/proc", pidText, "cwd")); err == nil && isProcPathUnderCanonicalDir(cwdPath, canonicalRoot) {
		return true
	}

	cmdline, err := os.ReadFile(filepath.Join("/proc", pidText, "cmdline"))
	if err != nil || len(cmdline) == 0 {
		return false
	}
	for _, raw := range strings.Split(string(cmdline), "\x00") {
		if processArgMatchesRoot(raw, canonicalRoot) {
			return true
		}
	}
	return false
}

func processDetailsMatchesRoot(detail ProcessDetails, canonicalRoot string) bool {
	if strings.TrimSpace(canonicalRoot) == "" {
		return false
	}
	if detail.ExecutablePath != "" && isProcPathUnderCanonicalDir(detail.ExecutablePath, canonicalRoot) {
		return true
	}
	if detail.CurrentDirectory != "" && isProcPathUnderCanonicalDir(detail.CurrentDirectory, canonicalRoot) {
		return true
	}
	for _, arg := range detail.CommandLine {
		if processArgMatchesRoot(arg, canonicalRoot) {
			return true
		}
	}
	return false
}

func processArgMatchesRoot(arg string, canonicalRoot string) bool {
	arg = strings.Trim(strings.TrimSpace(arg), "\"'")
	if arg == "" {
		return false
	}
	if filepath.IsAbs(arg) {
		return isPathUnderCanonicalDir(arg, canonicalRoot)
	}
	return strings.Contains(filepath.ToSlash(arg), filepath.ToSlash(canonicalRoot))
}

func canonicalizeLinuxPath(path string) (string, error) {
	absPath, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return filepath.Clean(resolvedPath), nil
	}
	return absPath, nil
}

func isPathUnderCanonicalDir(path string, canonicalRoot string) bool {
	canonicalPath, err := canonicalizeLinuxPath(path)
	if err != nil {
		return false
	}
	return canonicalPathUnderDir(canonicalPath, canonicalRoot)
}

// Paths returned by /proc/<pid>/{exe,cwd} are already kernel-resolved, so only
// normalize them lexically after canonicalizing the configured root once.
func isProcPathUnderCanonicalDir(path string, canonicalRoot string) bool {
	absPath, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
	if err != nil {
		return false
	}
	return canonicalPathUnderDir(absPath, canonicalRoot)
}

func canonicalPathUnderDir(canonicalPath string, canonicalRoot string) bool {
	rel, err := filepath.Rel(canonicalRoot, canonicalPath)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func linuxBootTime() (time.Time, error) {
	stat, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Time{}, fmt.Errorf("read /proc/stat: %w", err)
	}
	for _, line := range strings.Split(string(stat), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "btime" {
			seconds, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return time.Time{}, fmt.Errorf("parse boot time: %w", err)
			}
			return time.Unix(seconds, 0), nil
		}
	}
	return time.Time{}, fmt.Errorf("boot time not found")
}

func sortProcesses(processes []ProcessInfo) {
	sort.Slice(processes, func(i, j int) bool {
		left := strings.ToLower(processes[i].Name)
		right := strings.ToLower(processes[j].Name)
		if left == right {
			return processes[i].PID < processes[j].PID
		}
		return left < right
	})
}

func sortProcessDetails(processes []ProcessDetails) {
	sort.Slice(processes, func(i, j int) bool {
		left := strings.ToLower(processes[i].Name)
		right := strings.ToLower(processes[j].Name)
		if left == right {
			return processes[i].PID < processes[j].PID
		}
		return left < right
	})
}
