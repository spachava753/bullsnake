# Bullsnake architecture

Status: living design

This document explains the design Bullsnake is working toward. It focuses on
choices that should remain useful as the code changes. For a description of the
code that exists today, read the [implementation guide](impl.md).

## What Bullsnake is

Bullsnake is an experimental Python interpreter written in Go. Its exact
compatibility reference is CPython 3.14.7 at commit
`823f0323ee6ec1402088b73bce1a38473cac36dc`. Bullsnake implements a selected
subset of that language. Moving the reference requires an intentional update
to this document, the implementation guide, and the checked-in conformance
data.

The project has two long-term goals:

- run a useful set of pure Python packages
- provide a natural way for Go programs to run and extend Python code

Bullsnake does not aim to become a drop-in replacement for CPython. It owns its
compiler, bytecode, object representation, memory model, and eventual Go API.
CPython is a reference for Python behavior, not an implementation template.

## How to judge compatibility

Compatibility is intentionally specific. A feature is compatible when
Bullsnake documents it as supported and its tests agree with the selected
Python 3.14 behavior. Features outside that set have no compatibility promise.

Python behavior is the default for supported features. Bullsnake may differ
when CPython behavior exposes implementation details, depends on reference
counting or the C API, or adds substantial complexity without helping the
selected package set. Any intentional difference should be documented and
covered by tests.

Package compatibility is also specific. A claim should name the package
version, its dependency versions, and the tests that pass. A package written in
Python can still depend on missing standard-library modules, native extensions,
or CPython-only inspection APIs. The label "pure Python" does not guarantee that
it will run.

The first practical package target is source code or an unpacked
`py3-none-any` wheel. Native CPython extensions and platform wheels are outside
the design.

## Design priorities

Bullsnake values, in order:

1. correct behavior for the supported subset
2. code that can be understood and changed
3. useful Go integration
4. broader Python and package compatibility
5. performance work justified by measurement

These priorities lead to a few working rules:

- Reject unsupported behavior clearly instead of approximating it.
- Add features because tests, packages, or the embedding API need them.
- Keep internal formats private while the interpreter is young.
- Use one implementation path for Python and Go-backed values whenever a
  Python protocol is supported.
- Avoid caches, specialization, fine-grained locking, and lifecycle machinery
  until a concrete requirement justifies them.

## System overview

The implemented execution path is:

```text
source bytes
    |
    v
source decoder -> lexer -> parser -> AST -> resolver -> compiler
                                                        |
                                                        v
                                                   code object
                                                        |
                                                        v
                                                validator -> VM
                                                        |
                                                        v
                                                   module values
```

Each step has one job:

- The source decoder turns file bytes into validated UTF-8 source text.
- The lexer turns text into tokens while preserving source positions.
- The parser turns tokens into an abstract syntax tree, or AST.
- The resolver decides what each name means in its surrounding scopes.
- The compiler turns the AST and resolver result into Bullsnake bytecode.
- The runtime validates the complete code object before executing it.
- The virtual machine, or VM, executes bytecode against Bullsnake values.

Import loading, the public Go API, and the coroutine driver already use this
compiler and VM. General async scheduling and Python threads remain later parts
of the design and must extend the same execution path rather than create an
alternate one.

## Why Bullsnake uses bytecode

Bullsnake compiles source into its own stack-based bytecode. The bytecode is an
internal format and is unrelated to CPython `.pyc` files.

Walking the AST directly would be simpler for a very small interpreter, but it
would make calls, suspended functions, exception unwinding, and tracebacks use
several different control-flow mechanisms. Bytecode lets those features share
one execution model.

A stack machine also keeps the first compiler and VM reasonably small.
Bullsnake can reconsider instruction layout after profiling, but code should
not depend on the current opcodes outside the compiler, runtime, and tests that
own them.

## Front end

The front end contains four separate decisions:

1. how source bytes become text
2. how text becomes tokens
3. how tokens become syntax nodes
4. how names and context-sensitive rules are resolved

Keeping these steps separate matters. An encoding error is different from an
indentation error. A malformed expression is different from a valid expression
used in the wrong scope. Each phase reports the error it owns.

The parser accepts more syntax than the compiler can execute. This is
deliberate. It lets Bullsnake build and test the language in stages without
forcing the parser, compiler, and runtime to grow in one large change.

The AST and symbol table are internal data structures. A future Python `ast`
module can expose Python objects through an adapter instead of freezing the Go
representation as a public API.

## Compiler and code objects

