// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileoffset

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// 使用包名作为 Scope Name 初始化 Meter
var meter = otel.Meter("seek_data")

var firstNonNullCounter, _ = meter.Int64Counter(
	"first_non_null_total",
	metric.WithDescription("Total number of first nonnull call"),
)

// FirstNonNUL 返回跳过前导 NUL（0x00）后的首个非零字节的物理偏移。
// 空文件或全为 NUL 的文件返回扫描到的文件末尾位置，不将 EOF 作为错误返回。
// 函数会恢复文件的原始读取位置；调用期间，调用方必须独占该文件句柄的偏移状态，
// 避免其他操作与内部 Seek 及位置恢复相互干扰。
func FirstNonNUL(file *os.File) (int64, error) {

	firstNonNullCounter.Add(context.Background(), 1)

	// 优先让文件系统定位数据区，避免逐字节读取大段稀疏空洞。
	offset, err := seekData(file)
	if err != nil {
		return 0, err
	}
	// 数据区仍可能包含 NUL；复用固定缓冲区，不随空洞长度分配内存。
	var buf [32 * 1024]byte
	for {
		// ReadAt 按物理偏移读取，不改变文件句柄的当前读取位置。
		n, readErr := file.ReadAt(buf[:], offset)
		prefixLen := n - len(bytes.TrimLeft(buf[:n], "\x00"))
		offset += int64(prefixLen)
		if prefixLen < n {
			// 找到首个非 NUL 后立即停止，后续内容中的 NUL 不属于前导空白。
			return offset, nil
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
