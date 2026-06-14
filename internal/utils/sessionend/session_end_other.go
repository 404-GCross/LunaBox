//go:build !windows

package sessionend

type Options struct {
	Reason            string
	OnQueryEndSession func()
}

type Hook struct{}

func Start(options Options) (*Hook, error) {
	return &Hook{}, nil
}

func (h *Hook) Stop() error {
	return nil
}

func (h *Hook) ReleaseShutdownBlockReason() {}
