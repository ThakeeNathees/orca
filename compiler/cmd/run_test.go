package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// runCommandTestCase defines inputs for run command tests.
type runCommandTestCase struct {
	name           string
	oc             string
	pythonOverride string
	expectError    bool
	expectPython   bool
}

// TestRunRun verifies that the run command builds output and invokes Python.
func TestRunRun(t *testing.T) {
	tests := []runCommandTestCase{
		{
			name: "builds and runs generated code",
			oc: `let {
  answer = 42
}
`,
			pythonOverride: "stub",
			expectPython:   true,
		},
		{
			name:        "fails on invalid syntax",
			oc:          "let { answer = 42\n",
			expectError: true,
		},
		{
			name: "fails when python is missing",
			oc: `let {
  answer = 42
}
`,
			pythonOverride: "missing",
			expectError:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("python stub uses POSIX shell")
			}

			workingDir := t.TempDir()
			ocPath := filepath.Join(workingDir, "sample.oc")
			if err := os.WriteFile(ocPath, []byte(tc.oc), 0644); err != nil {
				t.Fatalf("failed to write .oc file: %v", err)
			}

			pythonCalledMarkerPath := filepath.Join(workingDir, "python.called")
			pythonPath := ""
			if tc.pythonOverride == "stub" {
				pythonDir := filepath.Join(workingDir, "bin")
				if err := os.MkdirAll(pythonDir, 0755); err != nil {
					t.Fatalf("failed to create python dir: %v", err)
				}

				pythonScript := "#!/bin/sh\n" +
					"printf \"%s\\n%s\\n\" \"$(pwd)\" \"$*\" > \"$ORCA_PYTHON_CALLED\"\n"
				pythonPath = filepath.Join(pythonDir, "python")
				if err := os.WriteFile(pythonPath, []byte(pythonScript), 0755); err != nil {
					t.Fatalf("failed to write python stub: %v", err)
				}
			}

			originalPython := os.Getenv("ORCA_PYTHON")
			originalCalled := os.Getenv("ORCA_PYTHON_CALLED")
			originalWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}

			if tc.pythonOverride == "missing" {
				pythonPath = filepath.Join(workingDir, "missing-python")
			}
			if pythonPath != "" {
				if err := os.Setenv("ORCA_PYTHON", pythonPath); err != nil {
					t.Fatalf("failed to set ORCA_PYTHON: %v", err)
				}
			} else if err := os.Unsetenv("ORCA_PYTHON"); err != nil {
				t.Fatalf("failed to unset ORCA_PYTHON: %v", err)
			}
			if err := os.Setenv("ORCA_PYTHON_CALLED", pythonCalledMarkerPath); err != nil {
				t.Fatalf("failed to set ORCA_PYTHON_CALLED: %v", err)
			}
			if err := os.Chdir(workingDir); err != nil {
				t.Fatalf("failed to switch working directory: %v", err)
			}

			t.Cleanup(func() {
				if err := os.Setenv("ORCA_PYTHON", originalPython); err != nil {
					t.Errorf("failed to restore ORCA_PYTHON: %v", err)
				}
				if err := os.Setenv("ORCA_PYTHON_CALLED", originalCalled); err != nil {
					t.Errorf("failed to restore ORCA_PYTHON_CALLED: %v", err)
				}
				if err := os.Chdir(originalWd); err != nil {
					t.Errorf("failed to restore working directory: %v", err)
				}
			})

			err = runRun(&cobra.Command{}, nil)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected runRun to return an error")
				}
			} else if err != nil {
				t.Fatalf("runRun returned error: %v", err)
			}

			if !tc.expectError {
				if _, err := os.Stat(filepath.Join(workingDir, buildOutputDir, "main.py")); err != nil {
					t.Fatalf("expected build/main.py to exist: %v", err)
				}
			}

			if tc.expectPython {
				calledBytes, err := os.ReadFile(pythonCalledMarkerPath)
				if err != nil {
					t.Fatalf("expected python stub to be called: %v", err)
				}
				lines := strings.Split(strings.TrimSpace(string(calledBytes)), "\n")
				if len(lines) < 2 {
					t.Fatalf("expected python stub to log cwd and args, got %q", string(calledBytes))
				}
				if got := lines[0]; got != filepath.Join(workingDir, buildOutputDir) {
					t.Fatalf("expected python to run from build dir, got %q", got)
				}
				if got := lines[1]; got != "main.py" {
					t.Fatalf("expected python to be invoked with main.py, got %q", got)
				}
			} else if _, err := os.Stat(pythonCalledMarkerPath); err == nil {
				t.Fatalf("expected python stub not to be called")
			}
		})
	}
}
