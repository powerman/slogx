package slogx_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/powerman/slogx"
)

func ExampleLayoutHandlerOptions_formatRemoveAttr() {
	// This example demonstrates how to use the LayoutHandlerOptions to customize
	// the log output format by omitting the time field.
	opts := slogx.LayoutHandlerOptions{
		Format: map[string]string{
			slog.TimeKey: "", // Omit field.
		},
	}
	logger := slog.New(slogx.NewLayoutHandler(os.Stdout, &opts))
	logger.Info("Test message")
	// Output:
	// level=INFO msg="Test message"
}

func ExampleLayoutHandlerOptions_formatShortenLevel() {
	// This example demonstrates how to use the LayoutHandlerOptions to customize
	// the log output format by shortening the level field to 3 characters.
	opts := slogx.LayoutHandlerOptions{
		Level: slog.LevelDebug - 2, // Output all levels.
		Format: map[string]string{
			slog.TimeKey:  "",            // Omit time field for predictable output.
			slog.LevelKey: "level=%3.3v", // Use alternate level format.
		},
	}
	logger := slog.New(slogx.NewLayoutHandler(os.Stdout, &opts))
	for level := slog.LevelDebug - 1; level <= slog.LevelError+1; level++ {
		logger.LogAttrs(context.Background(), level, "Test message")
	}
	// Output:
	// level=D-1 msg="Test message"
	// level=DBG msg="Test message"
	// level=D+1 msg="Test message"
	// level=D+2 msg="Test message"
	// level=D+3 msg="Test message"
	// level=INF msg="Test message"
	// level=I+1 msg="Test message"
	// level=I+2 msg="Test message"
	// level=I+3 msg="Test message"
	// level=WRN msg="Test message"
	// level=W+1 msg="Test message"
	// level=W+2 msg="Test message"
	// level=W+3 msg="Test message"
	// level=ERR msg="Test message"
	// level=E+1 msg="Test message"
}

func ExampleLayoutHandlerOptions_formatTruncate() {
	uuidV1s := []string{
		"6f31402a-a14d-11f0-a01f-169545a16ec7",
		"6f942726-a14d-11f0-9fa9-169545a16ec7",
		"6ffbe866-a14d-11f0-8efe-169545a16ec7",
	}
	uuidV7s := []string{
		"0199b06f-3a9d-7634-9ed4-5a0927ebfb89",
		"0199b06f-3ead-75d3-9080-c9b2a0aac316",
		"0199b06f-424d-74e9-8f89-fa5b2cc24170",
	}

	// This example demonstrates how to use the LayoutHandlerOptions to customize
	// the log output format by truncating given fields to a maximum length.
	opts := slogx.LayoutHandlerOptions{
		Format: map[string]string{
			slog.TimeKey: "",                // Omit time field for predictable output.
			"uuid_v1":    " uuid_v1=%.9v",   // Truncate to 9 characters.
			"uuid_v7":    " uuid_v7=%.-13v", // Truncate to last 13 characters.
		},
	}
	logger := slog.New(slogx.NewLayoutHandler(os.Stdout, &opts))
	for i := range 3 {
		logger.Info("Test message", "uuid_v1", uuidV1s[i], "uuid_v7", uuidV7s[i])
	}
	// Output:
	// level=INFO msg="Test message" uuid_v1=6f31402a… uuid_v7=…5a0927ebfb89
	// level=INFO msg="Test message" uuid_v1=6f942726… uuid_v7=…c9b2a0aac316
	// level=INFO msg="Test message" uuid_v1=6ffbe866… uuid_v7=…fa5b2cc24170
}

func ExampleLayoutHandlerOptions_formatRedact() {
	// This example demonstrates how to use the LayoutHandlerOptions to customize
	// the log output format by redacting sensitive fields.
	opts := slogx.LayoutHandlerOptions{
		Format: map[string]string{
			slog.TimeKey: "",               // Omit time field for predictable output.
			"pass":       " pass=REDACTED", // Replace field value with constant.
		},
	}
	logger := slog.New(slogx.NewLayoutHandler(os.Stdout, &opts))
	logger.Info("Test message", "user", "alice", "pass", "s3cr3t")
	// Output:
	// level=INFO msg="Test message" user=alice pass=REDACTED
}

func ExampleLayoutHandlerOptions_formatCustomAttrs() {
	// This example demonstrates how to use the LayoutHandlerOptions to customize
	// the log output format by adding custom formatted fields.
	opts := slogx.LayoutHandlerOptions{
		Format: map[string]string{
			slog.TimeKey: "",    // Omit time field for predictable output.
			"host":       " %v", // Omit field name.
			"port":       ":%v", // Omit field name and separate with colon.
		},
	}
	logger := slog.New(slogx.NewLayoutHandler(os.Stdout, &opts))
	// Order of host and port attributes does matter - port must follow host.
	logger.Info("Test message", "a", 1, "host", "localhost", "port", 8080, "z", true)
	// Output:
	// level=INFO msg="Test message" a=1 localhost:8080 z=true
}

