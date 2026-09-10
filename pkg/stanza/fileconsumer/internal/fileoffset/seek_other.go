// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux && !darwin

package fileoffset

import "os"

func seekData(*os.File) (int64, error) {
	return 0, nil
}
