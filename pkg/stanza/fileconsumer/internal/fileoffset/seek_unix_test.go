// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package fileoffset

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSeekDataSkipsHole(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "hole")
	require.NoError(t, err)
	defer file.Close()
	const dataOffset = 256 << 20
	_, err = file.WriteAt([]byte("log"), dataOffset)
	require.NoError(t, err)
	offset, err := seekData(file)
	require.NoError(t, err)
	if offset == 0 {
		t.Skip("filesystem does not report leading holes")
	}
	require.LessOrEqual(t, offset, int64(dataOffset))
	t.Logf("SEEK_DATA skipped %d bytes", offset)
}
