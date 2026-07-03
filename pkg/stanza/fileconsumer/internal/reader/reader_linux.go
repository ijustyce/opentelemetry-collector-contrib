// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package reader // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"

import (
	"errors"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

func (r *Reader) tryLockFile() bool {
	if err := unix.Flock(int(r.file.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
		if !errors.Is(err, unix.EWOULDBLOCK) {
			r.set.Logger.Error("Failed to lock", zap.Error(err))
		}
		return false
	}

	return true
}

func (r *Reader) unlockFile() {
	if err := unix.Flock(int(r.file.Fd()), unix.LOCK_UN); err != nil {
		// If delete_after_read is set then the file may already have been deleted by this point,
		// in which case we'll get EBADF.  This is harmless and not worth logging.
		if !errors.Is(err, unix.EBADF) {
			r.set.Logger.Error("Failed to unlock", zap.Error(err))
		}
	}
}

func (r *Reader) fadviseFile() {
	if r.file == nil {
		r.Reset(r.Offset)
		return
	}

	if length := r.Offset - r.DontNeedOffset; length > 0 {
		if err := unix.Fadvise(int(r.file.Fd()), r.DontNeedOffset, length, unix.FADV_DONTNEED); err != nil {
			r.set.Logger.Warn("fadvise DONTNEED failed", zap.Error(err))
		} else {
			r.set.Logger.Info("fadvise DONTNEED success", zap.Int64("offset", r.Offset),
				zap.Int64("dontNeedOffset", r.DontNeedOffset), zap.Int64("length", length),
				zap.Int64("DontNeedIdlePolls", r.DontNeedIdlePolls))
			r.DontNeedOffset = r.Offset
			r.DontNeedIdlePolls = 0
		}
	}
}
