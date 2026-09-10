// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package fileoffset

import (
	"context"
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// seekData 返回文件系统报告的首个数据区的物理偏移，并恢复原始文件位置。
// 数据区仍可能包含零字节，调用方需要继续检查实际内容。
// 不支持 SEEK_DATA 时返回 0 供调用方顺序扫描；没有数据区时返回查询前的文件大小。
// 调用期间不能有其他操作并发修改同一文件句柄的偏移。
func seekData(file *os.File, metrics *Metrics) (offset int64, err error) {
	// SEEK_DATA 会改变文件位置，因此先保存位置，并在所有后续返回路径中恢复。
	position, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}
	defer func() {
		_, restoreErr := file.Seek(position, io.SeekStart)
		// 恢复失败也必须上报，不能让调用方误以为文件位置保持不变。
		err = errors.Join(err, restoreErr)
	}()
	// 从文件头查询数据区，跳过文件系统能够识别的前导空洞。
	offset, err = file.Seek(0, unix.SEEK_DATA)
	switch {
	case errors.Is(err, unix.ENXIO):
		// ENXIO：查询位置已到达文件末尾，或其后没有数据区（如空文件、全空洞文件）。
		// 使用当前文件大小作为 offset 返回，标明该范围后无有效数据。
		if metrics != nil {
			metrics.seekFull.Add(context.Background(), 1)
		}
		info, err2 := file.Stat()
		if err2 != nil {
			return 0, err2
		}
		return info.Size(), nil
	case errors.Is(err, unix.EINVAL), errors.Is(err, unix.ENOTSUP), errors.Is(err, unix.ENOSYS):
		// EINVAL：在此查询中通常表示不识别或不支持 SEEK_DATA 这一定位方式。
		// ENOTSUP：文件系统或文件不支持该操作；ENOSYS：系统未实现该操作。
		// 将这些情况视为不支持空洞定位，返回文件头位置，回退到有界内存的顺序扫描。
		if metrics != nil {
			metrics.seekFailed.Add(context.Background(), 1)
		}
		return 0, nil
	default:
		// 成功时返回数据区偏移；其他错误原样上报，不掩盖实际的读取或定位故障。
		if metrics != nil {
			metrics.seekSuccess.Add(context.Background(), 1)
		}
		return offset, err
	}
}
