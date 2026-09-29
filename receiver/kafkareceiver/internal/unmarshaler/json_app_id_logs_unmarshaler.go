// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/unmarshaler"
import (
	"time"

	"github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

var _ plog.Unmarshaler = JSONAppIdLogsUnmarshaler{}

type JSONAppIdLogsUnmarshaler struct{}

func removeNilFields(value any) {
	switch record := value.(type) {
	case map[string]any:
		for key, field := range record {
			if field == nil {
				delete(record, key)
				continue
			}
			removeNilFields(field)
		}
	case []any:
		for _, item := range record {
			removeNilFields(item)
		}
	}
}

func hasAppId(value any) bool {
	switch record := value.(type) {
	case map[string]any:
		for k, v := range record {
			if k == "app_id" && v != nil && v != "" {
				return true
			}
			if hasAppId(v) {
				return true
			}
		}
	case []any:
		for _, item := range record {
			if hasAppId(item) {
				return true
			}
		}
	}
	return false
}

func (JSONAppIdLogsUnmarshaler) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	// create a new Logs struct to be populated with log data and returned
	p := plog.NewLogs()

	// get json logs from the buffer
	jsonVal := map[string]any{}
	if err := json.Unmarshal(buf, &jsonVal); err != nil {
		return p, err
	}

	removeNilFields(jsonVal)
	if !hasAppId(jsonVal) {
		return p, nil
	}
	// create a new log record
	logRecords := p.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	logRecords.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Set the unmarshaled jsonVal as the body of the log record
	if err := logRecords.Body().SetEmptyMap().FromRaw(jsonVal); err != nil {
		return p, err
	}
	return p, nil
}
