//go:build darwin

package main

func (s *lifecycleState) StartTray() {
	s.MarkTrayReady()
}

func (s *lifecycleState) RequestTrayQuit() {
	s.trayQuitOnce.Do(func() {
		s.MarkTrayExit()
	})
}