The compiler receives an AST and a resolved scope table. It chooses
instructions, constants, local-variable positions, closure positions, jump
targets, and source locations. Class-private names use the resolver scope's
private owner consistently for symbol indexes and attribute spellings.

The result is an immutable code object. A code object contains the information
the VM needs to run one module, function, class body, comprehension, or
annotation body. Child functions and eager comprehensions have child code
objects rather than hidden Go closures.

Lazy method annotations may retain the live class namespace in a closure cell.
Dedicated lookup instructions consult that mapping first and then use either
the annotation function's globals or its enclosing closure cell.

The compiler tracks operand-stack depth while it emits instructions. Every
control-flow path that joins another path must agree on that depth. This catches
compiler mistakes before the VM runs them.

Structural pattern matching also uses explicit stack joins. One retained
subject feeds each case, runtime match instructions return extracted values or
an internal miss marker, and the compiler restores one failure depth before
trying the next case.

Exceptions use protected instruction ranges. A range says where the handler
starts and how much of the operand stack to keep when an instruction raises.
Normal execution pays no setup cost for entering a `try` block. `finally` and
handler-name cleanup are compiler-controlled actions that run before a return,
loop transfer, or propagated exception completes.

Synchronous context managers use those same protected ranges and lexical
cleanup machinery. Entered managers remain visible to return and loop-transfer
compilation, while exceptional exits receive the active exception and can
suppress the pending propagation.

Code objects stay in memory today. A bytecode cache, if one is ever needed,
will require an explicit format version and must reject stale or foreign data.

## Virtual machine and frames

The VM uses explicit Python frames stored on the Go heap. A frame holds the code
being executed, the next instruction, its operand stack, its local variables,
closure cells, namespaces, active exception handlers, and a link to its caller.

Python calls and generator resumptions switch the current frame in one
iterative dispatch loop. They do not use Go recursion as the Python call stack.
This choice has several benefits:

- recursive Python code does not require one Go call per Python frame
- exception unwinding can walk Python frames directly
- tracebacks use the same frame chain as calls
- generators and coroutines keep frames between suspension points

Coroutine calls already allocate detached frames, and awaiting one Bullsnake
coroutine from another uses the same frame switching as an ordinary call. Async
iteration and scheduling remain separate policy layers rather than alternate
execution engines.

Before execution, the runtime validates the entire code tree, including child
functions and unreachable instructions. It checks instruction operands, table
indexes, jump targets, exception ranges, and stack use. Invalid or unsupported
bytecode fails before the module can make changes.

This validation is a boundary between the compiler and runtime. The runtime
must not trust code merely because the current compiler produced it.

## Values and the object model

Runtime values implement a sealed Go interface inside `internal/runtime`.
References between values remain ordinary typed Go pointers or interfaces so
Go's garbage collector can see the complete object graph.

The current object model is intentionally small. It has concrete values for the
scalars, collections, functions, classes, modules, and exceptions needed by the
executable subset. More of Python's data model will be added when language
features or packages require it.

A future Go-defined type must use the same attribute, call, iteration, equality,
and exception paths as a Python-defined type. A smaller second object model for
native values would create two subtly different languages.

Python containers cannot be represented as plain Go maps or slices forever.
Hashing, equality, attribute access, and descriptors may call Python code or
raise exceptions. Container and object implementations must support those calls
when the protocols are added.

The initial descriptor slice represents `classmethod`, `staticmethod`, and
`property` as explicit runtime values. Attribute loading performs their binding
and can suspend into a Python property getter through the ordinary frame call
path.

## Exceptions

Python exceptions are runtime values, not Go errors. An instruction can
advance, call another Python frame, return, or raise. Go errors are reserved for
invalid bytecode and host-level failures.

The VM finds an exception handler from the protected ranges in the current code
object. If no range applies, it moves to the caller frame. An uncaught exception
crosses the current Go boundary with its Python traceback information.

Handled exceptions remain available for bare `raise`. New exceptions record
implicit context, and `raise ... from ...` records an explicit cause. Exception
groups use the same unwind path. `except*` splits a group, runs every matching
clause, and then recombines unmatched or newly raised exceptions.

Python traceback objects and broad frame inspection are separate features. The
runtime currently keeps only the information needed for host-facing tracebacks
and future expansion.

## Runtime instances and imports

Mutable interpreter state belongs to a `Runtime`. Today that includes builtins,
prepared code, and the module cache, including modules whose bodies are still
initializing. The direction is for scheduler state, thread state,
configuration, and Go module registration to belong to the same runtime
instance.

This ownership makes isolation explicit. Separate runtimes may execute in
parallel without silently sharing modules or mutable Python values.

