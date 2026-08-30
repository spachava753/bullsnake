# Runtime package notes

Before changing the runtime, read the `Virtual machine and frames`, `Object model
and runtime`, `Import system`, and `Testing` sections in `docs/impl.md`, plus the
matching design sections in `docs/architecture.md`.

Runtime stage work currently belongs on `feat/vm`. Do not merge that branch until
the user asks.

## Package contract

The runtime accepts immutable Bullsnake code objects. It prepares and validates
the complete code tree before executing a module. A `BytecodeError` reports bad
or unsupported code. A Python exception is a runtime value and crosses the Go
boundary through `UncaughtException` when Python code does not handle it.

Preparation must reject every unsupported opcode, operand, constant, code flag,
bad table index, bad jump, and bad stack path before any module side effect. It
must also inspect unreachable instructions and every child code object.

Keep those guards until the runtime feature is intentionally implemented. The
parser or compiler may already support the related source. That does not make it
a runtime feature. Validation must not skip dormant code, deferred annotations,
unreachable instructions, or code that appears unlikely to run.

Do not change internal errors merely to polish unfinished features for a future
release. Bullsnake has no tagged release checkpoint or public execution API yet.

All live Python references must stay in typed Go pointers or interfaces so Go's
collector can see them. Python calls switch heap-allocated frames in the one VM
loop instead of using Go calls as the Python call stack. Runtime-owned mutable
state belongs to one `Runtime`; only documented immutable singletons may be
shared.

Validation and execution must agree. When an opcode becomes supported, update
operand checks, stack paths, dispatch, runtime values, Python errors, tests, and
`docs/impl.md` in the same finished slice.

## Testing

Put language behavior that can be written in Python source in chunked files under
`testdata/execution`. Each successful chunk uses Python assertions. Each expected
Python failure names its exception type and exact message.

Use `validation_test.go` for hand-built bytecode, unsupported instructions,
invalid operands, bad code metadata, and failures that source cannot express.
Keep these tests even when the compiler never emits the bad code.

Use `runtime_test.go` only for Go-facing behavior, code and value identity,
runtime caches, or several modules that must share one runtime. Do not move
ordinary Python semantics back into Go source strings.

Add behavior tests before implementation. Research the pinned CPython revision
for selected Python behavior and use StarlarkX only as a Go design reference.
Finish one small runtime slice, run all repository checks, update both design
documents when needed, and commit it before starting the next slice.
