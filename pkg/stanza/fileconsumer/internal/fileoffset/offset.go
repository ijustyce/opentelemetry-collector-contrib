// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileoffset

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"

	"go.opentelemetry.io/otel/metric"
)

type Metrics struct {
	firstNonNullCounter metric.Int64Counter
	seekSuccess         metric.Int64Counter
	seekFailed          metric.Int64Counter
	seekError           metric.Int64Counter
	seekFull            metric.Int64Counter
	seekFallback        metric.Int64Counter
	seek0               metric.Int64Counter
}

func NewMetrics(meter metric.Meter) (*Metrics, error) {
	metrics := &Metrics{}
	var errs error
	var err error
	metrics.firstNonNullCounter, err = meter.Int64Counter(
		"first_non_null_total",
		metric.WithDescription("Total number of first nonnull call"),
	)
	errs = errors.Join(errs, err)
	metrics.seekSuccess, err = meter.Int64Counter(
		"first_non_null_success",
		metric.WithDescription("Total number of first nonnull success call"),
	)
	errs = errors.Join(errs, err)
	metrics.seekFailed, err = meter.Int64Counter(
		"first_non_null_failed",
		metric.WithDescription("Total number of first nonnull failed call"),
	)
	errs = errors.Join(errs, err)
	metrics.seekFull, err = meter.Int64Counter(
		"first_non_null_full",
		metric.WithDescription("Total number of first nonnull full call"),
	)
	errs = errors.Join(errs, err)
	metrics.seekError, err = meter.Int64Counter(
		"first_non_null_error",
		metric.WithDescription("Total number of first nonnull error call"),
	)
	errs = errors.Join(errs, err)
	metrics.seekFallback, err = meter.Int64Counter(
		"first_non_null_fallback",
		metric.WithDescription("Total number of first nonnull fallback call"),
	)
	errs = errors.Join(errs, err)
	metrics.seek0, err = meter.Int64Counter(
		"first_non_null_fallback0",
		metric.WithDescription("Total number of first nonnull fallback0 call"),
	)
	errs = errors.Join(errs, err)
	return metrics, errs
}

// FirstNonNUL 返回跳过前导 NUL（0x00）后的首个非零字节的物理偏移。
// 空文件或全为 NUL 的文件返回扫描到的文件末尾位置，不将 EOF 作为错误返回。
// 函数会恢复文件的原始读取位置；调用期间，调用方必须独占该文件句柄的偏移状态，
// 避免其他操作与内部 Seek 及位置恢复相互干扰。
func FirstNonNUL(file *os.File, metric *Metrics) (int64, error) {
	if metric != nil {
		metric.firstNonNullCounter.Add(context.Background(), 1)
	}

	// 优先让文件系统定位数据区，避免逐字节读取大段稀疏空洞。
	offset, err := seekData(file, metric)
	if err != nil {
		if metric != nil {
			metric.seekError.Add(context.Background(), 1)
		}
		return 0, err
	}
	fallback0 := offset == 0
	// 数据区仍可能包含 NUL；复用固定缓冲区，不随空洞长度分配内存。
	// 1 MiB 能显著减少连续 NUL 数据区的 ReadAt 次数，同时控制单次调用的内存占用。
	var buf [1 << 20]byte
	for {
		// ReadAt 按物理偏移读取，不改变文件句柄的当前读取位置。
		n, readErr := file.ReadAt(buf[:], offset)
		if nonNUL := bytes.IndexByte(buf[:n], 0); nonNUL >= 0 {
			// 找到首个非 NUL 后立即停止，后续内容中的 NUL 不属于前导空白。
			return offset + int64(nonNUL), nil
		}
		offset += int64(n)
		if metric != nil {
			metric.seekFallback.Add(context.Background(), 1)
			if fallback0 {
				fallback0 = false
				metric.seek0.Add(context.Background(), 1)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				// 本轮已读字节也全部为 NUL，offset 即扫描到的末尾位置。
				return offset, nil
			}
			return 0, readErr
		}
		if n == 0 {
			// 防止读取既没有返回数据也没有返回错误时陷入循环。
			return 0, io.ErrNoProgress
		}
	}
}