The current importer asks a host-supplied loader for a module description with
immutable code and package metadata. For a dotted absolute name, the runtime
loads each parent first, verifies that it is a package, and publishes each child
on that parent. Relative from-imports resolve their level against the executing
module's package name. A from-import also tries a missing package attribute as a
child module. Wildcard imports honor an explicit `__all__` list and load listed
package children. Modules execute in the existing frame loop. Repeated imports
reuse one object. Because the cache entry exists before execution, circular
see the names assigned so far. If execution fails, the runtime removes only
that module; dependencies that finished successfully remain cached.

The filesystem loader searches configured roots for top-level modules, regular
packages, and namespace packages. Within one location it prefers
`name/__init__.py` over `name.py`; otherwise it combines matching namespace
directories. Child lookup uses the parent package's recorded search locations
rather than restarting at global roots. The loader decodes and compiles files
outside the runtime. Source modules and statically linked Go modules should
enter through the same runtime loading path. Advanced `importlib` hooks, zip
imports, reload, and bytecode caches should be added only when package tests
require their observable behavior.

Go-backed system modules use the same runtime cache and Python `Module` values
as source modules. `sys.modules` observes cache insertion, successful import,
and rollback through a Python dictionary. Python-side replacement or deletion
in that dictionary does not yet change the runtime's authoritative cache.

Test-framework compatibility uses CPython's pinned pure-Python `Lib/unittest`
package through the ordinary source loader. Bullsnake does not substitute a
Go-backed runner, so the same compiler, VM, object model, imports, and host
capability boundary used by application code must support its dependencies.
The 535 core `test.test_unittest` tests and all 559 tests in its `testmock`
package pass at the pinned revision, including discovery, command-line,
interrupt, isolated-asyncio, patching, autospec, magic-method, and async-mock
coverage.

## Host capability boundary

Interpreter code must not call operating-system, clock, network, process, or
standard-stream APIs directly. The public `host` package groups those powers as
independent interfaces in `host.Services`. A nil service denies that capability.
`bullsnake.New` preserves exactly the supplied services, while
`bullsnake.NewDefault` is the explicit convenience path that grants current
process capabilities.

The first boundary contains:

- a read-only filesystem for source reads, directory listing, metadata, and
  real-path resolution, plus a separately grantable filesystem mutation
  capability
- wall time, monotonic time, and sleeping through one clock, with local civil
  time conversion through a separate time-zone capability
- cryptographic entropy through an independently replaceable entropy source
- outbound dialing through a network interface, reserved for a future socket
  module
- arguments, environment, executable, and working-directory process data
- independently replaceable standard input, output, and error streams

Interfaces grow by adding separate capabilities, not by adding unrelated
methods to an existing interface. Filesystem mutation uses `FileMutator`, so
read-only mocks remain valid. Policy wrappers can record,
rewrite, or reject an operation with `host.ErrDenied`; a system module converts
that denial to `PermissionError`. Source-loader failures remain Go errors at the
embedding boundary.

## Go embedding and extensions

The first public Go API creates an isolated interpreter from explicit host
services and module search roots, executes source or a host-read file, inspects
module identity, and reads stable value metadata. Its intended completed shape
remains small:

- create an isolated runtime
- compile or execute source
- import a module
- exchange supported values
- call Python functions
- expose Go functions, types, and modules

The public value view exposes only type names and representations. It does not
expose internal code objects, frames, namespaces, or object layouts. A future
extension API will let Go modules register explicitly with an interpreter or
builder. Static linking is the first distribution model.

A Go callback may call back into Python only through an execution context owned
by the runtime. Background goroutines may finish host work and post a result,
but they must not mutate Python objects directly.

## Async and Python threads

Coroutine execution is implemented with suspended Python frames, direct
`await` between Bullsnake coroutines, asynchronous iteration and
comprehensions, and a small interpreter-owned root driver.
The `asyncio` and `contextvars` bootstrap surface is deliberately limited to
what CPython's `IsolatedAsyncioTestCase` and async-mock tests exercise; it is
not a general event loop and performs no ambient socket I/O.

A general event loop remains a design direction. It will manage ready tasks,
timers, host-mediated I/O completion, cancellation, and task context. Async
tasks will not be modeled as one goroutine each because Python task scheduling
and cancellation need explicit interpreter state. Python threads remain future
work; the synchronization objects needed by `ThreadingMock` are
interpreter-local compatibility primitives.

The intended threading model maps each supported Python thread to one Go
goroutine. One runtime execution token will initially allow only one such thread
to execute Python code or mutate Python objects at a time. A thread can release
the token while waiting on I/O or synchronization. Separate runtimes can still
execute in parallel.

