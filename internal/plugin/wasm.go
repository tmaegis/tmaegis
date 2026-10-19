// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

// maxWasmOutput caps what a plugin may write to stdout/stderr.
const maxWasmOutput = 1 << 20

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > maxWasmOutput {
		return 0, errors.New("plugin output limit exceeded")
	}
	return b.Buffer.Write(p)
}

// RunWasm executes a WASI plugin in a sandbox: no filesystem, no network, no env.
// The scan report JSON is delivered on stdin; whatever the plugin prints to stdout is returned.
// Plugins can be built with `GOOS=wasip1 GOARCH=wasm go build` or TinyGo/Rust `wasm32-wasip1`.
func RunWasm(ctx context.Context, path string, stdin []byte) ([]byte, error) {
	code, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Closing on context cancellation lets the timeout stop a runaway plugin.
	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCloseOnContextDone(true))
	defer rt.Close(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	compiled, err := rt.CompileModule(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("invalid wasm plugin %s: %w", path, err)
	}

	var out limitedBuffer
	cfg := wazero.NewModuleConfig().
		WithStdin(bytes.NewReader(stdin)).
		WithStdout(&out).
		WithStderr(&out).
		WithName("")
	_, err = rt.InstantiateModule(ctx, compiled, cfg)
	if err != nil {
		var exit *sys.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 0 {
			return out.Bytes(), nil
		}
		return out.Bytes(), fmt.Errorf("wasm plugin failed: %w", err)
	}
	return out.Bytes(), nil
}
