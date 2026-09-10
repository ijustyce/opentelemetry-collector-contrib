// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fingerprint // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/compression"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fileoffset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/metadata"
)

const DefaultSize = 1000 // bytes

const MinSize = 16 // bytes

// Fingerprint is used to identify a file
// A file's fingerprint is the first N bytes of the file
type Fingerprint struct {
	firstBytes []byte
}

func New(first []byte) *Fingerprint {
	return &Fingerprint{firstBytes: first}
}

// NewFromFile computes the fingerprint using the first 'N' bytes after the
// leading NUL prefix of an uncompressed file, without changing its position.
// Set decompressData to true to compute fingerprint of compressed files by decompressing its data first
func NewFromFile(file *os.File, size int, decompressData bool, logger *zap.Logger) (*Fingerprint, error) {
	buf := make([]byte, size)
	if metadata.FilelogDecompressFingerprintFeatureGate.IsEnabled() {
		if decompressData {
			if compression.IsGzipFile(file, logger) {
				// If the file is of compressed type, uncompress the data before creating its fingerprint
				uncompressedData, err := gzip.NewReader(io.NewSectionReader(file, 0, 1<<63-1))
				if err != nil {
					return nil, fmt.Errorf("error uncompressing gzip file: %w", err)
				}
				defer uncompressedData.Close()

				n, err := io.ReadFull(uncompressedData, buf)
				if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
					return nil, fmt.Errorf("error reading fingerprint bytes: %w", err)
				}
				return New(buf[:n]), nil
			}
		}
	}

	if size == 0 {
		return New(buf), nil
	}
	// 正常文件只从文件头读取一次指纹，避免额外的空洞定位和前导字节扫描。
	n, err := file.ReadAt(buf, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading fingerprint bytes: %w", err)
	}
	if n == 0 || buf[0] != 0 {
		return New(buf[:n]), nil
	}
	// 仅在首字节为 NUL 时定位实际内容，并复用缓冲区重新读取指纹。
	offset, err := fileoffset.FirstNonNUL(file)
	if err != nil {
		return nil, fmt.Errorf("finding fingerprint start: %w", err)
	}
	n, err = file.ReadAt(buf, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading fingerprint bytes: %w", err)
	}
	return New(buf[:n]), nil
}

// Copy creates a new copy of the fingerprint
func (f Fingerprint) Copy() *Fingerprint {
	buf := make([]byte, len(f.firstBytes), cap(f.firstBytes))
	n := copy(buf, f.firstBytes)
	return New(buf[:n])
}

func (f *Fingerprint) Len() int {
	return len(f.firstBytes)
}

// Equal returns true if the fingerprints have the same FirstBytes,
// false otherwise. This does not compare other aspects of the fingerprints
// because the primary purpose of a fingerprint is to convey a unique
// identity, and only the FirstBytes field contributes to this goal.
func (f Fingerprint) Equal(other *Fingerprint) bool {
	return bytes.Equal(f.firstBytes, other.firstBytes)
}

// StartsWith returns true if the fingerprints are the same
// or if the new fingerprint starts with the old one
// This is important functionality for tracking new files,
// since their initial size is typically less than that of
// a fingerprint. As the file grows, its fingerprint is updated
// until it reaches a maximum size, as configured on the operator
func (f Fingerprint) StartsWith(old *Fingerprint) bool {
	l0 := len(old.firstBytes)
	if l0 == 0 {
		return false
	}
	l1 := len(f.firstBytes)
	if l0 > l1 {
		return false
	}
	return bytes.Equal(old.firstBytes[:l0], f.firstBytes[:l0])
}

func (f *Fingerprint) MarshalJSON() ([]byte, error) {
	m := marshal{FirstBytes: f.firstBytes}
	return json.Marshal(&m)
}

func (f *Fingerprint) UnmarshalJSON(data []byte) error {
	m := new(marshal)
	if err := json.Unmarshal(data, m); err != nil {
		return err
	}
	f.firstBytes = m.FirstBytes
	return nil
}

type marshal struct {
	FirstBytes []byte `json:"first_bytes"`
}