func ExampleLayoutHandlerOptions_formatUnquoted() {
	type Data struct {
		Key1 string `json:"key1"`
		Key2 int    `json:"key2"`
		Key3 bool   `json:"key3"`
	}
	data := Data{
		Key1: "A Key4:B",
		Key2: 2,
		Key3: true,
	}
	val, _ := json.Marshal(data)
	valIndent, _ := json.MarshalIndent(data, "", "  ")

	// This example demonstrates how to use the LayoutHandlerOptions to customize
	// the log output format by outputting JSON without extra quotes.
	opts := slogx.LayoutHandlerOptions{
		Level: slog.LevelDebug,
		Format: map[string]string{
			slog.TimeKey: "",         // Omit time field for predictable output.
			"debug":      " %s",      // Unquoted value without key.
			"json":       " json=%s", // Unquoted value.
			"multiline":  "\n%s",     // Unquoted value on a new line.
		},
	}
	logger := slog.New(slogx.NewLayoutHandler(os.Stdout, &opts))
	logger.Debug("Test message", "debug", fmt.Sprintf("%#+v", data))
	logger.Debug("Test message", "default", data)
	logger.Info("Test message", "json", val)
	logger.Info("Test message", "multiline", valIndent)
	logger.Info("Test message", "default", val)
	// Output:
	// level=DEBUG msg="Test message" slogx_test.Data{Key1:"A Key4:B", Key2:2, Key3:true}
	// level=DEBUG msg="Test message" default="{Key1:A Key4:B Key2:2 Key3:true}"
	// level=INFO msg="Test message" json={"key1":"A Key4:B","key2":2,"key3":true}
	// level=INFO msg="Test message"
	// {
	//   "key1": "A Key4:B",
	//   "key2": 2,
	//   "key3": true
	// }
	// level=INFO msg="Test message" default="{\"key1\":\"A Key4:B\",\"key2\":2,\"key3\":true}"
}

func ExampleLayoutHandlerOptions_formatColorAttr() {
	// This example demonstrates how to use the LayoutHandlerOptions to customize
	// the log output format by adding color to specific fields.
	opts := slogx.LayoutHandlerOptions{
		Format: map[string]string{
			slog.TimeKey: "",                       // Omit time field for predictable output.
			"err":        " err=\x1b[91m%v\x1b[0m", // Bright red color.
		},
	}
	logger := slog.New(slogx.NewLayoutHandler(os.Stdout, &opts))
	logger.Error("Test message", "status", 500, "err", io.EOF)
	// Output:
	// level=ERROR msg="Test message" status=500 err=[91mEOF[0m
}

func ExampleLayoutHandlerOptions_prefixVerticalAlign() {
	// This example demonstrates how to use the LayoutHandlerOptions to customize
	// the log output format by vertically aligning the prefix fields.
	opts := slogx.LayoutHandlerOptions{
		Format: map[string]string{
			slog.TimeKey:    "",       // Omit time field for predictable output.
			slog.LevelKey:   "%-5v",   // Set fixed width for level.
			slog.MessageKey: " %v",    // Omit field name.
			"app":           " %v",    // Omit field name.
			"server":        " [%7v]", // Set padding for server name.
			"remote_ip":     " %-15v", // Set left align and padding for remote IP.
			"http_method":   " %7v",   // Set padding for HTTP method.
			"http_code":     " %3s",   // Set padding for HTTP code placeholder.
			"user_id":       " @%v",   // Replace field name with "@" mark.
		},
		PrefixKeys: []string{
			"app",    // App name is fixed for all log records.
			"server", // "OpenAPI", "gRPC", "Metrics", etc.
			"remote_ip",
			"http_method",
			"http_code",
		},
		SuffixKeys: []string{
			slog.SourceKey, // Move here to keep prefix small and fixed width.
			"user_id",      // Place at EOL to easily spot user-related records.
		},
	}
	logger := slog.New(slogx.NewLayoutHandler(os.Stdout, &opts))
	logger = logger.
		// Set in main().
		With("app", "MyApp").
		// Set in OpenAPI middleware.
		With(
			"server", "OpenAPI",
			"remote_ip", "192.168.100.1",
			"http_method", "GET",
			"http_code", "", // A placeholder, will be set later.
		).
		// Set in auth middleware if known.
		With("user_id", "alice")
	logger.Warn("Something is wrong", "err", io.EOF)
	logger.Error("Request handled", "http_code", 500)
	// Output:
	// WARN  MyApp [OpenAPI] 192.168.100.1       GET     "Something is wrong" err=EOF @alice
	// ERROR MyApp [OpenAPI] 192.168.100.1       GET 500 "Request handled" @alice
}

func ExampleLayoutHandlerOptions_timeFormat() {
	now, _ := time.Parse(time.RFC3339Nano, "2006-01-02T15:04:05.789123456+01:00")
	setRecordTime := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey && len(groups) == 0 {
			a.Value = slog.TimeValue(now)
		}
		return a
	}

	// This example demonstrates how to use the LayoutHandlerOptions to customize
	// the log output format by changing the time format.
	opts := slogx.LayoutHandlerOptions{
		ReplaceAttr:      setRecordTime, // Fix time for the example output.
		RecordTimeFormat: time.Kitchen,
		TimeFormat:       time.TimeOnly,
	}
	logger := slog.New(slogx.NewLayoutHandler(os.Stdout, &opts))
	logger.Info("Test message", "something", now)
	// Output:
	// time=3:04PM level=INFO msg="Test message" something=15:04:05
}
