// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileoffset

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFirstNonNUL(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prefix int64
		data   string
		want   int64
	}{
		{name: "empty"},
		{name: "ordinary", data: "ab\x00cd"},
		{name: "written NULs", data: "\x00\x00ab\x00cd", want: 2},
		{name: "only NULs", data: "\x00\x00", want: 2},
		{name: "large unaligned hole", prefix: (256 << 20) + 123, data: "\x00\x00ab\x00cd", want: (256 << 20) + 125},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "sparse")
			require.NoError(t, err)
			defer file.Close()
			_, err = file.WriteAt([]byte(tc.data), tc.prefix)
			require.NoError(t, err)
			_, err = file.Seek(7, io.SeekStart)
			require.NoError(t, err)
			offset, err := FirstNonNUL(file, nil)
			require.NoError(t, err)
			require.Equal(t, tc.want, offset)
			position, err := file.Seek(0, io.SeekCurrent)
			require.NoError(t, err)
			require.Equal(t, int64(7), position)
		})
	}
}

func TestFirstNonNULOnlyHole(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "hole")
	require.NoError(t, err)
	defer file.Close()
	const size = 64 << 20
	require.NoError(t, file.Truncate(size))
	offset, err := FirstNonNUL(file, nil)
	require.NoError(t, err)
	require.Equal(t, int64(size), offset)
}

func BenchmarkFirstNonNULLargeHole(b *testing.B) {
	file, err := os.CreateTemp(b.TempDir(), "hole")
	require.NoError(b, err)
	defer file.Close()
	const start = (256 << 20) + 123
	_, err = file.WriteAt([]byte("log"), start)
	require.NoError(b, err)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		offset, readErr := FirstNonNUL(file, nil)
		if readErr != nil || offset != start {
			b.Fatalf("offset=%d, err=%v", offset, readErr)
		}
	}
}

func TestOffset(t *testing.T) {
	file, err := os.Open("/tmp/alloy.log")
	assert.Nil(t, err)
	defer file.Close()

	FirstNonNUL(file, nil)
}
