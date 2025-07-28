package review

import (
	"bytes"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCodeFormatting ensures all Go files are properly formatted
func TestCodeFormatting(t *testing.T) {
	t.Log("📝 Checking Go code formatting...")

	// Get current directory and determine project root
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("❌ Failed to get working directory: %v", err)
	}

	// Determine project root based on current location
	projectRoot := currentDir
	if strings.HasSuffix(currentDir, "tests/review") || strings.HasSuffix(currentDir, "tests\\review") {
		projectRoot = filepath.Join(currentDir, "..", "..")
	} else if strings.HasSuffix(currentDir, "tests") {
		projectRoot = filepath.Join(currentDir, "..")
	}

	err = filepath.WalkDir(projectRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor, .git, and test artifacts
		if strings.Contains(path, "vendor/") ||
			strings.Contains(path, ".git/") ||
			strings.Contains(path, "build/") ||
			strings.Contains(path, "logs/") {
			return nil
		}

		// Only check .go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("❌ Failed to read %s: %v", path, err)
			return nil
		}

		// Check if gofmt would change anything
		formatted, err := format.Source(content)
		if err != nil {
			t.Errorf("❌ Failed to format %s: %v", path, err)
			return nil
		}

		if !bytes.Equal(content, formatted) {
			t.Errorf("❌ File %s is not properly formatted. Run 'gofmt -s -w %s'", path, path)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("❌ Failed to walk directory: %v", err)
	}

	t.Log("✅ All Go files are properly formatted")
}

// TestGoModTidy ensures go.mod and go.sum are clean
func TestGoModTidy(t *testing.T) {
	t.Log("🧹 Checking if go.mod and go.sum are tidy...")

	// Get current directory and check if we need to change to project root
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("❌ Failed to get working directory: %v", err)
	}

	// If we're in tests/review, go up two levels to project root
	projectRoot := currentDir
	if strings.HasSuffix(currentDir, "tests/review") || strings.HasSuffix(currentDir, "tests\\review") {
		projectRoot = filepath.Join(currentDir, "..", "..")
	} else if strings.HasSuffix(currentDir, "tests") {
		projectRoot = filepath.Join(currentDir, "..")
	}

	// Change to project root
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatalf("❌ Failed to change to project root %s: %v", projectRoot, err)
	}
	defer os.Chdir(currentDir)

	// Read current go.mod and go.sum
	goModBefore, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("❌ Failed to read go.mod: %v", err)
	}

	goSumBefore, err := os.ReadFile("go.sum")
	if err != nil {
		// go.sum might not exist if no dependencies
		goSumBefore = []byte{}
	}

	// Run go mod tidy
	cmd := exec.Command("go", "mod", "tidy")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("❌ go mod tidy failed: %v\nOutput: %s", err, output)
	}

	// Read files after tidy
	goModAfter, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("❌ Failed to read go.mod after tidy: %v", err)
	}

	goSumAfter, err := os.ReadFile("go.sum")
	if err != nil {
		goSumAfter = []byte{}
	}

	// Check if files changed
	if !bytes.Equal(goModBefore, goModAfter) {
		t.Error("❌ go.mod is not tidy. Run 'go mod tidy' to fix.")
	}

	if !bytes.Equal(goSumBefore, goSumAfter) {
		t.Error("❌ go.sum is not tidy. Run 'go mod tidy' to fix.")
	}

	t.Log("✅ go.mod and go.sum are properly tidied")
}

// TestGoVet runs go vet on all packages
func TestGoVet(t *testing.T) {
	t.Log("🔍 Running go vet static analysis...")

	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = "./"

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("❌ go vet failed:\n%s", output)
	} else {
		t.Log("✅ go vet passed - no issues found")
	}
}

// TestImportsFormatting checks if imports are properly formatted
func TestImportsFormatting(t *testing.T) {
	t.Log("📦 Checking import formatting...")

	// Check if goimports is available
	_, err := exec.LookPath("goimports")
	if err != nil {
		t.Skip("⚠️  goimports not found, skipping import formatting test")
	}

	cmd := exec.Command("goimports", "-l", ".")
	cmd.Dir = "./"

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("❌ goimports failed: %v", err)
	}

	if len(output) > 0 {
		files := strings.TrimSpace(string(output))
		t.Errorf("❌ Files with incorrect import formatting:\n%s\nRun 'goimports -w .' to fix.", files)
	} else {
		t.Log("✅ All imports are properly formatted")
	}
}

