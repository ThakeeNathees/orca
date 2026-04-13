package cmd

import (
	"errors"
	"strings"
	"testing"
)

type commandCall struct {
	Dir  string
	Name string
	Args []string
}

func withRunDependencies(
	t *testing.T,
	compileFn func() (compileResult, error),
	execFn func(dir, name string, args ...string) error,
) {
	t.Helper()

	previousCompile := compileForRun
	previousExec := executeCommandInDir
	previousLookup := lookupUV

	compileForRun = compileFn
	executeCommandInDir = execFn
	lookupUV = func() (string, error) { return "uv", nil }

	t.Cleanup(func() {
		compileForRun = previousCompile
		executeCommandInDir = previousExec
		lookupUV = previousLookup
	})
}

func TestRunRunFailsWithHelpfulMessageWhenUvMissing(t *testing.T) {
	withRunDependencies(
		t,
		func() (compileResult, error) {
			return compileResult{FileCount: 1, OutputDir: "build"}, nil
		},
		func(dir, name string, args ...string) error {
			t.Fatalf("executeCommandInDir should not run when uv is missing")
			return nil
		},
	)
	lookupUV = func() (string, error) { return "", errors.New("not found") }

	err := runRun(nil, nil)
	if err == nil {
		t.Fatal("expected error when uv is missing")
	}
	msg := err.Error()
	if !strings.Contains(msg, "uv not found on PATH") {
		t.Errorf("expected 'uv not found on PATH' in error, got %q", msg)
	}
	if !strings.Contains(msg, "TODO: auto-download uv binary to ~/.orcalang/bin/uv") {
		t.Errorf("expected TODO auto-download line in error, got %q", msg)
	}
	if !strings.Contains(msg, "install uv manually") {
		t.Errorf("expected manual install hint in error, got %q", msg)
	}
}

func TestRunRunExecutesUvSyncThenUvRunWithArgs(t *testing.T) {
	var calls []commandCall

	withRunDependencies(
		t,
		func() (compileResult, error) {
			return compileResult{
				FileCount: 2,
				OutputDir: "build",
			}, nil
		},
		func(dir, name string, args ...string) error {
			calls = append(calls, commandCall{
				Dir:  dir,
				Name: name,
				Args: append([]string(nil), args...),
			})
			return nil
		},
	)

	err := runRun(nil, []string{"p1", "p2", "p3"})
	if err != nil {
		t.Fatalf("runRun returned error: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("expected 2 uv command calls, got %d", len(calls))
	}

	if calls[0].Dir != "build" || calls[0].Name != "uv" {
		t.Fatalf("unexpected first call metadata: %+v", calls[0])
	}
	if got := strings.Join(calls[0].Args, " "); got != "sync" {
		t.Fatalf("unexpected first call args: %q", got)
	}

	if calls[1].Dir != "build" || calls[1].Name != "uv" {
		t.Fatalf("unexpected second call metadata: %+v", calls[1])
	}
	if got := strings.Join(calls[1].Args, " "); got != "run main.py p1 p2 p3" {
		t.Fatalf("unexpected second call args: %q", got)
	}
}

func TestRunRunReturnsCompileErrorWithoutExecutingUv(t *testing.T) {
	compileErr := errors.New("compile failed")
	execCalled := false

	withRunDependencies(
		t,
		func() (compileResult, error) {
			return compileResult{}, compileErr
		},
		func(dir, name string, args ...string) error {
			execCalled = true
			return nil
		},
	)

	err := runRun(nil, nil)
	if !errors.Is(err, compileErr) {
		t.Fatalf("expected compile error, got %v", err)
	}
	if execCalled {
		t.Fatalf("uv commands should not execute when compile fails")
	}
}

func TestRunRunStopsWhenUvSyncFails(t *testing.T) {
	uvSyncErr := errors.New("uv sync failed")
	callCount := 0

	withRunDependencies(
		t,
		func() (compileResult, error) {
			return compileResult{
				FileCount: 1,
				OutputDir: "build",
			}, nil
		},
		func(dir, name string, args ...string) error {
			callCount++
			if callCount == 1 {
				return uvSyncErr
			}
			return nil
		},
	)

	err := runRun(nil, []string{"arg"})
	if err == nil {
		t.Fatalf("expected error when uv sync fails")
	}
	if !strings.Contains(err.Error(), "failed to run uv sync") {
		t.Fatalf("expected uv sync context in error, got %q", err)
	}
	if callCount != 1 {
		t.Fatalf("expected only uv sync to run, got %d calls", callCount)
	}
}

func TestRunRunPassesFlagLikeArgsToUvRun(t *testing.T) {
	var secondCallArgs []string
	callCount := 0

	withRunDependencies(
		t,
		func() (compileResult, error) {
			return compileResult{
				FileCount: 1,
				OutputDir: "build",
			}, nil
		},
		func(dir, name string, args ...string) error {
			callCount++
			if callCount == 2 {
				secondCallArgs = append([]string(nil), args...)
			}
			return nil
		},
	)

	err := runRun(nil, []string{"--foo", "bar"})
	if err != nil {
		t.Fatalf("runRun returned error: %v", err)
	}

	if got := strings.Join(secondCallArgs, " "); got != "run main.py --foo bar" {
		t.Fatalf("unexpected uv run args: %q", got)
	}
}
