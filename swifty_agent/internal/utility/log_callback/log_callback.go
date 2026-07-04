// Package log_callback provides a callback handler for the Eino framework
// that logs pipeline component lifecycle events (start/end) to stdout.
package log_callback

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/callbacks"
)

// Config controls the verbosity of the log callback handler.
type Config struct {
	// Detail enables logging of input/output payloads.
	Detail bool
	// Debug enables pretty-printed (indented) JSON output.
	Debug bool
}

// NewHandler creates an Eino callback handler that logs component start/end events.
// If config is nil, a default configuration with Detail=true is used.
func NewHandler(config *Config) callbacks.Handler {
	if config == nil {
		config = &Config{Detail: true}
	}

	builder := callbacks.NewHandlerBuilder()
	builder.OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		fmt.Printf("[view start]:[%s:%s:%s]\n", info.Component, info.Type, info.Name)
		if config.Detail {
			var b []byte
			if config.Debug {
				b, _ = json.MarshalIndent(input, "", "  ")
			} else {
				b, _ = json.Marshal(input)
			}
			fmt.Printf("%s\n", string(b))
		}
		return ctx
	})
	builder.OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
		fmt.Printf("[view end]:[%s:%s:%s]\n", info.Component, info.Type, info.Name)
		return ctx
	})
	return builder.Build()
}
