package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// runRunTestCase defines inputs for run command tests.
type runRunTestCase struct {
	name string
	oc   string
}

// TestRunRun verifies that the run command builds output and invokes Python.
func TestRunRun(t *testing.T) {
	tests := []runRunTestCase{
		{
			name: "builds and runs generated code",
			oc: `let {
  answer = 42
}
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			workingDir := t.TempDir()
			ocPath := filepath.Join(workingDir, "sample.oc")
			if err := os.WriteFile(ocPath, []byte(tc.oc), 0644); err != nil {
				t.Fatalf("failed to write .oc file: %v", err)
			}

			pythonDir := filepath.Join(workingDir, "bin")
			if err := os.MkdirAll(pythonDir, 0755); err != nil {
				t.Fatalf("failed to create python dir: %v", err)
			}

			calledPath := filepath.Join(workingDir, "python.called")
			pythonScript := "#!/bin/sh\n" +
				"echo \"$@\" > \"$ORCA_PYTHON_CALLED\"\n"
			pythonPath := filepath.Join(pythonDir, "python")
			if err := os.WriteFile(pythonPath, []byte(pythonScript), 0755); err != nil {
				t.Fatalf("failed to write python stub: %v", err)
			}

			originalPath := os.Getenv("PATH")
			originalCalled := os.Getenv("ORCA_PYTHON_CALLED")
			originalWd, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}

			if err := os.Setenv("PATH", pythonDir+string(os.PathListSeparator)+originalPath); err != nil {
				t.Fatalf("failed to set PATH: %v", err)
			}
			if err := os.Setenv("ORCA_PYTHON_CALLED", calledPath); err != nil {
				t.Fatalf("failed to set ORCA_PYTHON_CALLED: %v", err)
			}
			if err := os.Chdir(workingDir); err != nil {
				t.Fatalf("failed to switch working directory: %v", err)
			}

			t.Cleanup(func() {
				if err := os.Setenv("PATH", originalPath); err != nil {
					t.Errorf("failed to restore PATH: %v", err)
				}
				if err := os.Setenv("ORCA_PYTHON_CALLED", originalCalled); err != nil {
					t.Errorf("failed to restore ORCA_PYTHON_CALLED: %v", err)
				}
				if err := os.Chdir(originalWd); err != nil {
					t.Errorf("failed to restore working directory: %v", err)
				}
			})

			if err := runRun(&cobra.Command{}, nil); err != nil {
				t.Fatalf("runRun returned error: %v", err)
			}

			if _, err := os.Stat(filepath.Join(workingDir, "build", "main.py")); err != nil {
				t.Fatalf("expected build/main.py to exist: %v", err)
			}

			calledBytes, err := os.ReadFile(calledPath)
			if err != nil {
				t.Fatalf("expected python stub to be called: %v", err)
			}
			if got := strings.TrimSpace(string(calledBytes)); got != "main.py" {
				t.Fatalf("expected python to be invoked with main.py, got %q", got)
			}
		})
	}
}
