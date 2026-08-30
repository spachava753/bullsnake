package runtime_test

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spachava753/bullsnake/internal/compiler"
	"github.com/spachava753/bullsnake/internal/compiler/parser"
	"github.com/spachava753/bullsnake/internal/compiler/resolver"
	bullruntime "github.com/spachava753/bullsnake/internal/runtime"
)

type executionChunk struct {
	name        string
	moduleName  string
	source      string
	expectError bool
	messageSet  bool
	wantType    string
	wantMessage string
}

func TestExecutionFixtures(t *testing.T) {
	paths, err := filepath.Glob("testdata/execution/*.py")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		for _, chunk := range readExecutionChunks(t, path) {
			chunk := chunk
			t.Run(filepath.Base(path)+"/"+chunk.name, func(t *testing.T) {
				runExecutionChunk(t, path, chunk)
			})
		}
	}
}

func runExecutionChunk(t *testing.T, path string, chunk executionChunk) {
	t.Helper()
	module, err := parser.Parse(path, chunk.source)
	if err != nil {
		t.Fatal(err)
	}
	table, err := resolver.Resolve(path, module)
	if err != nil {
		t.Fatal(err)
	}
	code, err := compiler.Compile(path, module, table)
	if err != nil {
		t.Fatal(err)
	}
	executed, err := bullruntime.New().ExecuteModule(chunk.moduleName, code)
	if !chunk.expectError {
		if err != nil {
			t.Fatal(err)
		}
		if executed == nil {
			t.Fatal("execution returned no module")
		}
		return
	}
	if executed != nil {
		t.Fatalf("module = %#v, want nil after exception", executed)
	}
	var raised *bullruntime.UncaughtException
	if !errors.As(err, &raised) {
		t.Fatalf("error = %T %v, want *runtime.UncaughtException", err, err)
	}
	if got := raised.Exception().TypeName(); got != chunk.wantType {
		t.Errorf("exception type = %q, want %q", got, chunk.wantType)
	}
	if got := raised.Exception().Message(); got != chunk.wantMessage {
		t.Errorf("exception message = %q, want %q", got, chunk.wantMessage)
	}
}

// readExecutionChunks splits one feature file while padding each chunk so
// compiler and runtime diagnostics retain their physical fixture line numbers.
func readExecutionChunks(t *testing.T, path string) []executionChunk {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	physicalLines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	var chunks []executionChunk
	start := 0
	for line := 0; line <= len(physicalLines); line++ {
		if line < len(physicalLines) && strings.TrimSpace(physicalLines[line]) != "# ---" {
			continue
		}
		chunkLines := physicalLines[start:line]
		if strings.TrimSpace(strings.Join(chunkLines, "\n")) != "" {
			chunks = append(chunks, parseExecutionChunk(t, path, start+1, chunkLines))
		}
		start = line + 1
	}
	return chunks
}

// parseExecutionChunk reads comment metadata and leaves every directive in the
// source as ordinary Python trivia.
func parseExecutionChunk(
	t *testing.T,
	path string,
	startLine int,
	lines []string,
) executionChunk {
	t.Helper()
	chunk := executionChunk{
		name:       "line_" + strconv.Itoa(startLine),
		moduleName: "fixture",
	}
	for offset, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "# case:"):
			chunk.name = strings.TrimSpace(strings.TrimPrefix(trimmed, "# case:"))
		case strings.HasPrefix(trimmed, "# module:"):
			chunk.moduleName = strings.TrimSpace(strings.TrimPrefix(trimmed, "# module:"))
		case strings.HasPrefix(trimmed, "# error:"):
			chunk.expectError = true
			chunk.wantType = strings.TrimSpace(strings.TrimPrefix(trimmed, "# error:"))
		case strings.HasPrefix(trimmed, "# message:"):
			chunk.messageSet = true
			quoted := strings.TrimSpace(strings.TrimPrefix(trimmed, "# message:"))
			message, err := strconv.Unquote(quoted)
			if err != nil {
				t.Fatalf("%s:%d: invalid quoted message: %v", path, startLine+offset, err)
			}
			chunk.wantMessage = message
		}
	}
	if chunk.expectError != chunk.messageSet || (chunk.expectError && chunk.wantType == "") {
		t.Fatalf("%s:%d: error fixtures require both type and message", path, startLine)
	}
	chunk.source = strings.Repeat("\n", startLine-1) + strings.Join(lines, "\n") + "\n"
	return chunk
}
