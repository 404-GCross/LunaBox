package umbra

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"

	umbrsdk "github.com/Umbrae-Labs/umbra-sdk/umbra-go"
	"lunabox/internal/service/cloudprovider/batchupload"
)

const (
	umbraBatchSize            = 50
	umbraUploadPutConcurrency = 2
)

type preparedUpload struct {
	item        batchupload.Item
	input       umbrsdk.PresignUploadInput
	contentType string
	fileSize    uint64
	file        *os.File
}

// uploadBackupFiles uploads object backups through Umbra's batch presign and confirm APIs.
// Each batch is confirmed only after every corresponding object-storage PUT
// succeeds, so a partial PUT failure cannot advance that batch's metadata.
func (p *Provider) uploadBackupFiles(ctx context.Context, items []batchupload.Item) error {
	if len(items) == 0 {
		return nil
	}

	ordered := append([]batchupload.Item(nil), items...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].CloudPath < ordered[j].CloudPath })
	for start := 0; start < len(ordered); start += umbraBatchSize {
		end := min(start+umbraBatchSize, len(ordered))
		if err := p.uploadBatch(ctx, ordered[start:end]); err != nil {
			return fmt.Errorf("Umbra 批量上传第 %d-%d 项失败: %w", start+1, end, err)
		}
	}
	return nil
}

func (p *Provider) uploadBatch(ctx context.Context, items []batchupload.Item) error {
	prepared := make([]preparedUpload, 0, len(items))
	defer func() {
		for _, upload := range prepared {
			_ = upload.file.Close()
		}
	}()
	inputs := make([]umbrsdk.PresignUploadInput, 0, len(items))
	for _, item := range items {
		upload, err := p.prepareUpload(item)
		if err != nil {
			return err
		}
		prepared = append(prepared, upload)
		inputs = append(inputs, upload.input)
	}

	presigned, err := p.client.Backup.PresignUploadBatch(ctx, inputs)
	if err != nil {
		return fmt.Errorf("Umbra 批量预签名失败: %w", err)
	}
	if len(presigned) != len(prepared) {
		return fmt.Errorf("Umbra 批量预签名返回 %d 项，期望 %d 项", len(presigned), len(prepared))
	}
	for i, result := range presigned {
		if result.Error != nil {
			return fmt.Errorf("Umbra 批量预签名失败 (%s, %s): %s", prepared[i].item.CloudPath, result.Error.Code, result.Error.Message)
		}
		if result.BackupID == 0 || result.PresignedURL == "" {
			return fmt.Errorf("Umbra 批量预签名结果无效: %s", prepared[i].item.CloudPath)
		}
	}

	if err := p.putPresignedFiles(ctx, prepared, presigned); err != nil {
		return err
	}

	targets := make([]umbrsdk.BackupTarget, len(presigned))
	pathsByBackupID := make(map[uint64]string, len(presigned))
	for i, result := range presigned {
		targets[i] = umbrsdk.BackupTarget{BackupID: result.BackupID}
		pathsByBackupID[result.BackupID] = prepared[i].item.CloudPath
	}
	confirmed, err := p.client.Backup.ConfirmUploadBatch(ctx, targets)
	if err != nil {
		return fmt.Errorf("Umbra 批量确认上传失败: %w", err)
	}
	if confirmed == nil || len(confirmed.Items) != len(prepared) {
		count := 0
		if confirmed != nil {
			count = len(confirmed.Items)
		}
		return fmt.Errorf("Umbra 批量确认返回 %d 项，期望 %d 项", count, len(prepared))
	}
	for i, result := range confirmed.Items {
		if result.Error == nil {
			continue
		}
		cloudPath := pathsByBackupID[result.BackupID]
		if cloudPath == "" {
			cloudPath = prepared[i].item.CloudPath
		}
		return fmt.Errorf("Umbra 批量确认失败 (%s, %s): %s", cloudPath, result.Error.Code, result.Error.Message)
	}
	return nil
}

func (p *Provider) prepareUpload(item batchupload.Item) (preparedUpload, error) {
	if item.CloudPath == "" || item.LocalPath == "" {
		return preparedUpload{}, fmt.Errorf("Umbra 批量上传路径不能为空")
	}
	address, err := p.addressForCloudPath(item.CloudPath)
	if err != nil {
		return preparedUpload{}, fmt.Errorf("解析 Umbra 上传路径 %s 失败: %w", item.CloudPath, err)
	}
	file, err := os.Open(item.LocalPath)
	if err != nil {
		return preparedUpload{}, fmt.Errorf("打开 Umbra 上传文件 %s 失败: %w", item.CloudPath, err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return preparedUpload{}, fmt.Errorf("读取 Umbra 上传文件信息 %s 失败: %w", item.CloudPath, err)
	}
	if info.Size() <= 0 {
		file.Close()
		return preparedUpload{}, fmt.Errorf("Umbra 上传文件为空: %s", item.CloudPath)
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		file.Close()
		return preparedUpload{}, fmt.Errorf("计算 Umbra 上传文件哈希 %s 失败: %w", item.CloudPath, err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return preparedUpload{}, fmt.Errorf("重置 Umbra 上传文件 %s 失败: %w", item.CloudPath, err)
	}
	contentType := mime.TypeByExtension(filepath.Ext(item.LocalPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	fileSize := uint64(info.Size())
	return preparedUpload{
		item:        item,
		contentType: contentType,
		fileSize:    fileSize,
		file:        file,
		input: umbrsdk.PresignUploadInput{
			Address:     address,
			FileSize:    fileSize,
			ContentType: contentType,
			ContentHash: hex.EncodeToString(hasher.Sum(nil)),
		},
	}, nil
}

func (p *Provider) putPresignedFiles(ctx context.Context, prepared []preparedUpload, presigned []umbrsdk.BatchPresignResultItem) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan int)
	var firstErr error
	var once sync.Once
	var workers sync.WaitGroup
	workerCount := min(umbraUploadPutConcurrency, len(prepared))
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				if err := p.putPresignedFile(ctx, prepared[index], presigned[index].PresignedURL); err != nil {
					once.Do(func() {
						firstErr = err
						cancel()
					})
					return
				}
			}
		}()
	}
	for index := range prepared {
		select {
		case jobs <- index:
		case <-ctx.Done():
			break
		}
		if ctx.Err() != nil {
			break
		}
	}
	close(jobs)
	workers.Wait()
	if firstErr != nil {
		return firstErr
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("Umbra 对象存储上传已取消: %w", err)
	}
	return nil
}

func (p *Provider) putPresignedFile(ctx context.Context, upload preparedUpload, presignedURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignedURL, upload.file)
	if err != nil {
		return fmt.Errorf("创建 Umbra 对象存储请求 %s 失败: %w", upload.item.CloudPath, err)
	}
	req.Header.Set("Content-Type", upload.contentType)
	req.ContentLength = int64(upload.fileSize)
	res, err := p.client.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("Umbra 对象存储上传失败 %s: %w", upload.item.CloudPath, err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("Umbra 对象存储上传失败 %s: HTTP %d", upload.item.CloudPath, res.StatusCode)
	}
	return nil
}