This model uses Go's cheap goroutines without requiring the whole object model
to be thread-safe. Same-runtime parallel bytecode execution would require
locking rules throughout containers, classes, imports, extensions, and frame
inspection. That cost should be justified by real workloads before it is added.

## Memory and lifecycle

Bullsnake relies on Go's tracing garbage collector for Python object memory. It
does not maintain CPython reference counts or a second cycle detector.

The rule is simple: every live Python reference must remain visible to Go's
collector. The runtime must not hide references in integers, unsafe pointers,
or untracked packed storage.

The probe in [`experiments/gcprobe`](../experiments/gcprobe/) confirmed that Go
can reclaim ordinary cycles represented through interfaces. It also confirmed
that finalizers and cleanup callbacks have timing and cycle limitations that do
not fit normal Python finalization semantics.

The baseline therefore omits `__del__` and a CPython-compatible `gc` module.
External resources should use explicit `close()` methods and Go-side lifecycle
APIs. Python context managers can join that path when the VM implements them.
Weak references may be added if package tests need them, but Python callbacks
would run later at a safe VM point, never on a Go cleanup goroutine.

## Rules the implementation must preserve

These rules summarize the design:

1. Supported behavior and intentional differences are documented and tested.
2. Unsupported behavior fails clearly at the earliest phase that owns it.
3. Code is fully validated before execution can produce side effects.
4. Python calls, returns, suspension, and exceptions use explicit Python frames.
5. Python exceptions travel through VM outcomes rather than ordinary Go errors.
6. Every live Python reference remains visible to Go's collector.
7. Mutable modules, caches, scheduling, and thread state belong to one runtime.
8. Python-defined and Go-defined values share supported object protocols.
9. Internal AST, bytecode, frame, and object layouts remain private.
10. New complexity must be justified by a supported feature, package, or
    measured workload.
11. Every host interaction crosses an explicit replaceable capability.

## Repository boundaries

The current high-level layout is:

```text
bullsnake.go               minimal public embedding API
host/                      replaceable host capability contracts and defaults
internal/compiler/          source-to-code pipeline
internal/compiler/source/   source decoding
internal/compiler/lexer/    tokens
internal/compiler/ast/      syntax tree
internal/compiler/parser/   grammar
internal/compiler/resolver/ scopes and name meaning
internal/compiler/bytecode/ immutable code objects
internal/importer/          source module discovery and compilation
internal/runtime/           values, validation, frames, VM, and current imports
experiments/                disposable design probes
```

The compiler depends on syntax and resolution, but not on the runtime. The
runtime consumes immutable code objects, but it does not parse source. The
importer is a composition package: it depends on the compiler pipeline and the
runtime's loader contract, while neither core package depends on it. The public
root package configures these pieces without exposing their internal types.

The runtime should remain one coarse internal package until imports, scheduling,
or builtins have an independent API or dependency reason to split.

## Verification

Bullsnake uses several kinds of evidence:

- focused Go tests for package behavior and internal invariants
- checked-in Python source fixtures that pass through the complete pipeline
- pinned CPython source files promoted to execution tests as their dependencies
  become supported
- negative fixtures for Python errors and unsupported behavior
- direct malformed-bytecode tests for the runtime validator
- CPython-derived conformance cases for selected behavior
- fuzz tests for parsers and other input boundaries
- future package tests against pinned package and dependency versions

Comparisons with CPython should cover behavior Bullsnake intends to share.
Object IDs, hash values, finalization timing, private frames, exact error text,
and Bullsnake bytecode should be compared only when the documented contract
promises them.

## Open decisions

The project still needs concrete decisions about:

- the first package compatibility set
- which transitive standard-library dependencies need host-backed modules
- the Go callback, type, and module extension API
- namespace-package and extended import-hook behavior
- generator, coroutine, and scheduler behavior
- the first Python threading subset and execution-token policy
- whether weak references are needed
- which frame inspection features packages actually require

These decisions should be made from working code and tests rather than by
trying to reproduce every CPython feature in advance.

## References

- [Python 3.14 language reference](https://docs.python.org/3.14/reference/)
- [Python 3.14 data model](https://docs.python.org/3.14/reference/datamodel.html)
- [Python 3.14 execution model](https://docs.python.org/3.14/reference/executionmodel.html)
- [Python 3.14 import system](https://docs.python.org/3.14/reference/import.html)
- [Go garbage collector guide](https://go.dev/doc/gc-guide)
- [Go memory model](https://go.dev/ref/mem)
