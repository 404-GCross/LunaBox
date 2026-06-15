package cloudsync

import (
	"context"
	"fmt"
	"sync"

	"lunabox/internal/service/cloudprovider"
	"lunabox/internal/service/cloudprovider/onedrive"
	"lunabox/internal/service/cloudprovider/s3"
)

// ConcurrencyFor 根据具体 provider 类型返回安全的并发上限。
// OneDrive Graph API 易被节流；S3 受网络/带宽限制可以放宽。
func ConcurrencyFor(provider cloudprovider.CloudStorageProvider) int {
	switch provider.(type) {
	case *onedrive.OneDriveProvider:
		return ConcurrencyOneDrive
	case *s3.S3Provider:
		return ConcurrencyS3
	default:
		return ConcurrencyOneDrive
	}
}

// runConcurrent 在受限并发下对 items 逐一调用 fn(ctx, item)。
// 任一任务返回 error → 立即取消其余任务并返回首个错误。
// 不引入额外依赖（标准库 sync + channel 实现 semaphore）。
func runConcurrent[T any](ctx context.Context, items []T, limit int, fn func(ctx context.Context, item T) error) error {
	if len(items) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 1
	}

	derivedCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, limit)
	errCh := make(chan error, len(items))
	var wg sync.WaitGroup

	for _, item := range items {
		// ctx 取消后停止派发更多任务
		if derivedCtx.Err() != nil {
			break
		}

		wg.Add(1)
		go func(it T) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-derivedCtx.Done():
				return
			}
			defer func() { <-sem }()

			if derivedCtx.Err() != nil {
				return
			}
			if err := fn(derivedCtx, it); err != nil {
				errCh <- err
				cancel()
			}
		}(item)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	if ctx.Err() != nil {
		return fmt.Errorf("sync canceled: %w", ctx.Err())
	}
	return nil
}
