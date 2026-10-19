// Copyright (c) 2026 Tommy Huynh & Misha Milanovic. All rights reserved.
// SPDX-License-Identifier: MPL-2.0
// TMAegis™ is a trademark of the project founders. See TRADEMARK.md for details.

package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildWasm compiles a tiny WASI program with the host Go toolchain.
func buildWasm(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module p\ngo 1.22\n"), 0o600)
	out := filepath.Join(dir, "p.wasm")
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot build wasip1 plugin: %v\n%s", err, b)
	}
	return out
}

func TestRunWasmEchoesStdin(t *testing.T) {
	wasm := buildWasm(t, `package main
import ("io";"os";"fmt")
func main(){ b,_ := io.ReadAll(os.Stdin); fmt.Printf("got:%s", b) }`)
	out, err := RunWasm(context.Background(), wasm, []byte(`{"total_rules":3}`))
	if err != nil || string(out) != `got:{"total_rules":3}` {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestRunWasmSandboxAndFailures(t *testing.T) {
	// No filesystem is preopened, so reading a host file must fail inside the sandbox.
	wasm := buildWasm(t, `package main
import ("os";"fmt")
func main(){ _, err := os.ReadFile("/etc/passwd"); fmt.Print(err != nil) }`)
	if out, err := RunWasm(context.Background(), wasm, nil); err != nil || string(out) != "true" {
		t.Fatalf("sandbox leak? out=%q err=%v", out, err)
	}

	exit := buildWasm(t, `package main
import "os"
func main(){ os.Exit(3) }`)
	if _, err := RunWasm(context.Background(), exit, nil); err == nil {
		t.Fatal("non-zero exit must be an error")
	}

	spin := buildWasm(t, `package main
func main(){ for {} }`)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if _, err := RunWasm(ctx, spin, nil); err == nil {
		t.Fatal("runaway plugin must be stopped by the context deadline")
	}

	bad := filepath.Join(t.TempDir(), "bad.wasm")
	os.WriteFile(bad, []byte("not wasm"), 0o600)
	if _, err := RunWasm(context.Background(), bad, nil); err == nil || !strings.Contains(err.Error(), "invalid wasm") {
		t.Fatalf("expected invalid wasm error, got %v", err)
	}
}
