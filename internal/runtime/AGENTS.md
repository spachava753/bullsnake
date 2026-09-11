# Runtime package notes

Before changing the runtime, read the `Virtual machine and frames`, `Object model
and runtime`, `Import system`, and `Testing` sections in `docs/impl.md`, plus the
matching design sections in `docs/architecture.md`.

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
shared. Internal class registries and buffer export tables may hold Go `weak.Pointer`
references to actual Python class and memoryview allocations. Prune dead entries on the VM goroutine; never use a
temporary interface box as the weak target or invoke Python from a GC callback.

Validation and execution must agree. When an opcode becomes supported, update
operand checks, stack paths, dispatch, runtime values, Python errors, tests, and
`docs/impl.md` in the same finished slice.

Native I/O and generic-alias classes use per-runtime ordinary class allocations
and private typed instance storage. Shared constructors and method descriptors
live in native_class.go. Bind native instance methods through the shared attribute,
special-method, and super paths. Keep supplied class namespaces immutable and
user subclasses mutable. Do not introduce a separate I/O inheritance path.

Class namespace proxies use a separate class-owned dictionary, not the prepared
class-body Namespace.dictionary. Keep class stores, deletes, and annotation-cache
publication synchronized with retained views through the central mutation methods.
Native namespace descriptors are cached per runtime; publish only implemented
operations, never marker entries that pretend an unsupported protocol exists.
Module/global writes and deletes must use Namespace.store/delete after dictionary
publication. Python dictionary mutation also updates the namespace's name map;
Go lifecycle tests must use the same mutation methods rather than deleting only
from that map.

I/O callbacks run through VM continuations. Keep buffer leases and reentrancy
guards on the calling frame so Go-error unwinding releases them as well as Python
completion. Never call Python from Go GC. Text codec selection and filesystem
entry points must preserve explicit host permissions; a source loader is not a
filesystem provider, and a Python opener does not grant process-file access.
Text decoding must publish a chunk's characters and newline state only after
success. Test failure recovery with split input as well as complete byte strings.

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

Host configuration tests may load Python files from `testdata/host` using focused
Go tests in `runtime_test.go`. These files need caller-supplied providers, so they
do not run as default-runtime execution chunks. Keep Python assertions in those
files and provider call counts, ownership, and isolation assertions in Go.
Private constructor tests may use the runtime package to inspect registration,
circular initialization, and rollback without publishing extension APIs.

Private memory-ownership tests may inspect weak class or buffer-export storage
and force Go GC.
Keep Python-visible behavior in source fixtures; GC timing is a Go-facing contract.