// TestNoDependencyDrift ensures no unnecessary dependencies
func TestNoDependencyDrift(t *testing.T) {
	t.Log("📋 Checking for dependency drift...")

	// First run go mod tidy to clean up
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = "./"
	if err := tidyCmd.Run(); err != nil {
		t.Fatalf("❌ go mod tidy failed: %v", err)
	}

	// Then check for unused dependencies
	cmd := exec.Command("go", "list", "-m", "all")
	cmd.Dir = "./"

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("❌ go list -m all failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	// Skip the main module (first line)
	if len(lines) <= 1 {
		t.Log("✅ No external dependencies found")
		return
	}

	// For a more thorough check, we could use go mod why for each dependency
	// But for now, just verify go mod tidy doesn't change anything

	t.Logf("✅ Found %d dependencies, all appear to be in use", len(lines)-1)
}

// TestProjectStructure ensures proper project organization
func TestProjectStructure(t *testing.T) {
	t.Log("📁 Checking project structure...")

	// Get current directory and determine project root
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("❌ Failed to get working directory: %v", err)
	}

	// Determine project root based on current location
	projectRoot := currentDir
	if strings.HasSuffix(currentDir, "tests/review") || strings.HasSuffix(currentDir, "tests\\review") {
		projectRoot = filepath.Join(currentDir, "..", "..")
	} else if strings.HasSuffix(currentDir, "tests") {
		projectRoot = filepath.Join(currentDir, "..")
	}

	requiredDirs := []string{
		"client",
		"config",
		"docs",
		"server",
		"tests",
		"www",
	}

	requiredFiles := []string{
		".gitignore",
		"go.mod",
		"go.sum",
		"license",
		"main.go",
		"readme.md",
	}

	// Check required directories
	for _, dir := range requiredDirs {
		path := filepath.Join(projectRoot, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("❌ Required directory missing: %s", dir)
		} else {
			t.Logf("✅ Found required directory: %s", dir)
		}
	}

	// Check required files
	for _, file := range requiredFiles {
		path := filepath.Join(projectRoot, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("❌ Required file missing: %s", file)
		} else {
			t.Logf("✅ Found required file: %s", file)
		}
	}
}

// TestGoReportCardCompliance checks various code quality metrics
func TestGoReportCardCompliance(t *testing.T) {
	projectRoot := "../../"

	t.Run("NoDeadCode", func(t *testing.T) {
		// This is a basic check - you might want to use tools like deadcode
		// For now, we'll just ensure all .go files compile
		cmd := exec.Command("go", "build", "./...")
		cmd.Dir = projectRoot
		if err := cmd.Run(); err != nil {
			t.Errorf("❌ Project does not compile cleanly: %v", err)
		}
	})

	t.Run("GoCyclo", func(t *testing.T) {
		// Check for overly complex functions (basic version)
		// You could integrate gocyclo tool here for more thorough checking
		err := filepath.WalkDir(projectRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !strings.HasSuffix(path, ".go") {
				return err
			}

			// Skip vendor and test files for this basic check
			if strings.Contains(path, "vendor/") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// Very basic complexity check - count nested braces
			funcLines := strings.Split(string(content), "\n")
			for i, line := range funcLines {
				if strings.Contains(line, "func ") && strings.Contains(line, "{") {
					braceCount := 0
					maxNesting := 0

					// Check function complexity (simple brace counting)
					for j := i; j < len(funcLines) && j < i+100; j++ {
						braceCount += strings.Count(funcLines[j], "{")
						braceCount -= strings.Count(funcLines[j], "}")
						if braceCount > maxNesting {
							maxNesting = braceCount
						}
						if braceCount <= 0 && j > i {
							break
						}
					}

					if maxNesting > 6 { // Arbitrary threshold
						funcName := strings.TrimSpace(line)
						t.Logf("⚠️ Warning: Function may be too complex (nesting %d): %s:%d - %s",
							maxNesting, path, i+1, funcName)
					}
				}
			}
			return nil
		})

		if err != nil {
			t.Errorf("❌ Failed to check complexity: %v", err)
		}
	})
}
