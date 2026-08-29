# Bullsnake runtime design

Status: Living design

Last updated: 2026-08-27

## Purpose

Bullsnake is a Python interpreter implemented in Go. It deliberately targets a
selected subset of Python, with the goal of running a useful and growing corpus
of pure Python packages while remaining easy to embed and extend from Go.

Python 3.14 is the source-language reference, not a promise to implement every
Python 3.14 feature or reproduce every CPython behavior.
Bullsnake defines a smaller, documented language and runtime contract. CPython
is a useful comparison implementation for features Bullsnake chooses to
support.

Bullsnake owns its compiler, bytecode, object representation, memory model, and
extension API. CPython bytecode, reference counting, runtime inspection APIs,
and C extension machinery do not constrain the design.

The implementation uses a small stack-based bytecode virtual machine with
explicit Python frames stored on the Go heap. That design costs more than a
tree-walking interpreter at the first milestone, but it gives ordinary calls,
generators, coroutines, and tracebacks one execution model. Go goroutines
provide the separate lightweight-thread mechanism.

Correctness means that implemented features obey Bullsnake's documented
semantics consistently. Python-compatible behavior is the default when it is
clear and useful. Deliberate, documented differences are acceptable when they
remove a common pitfall, avoid CPython-specific behavior, or keep the
implementation substantially simpler.

## Decisions

These decisions guide the implementation:

1. Use Python 3.14 as the reference version for syntax and selected runtime
   behavior. Maintain an explicit feature manifest instead of claiming full
   Python 3.14 compatibility.
2. Prefer familiar Python semantics for supported features. Permit deliberate,
   documented divergences when they avoid a pitfall or remove complexity that
   does not serve the package corpus.
3. Reject unsupported syntax and operations clearly. Silent approximations are
   harder for users to diagnose than an explicit unsupported-feature error.
4. Compile Python source to Bullsnake-owned bytecode. Do not consume or emit
   CPython `.pyc` files as a compatibility format.
5. Use an iterative, stack-based VM. Keep Python frames and frame chains as
   explicit heap objects instead of mirroring Python calls on the Go call
   stack.
6. Map each supported Python `threading.Thread` to one Go goroutine with an
   explicit Bullsnake thread state. Use a per-runtime execution token in the
   baseline so only one thread mutates Python objects at a time. Independent
   runtimes may execute in parallel.
7. Rely on Go's tracing garbage collector for memory reclamation unless an
   experiment demonstrates that a selected feature requires more machinery.
   Do not build a second collector merely to resemble CPython.
8. Implement the selected generator, coroutine, awaitable, and asynchronous
   iterator protocols in the VM. Keep event-loop policy in a scheduler layer.
9. Use goroutines directly for Python threads and blocking host work. Keep
   Python async tasks as explicit scheduler objects because their await,
   cancellation, context, and ordering semantics differ from threads.
10. Expose an explicit, runtime-local Go module registry. Go extensions are
    compiled into the embedding program or Bullsnake binary.
11. Treat goroutine-backed Python threads as the baseline lightweight-thread
    model. Defer Stackless-style tasklets because they require interpreter-
    controlled suspension and scheduling beyond what Go exposes for a
    goroutine.
12. Measure compatibility with a versioned package corpus and upstream package
    tests. Compatibility claims apply to named features and package versions.

## Goals

Bullsnake should provide these capabilities as the selected subset grows:

- Parse and execute a documented subset derived from Python 3.14.
- Produce clear errors for unsupported syntax, modules, and operations.
- Implement enough of Python's object model for the chosen language features
  and package corpus, without treating every data-model hook as mandatory.
- Import source modules, regular packages, and runtime-provided Go modules
  through one coherent import path.
- Run useful package versions distributed as source trees or `py3-none-any`
  wheels.
- Provide the built-ins and standard-library modules required by that package
  corpus.
- Support a useful async subset, including `async def`, `await`, tasks,
  cancellation, and task-local context when those features enter the manifest.
- Let a Go program create isolated runtimes, execute Python, exchange values,
  call Python functions, and expose Go functions and types to Python.
- Let Go extensions call back into Python and provide awaitable operations
  without mutating VM state from arbitrary goroutines.
- Preserve enough frame and source information for actionable errors and
  tracebacks. Rich CPython-style runtime inspection is optional.
- Support a useful `threading` subset by mapping each Python thread to a Go
  goroutine and providing Bullsnake-owned thread identity, local state, joins,
  and synchronization primitives.
- Preserve the option to add interpreter-controlled tasklets later without
  making them part of the initial concurrency model.
- Keep each component understandable and replaceable. Add optimization or
  compatibility machinery only after a supported feature requires it.

## Non-goals

The baseline design does not include these capabilities:

- Full Python 3.14 language, data-model, built-in, or standard-library
  compatibility.
- The CPython C API, limited C API, stable ABI, or loading CPython extension
  modules.
- Compatibility with CPython bytecode, `.pyc` contents, code object layout, or
  memory addresses.
- CPython-specific inspection modules such as `dis`.
- A CPython-compatible `gc` module, reference counts, collection generations,
  or user control over Go's collector.
- Platform wheels containing native extensions, including `abi3` wheels.
- Exact CPython error text, finalization timing, object interning, hash values,
  or other behavior outside Bullsnake's feature contract.
- A JIT compiler, CPython-level performance, or speculative specialization.
- Free-threaded parallel execution of Python bytecode inside one shared runtime
  in the baseline. Goroutine-backed threads initially serialize Python object
  access through a runtime execution token.
- Dynamic loading of arbitrary compiled Go plugins. Static linking is the
  portable first model for Go extensions.
- A security sandbox. Python code and Go extensions initially run with the
  host process's authority.
- Running `pip` or arbitrary source-distribution build backends in the first
  package-compatibility milestone.

## Design values and divergence policy

Simplicity, extensibility, and correctness are the design priorities. Raw
interpreter speed and completeness rank below them.

A proposed feature should pass these questions before it enters the supported
subset:

1. Does the package corpus or embedding API need it?
2. Can users understand and test its Bullsnake semantics?
3. Does it fit the existing architecture without special-case machinery?
4. Is Python compatibility worth the implementation and maintenance cost?
5. Would a smaller or safer behavior avoid a known Python pitfall?

Python behavior remains the starting point because it makes the language
familiar and lets packages run with fewer changes. Bullsnake may diverge when a
behavior exposes CPython internals, depends on reference counting, creates a
common correctness trap, or adds substantial complexity for little practical
value.

Every intentional divergence must be visible in the feature manifest,
documented in user-facing behavior, and covered by tests. Unsupported features
should fail at parse time, import time, or the narrowest practical runtime
boundary. Bullsnake must not silently accept a feature and produce subtly
incorrect results.

Examples of acceptable omissions include `dis`, `gc`, reference-count APIs,
CPython bytecode caches, C extension loading, and exact finalizer timing. Other
divergences should be decided from concrete use cases rather than a general
desire to redesign Python.

## Defining compatibility

### Language compatibility

Bullsnake uses Python 3.14 as the grammar and semantic reference for its
selected subset. A feature manifest records supported statements, expressions,
built-ins, protocols, and intentional differences. Programs outside that
manifest have no compatibility promise.

The reference version is still an architectural input. Grammar, AST nodes,
annotation behavior, code metadata, and built-ins change across minor versions.
Bullsnake must identify its baseline as a Python 3.14-derived subset instead of
claiming unqualified "Python 3" compatibility.

### Runtime compatibility

Packages depend on behavior beyond grammar. The possible runtime surface
includes built-in types, descriptors, exceptions, imports, modules, I/O,
encodings, paths, time, networking, and selected introspection. Bullsnake
implements only the portions named in its feature manifest or required by its
package corpus.

CPython behavior is a differential-testing reference for those selected
features. CPython implementation details remain excluded unless Bullsnake
adopts one deliberately. Exact error wording, private attributes, bytecode,
frame internals, collection controls, and shutdown timing receive no implicit
compatibility promise.

### Pure Python package compatibility

The first concrete distribution target is an unpacked source tree or a wheel
tagged `py3-none-any`. That wheel tag means the artifact has no Python ABI or
platform dependency. It does not guarantee that the package will run on every
interpreter.

A package remains incompatible when any part of its dependency closure:

- Requires a native extension
- Uses `ctypes` or another foreign-function mechanism
- Checks for CPython and depends on CPython internals
- Depends on a standard-library module Bullsnake has not implemented
- Relies on unsupported introspection, threading, signals, subprocesses, or
  finalization behavior
- Requires a build backend that Bullsnake cannot execute

Package compatibility must therefore be reported against named package
versions and their tests. The useful unit is a tested dependency closure, not
a count of syntax features.

Installing wheels and executing packages are separate concerns. Bullsnake can
initially consume an externally unpacked pure wheel through `sys.path`. A
Bullsnake-native installer and support for packaging tools can follow later.

## Runtime invariants

The implementation should preserve these invariants from its first executable
slice:

1. **The reference version and subset are explicit.** Parser behavior, runtime
   behavior, tests, and documentation all refer to a Python 3.14-derived
   feature manifest.
2. **Bullsnake's documented behavior is the compatibility boundary.** Python
   behavior is the default for supported features. Intentional differences and
   omissions are part of the contract rather than hidden test exceptions.
3. **Unsupported behavior fails clearly.** The parser, importer, or runtime
   rejects behavior outside the manifest at the narrowest practical boundary.
4. **Every supported Python value has stable identity, type, and value
   semantics.** The implementation cannot use a movable or reusable Go address
   as the public definition of `id()` if `id()` is supported.
5. **Go's collector can see every live Python reference.** Python references
   remain in typed Go pointers or interfaces. Optimizations must not hide the
   object graph in `uintptr`, unsafe pointers, or untraced packed values.
6. **Python calls do not require Go recursion.** Calling a Python function
   pushes an explicit frame onto a Python task and returns control to the same
   VM dispatch loop.
7. **Suspension is explicit.** A generator, coroutine, asynchronous generator,
   or future interpreter-controlled tasklet owns all state needed to resume
   without reconstructing a Go call stack.
8. **Python exceptions are Python values.** Expected language errors propagate
   through VM unwind state. Go panics indicate interpreter defects or truly
   unrecoverable host failures.
9. **Selected object protocols use one semantic path.** Python-defined and
   Go-defined objects follow the same rules for every protocol Bullsnake
   supports.
10. **Protocol calls may re-enter Python.** Attribute lookup, descriptors,
   equality, hashing, and other apparently small operations may execute Python
   code or raise exceptions. Runtime code cannot assume these operations are
   leaf calls.
11. **A runtime execution token has one owner.** Each Python thread has its own
    goroutine and thread state, but it acquires the runtime token before
    executing bytecode or touching Python objects. It releases the token while
    waiting on blocking I/O or synchronization. Bullsnake assigns logical
    thread IDs and stores thread-local state explicitly; neither depends on an
    OS thread or an unavailable Go goroutine ID.
12. **Runtime instances isolate mutable state.** Modules, built-ins, import
    paths, scheduler state, exception state, and extension registration belong
    to an instance. Process-global mutable Python state is avoided.
13. **Import creates one module identity per cached name.** A module enters the
    runtime's module cache before its body executes, which permits circular
    imports and requires cleanup when loading fails.
14. **Python code never runs on a Go finalizer or cleanup goroutine.** If a
    selected feature uses lifecycle notifications, the runtime queues and
    handles them later at a safe point.
15. **Go extensions use public contracts.** An extension cannot import
    internal compiler, object-layout, or VM packages.
16. **Logical execution state is independent of the Go stack.** Recursion
    limits and tracebacks, when supported, follow logical Python frames.
    Richer tracing, profiling, and frame inspection are optional features.
17. **Compatibility is evidence-based.** Each supported feature has semantic
    tests, and each supported package has a pinned version and passing test
    result.
18. **Complexity must be earned.** Compatibility shims, lifecycle machinery,
    optimizations, and public APIs enter the design only for a selected feature
    or measured package need.

## Architecture decision

Four implementation styles are plausible:

| Option | Assessment |
| --- | --- |
| Port CPython internals | Retains architecture for reference counting, the C ABI, and CPython bytecode that Bullsnake does not need. It also makes Go integration awkward. |
| Walk the AST directly | Useful for a small prototype. Resumable functions, exception unwinding, frames, tracing, and repeated evaluation become harder as compatibility grows. |
| Transpile Python to Go | Conflicts with `eval`, `exec`, dynamic classes, frame inspection, fast startup, and runtime compilation. Go compilation also becomes part of normal execution. |
| Compile to Bullsnake bytecode | Separates syntax from execution, supports explicit frames, permits interpreter-specific instructions, and leaves room for later optimization. |

Bullsnake uses its own bytecode VM. It borrows proven compiler stages from
CPython and other interpreters without copying CPython's bytecode or memory
architecture.

The initial VM is a stack machine. Python expressions map naturally to an
operand stack, the compiler stays small, and bytecode is easy to inspect in
tests. A register VM may reduce dispatch and copying later, but that is a
performance decision to make from profiles.

## Simplicity rules

The architecture should stay small in code as well as in diagrams:

- Use one source-to-code path, one object protocol, one VM, and one scheduler.
- Prefer concrete internal types and direct calls. Add an interface only when
  there are multiple implementations or a real host or extension boundary.
- Keep internal packages coarse at first. Split a package when dependency
  direction, ownership, or independent testing makes the boundary useful.
- Keep bytecode private and small. Add instructions to express selected
  semantics, not to mirror CPython or anticipate optimizations.
- Avoid per-object locks, reference counts, finalizers, and instrumentation in
  the baseline.
- Keep unsupported behavior out of the implementation instead of adding
  compatibility placeholders that appear to work.
- Stabilize only the Go embedding and extension API. Internal compiler and VM
  structures should remain easy to change.
- Add caches, specialization, alternate execution engines, and parallelism only
  from measured need.

A small bytecode VM remains consistent with these rules. Async and resumable
execution require explicit continuation state somewhere. Keeping that state in
one frame representation is simpler over the project lifetime than adding
separate control-flow mechanisms to an AST walker.

## System overview

```text
Python source bytes -> source loader -> UTF-8 source
UTF-8 source -> lexer -> parser -> AST -> symbol table -> compiler -> code object
code object -> VM

Go host -> runtime -> thread registry -> goroutine -> ThreadState -> VM
              |                              |             |         |
              |                              |             v         v
              |                              |         frame stack  object model
              |                              |
              |                              +-> async tasks and contexts
              |
              +-> import system -> source and Go modules
              +-> timers, I/O poller, and completion queue
```

The runtime composes the components and owns the execution token shared by its
Python thread goroutines. The compiler does not know about active runtime
instances. The VM does not know how source files are found. Native Go modules
enter through the import system instead of receiving special import opcodes.

## Front end

Source loading is separate from the compiler front end. It owns file and reader
I/O, BOM and coding-cookie detection, and decoding source bytes to UTF-8. The
front end receives decoded source and has four phases. The implemented lexer
contract and its CPython reference revision are recorded in the
[implementation notes](impl.md).

1. Tokenize decoded UTF-8 while retaining byte offsets, line and column
   positions, comments needed for diagnostics and source tooling, and
   indentation state.
2. Parse tokens into an internal `Module` AST with complete source spans.
3. Build a symbol table that classifies locals, globals, nonlocals, closure
   cells, free variables, comprehensions, annotation scopes, generators, and
   coroutines.
4. Validate context-sensitive syntax before bytecode generation.

The parser recognizes only the selected grammar. It is hand-written recursive
descent that constructs AST nodes directly. Ordinary binary operators use
precedence climbing; Python-specific forms use dedicated rules. A lazy buffered
token cursor supports local rewinds for contextual ambiguities without a PEG
runtime, generated parser, or general memoization.

Every source unit parses to one `Module` containing statements. Eval requires
one expression statement and preserves its value; a REPL displays
expression-statement values and executes other statements normally. These are
compiler and execution policies rather than parser modes. Parser and lexer
errors mark whether reaching the end of source left a construct incomplete,
which lets a REPL request more input without changing the grammar.

The internal AST should not become a stable Go extension API early. Python's
`ast` module can expose Python objects through an adapter, which leaves room to
change compiler data structures.

## Compiler and bytecode

The compiler turns an AST plus symbol table into immutable code objects. A
code object should contain:

- Bullsnake bytecode and constants
- Local, cell, free, global, and referenced names
- Argument shape and function flags
- Source filename, qualified name, and first line
- Instruction-to-source-position data
- Exception and cleanup regions
- Stack-size metadata if the VM needs it

Bullsnake bytecode is private and versioned. Cached compiled files, if added,
need a Bullsnake magic value, language version, bytecode version, source hash,
and implementation cache tag. The runtime must reject stale or foreign cache
files.

The first instruction set should favor correctness and clarity. Specialized
opcodes, inline caches, superinstructions, and adaptive optimization can be
added after semantic conformance and profiling.

## Virtual machine and frames

The first VM slice executes module code with a heap-allocated frame. A frame
contains prepared immutable code, the next instruction index, an operand
stack, local, global, and builtin namespaces, and a link to its logical caller.
A thread state points to the active frame. The iterative dispatcher handles
normal progress, return, and Python exception outcomes without using a Go call
as the definition of a Python frame.

Before execution, the runtime copies the code tables it consumes, materializes
compiler constants as runtime values, and validates every instruction,
operand, table index, jump target, and reachable stack transition. A worklist
requires all control-flow edges into an instruction to agree on stack depth.
`FOR_ITER` has separate yield and exhaustion depths because it retains the
iterator and pushes an item only on the yield edge. Unsupported or malformed
bytecode fails before the module body can produce side effects. Prepared code
and its materialized constants are cached per runtime and immutable code-object
identity.

The frame and dispatcher are intentionally shaped for later calls and
suspension even though the first slice executes only modules. A Python call
will replace the active frame with a child whose `previous` link names the
caller. Return will restore that caller and push the result. A suspended async
task or generator will own the same frame state needed to resume it.

As more execution forms enter the supported subset, the VM must add outcomes
for calls, yield, await suspension, exception propagation through handlers,
scheduler safe points, host cancellation, and execution-budget stops. A call
into a synchronous Go extension will run on the Python thread's current Go
stack, but a Python-to-Python call will remain in the iterative dispatcher.

## Object model

The runtime has a sealed internal `Value` interface. Immutable singleton
objects represent `None`, both booleans, and ellipsis. Heap-backed objects
represent arbitrary-precision integers, binary64 floats, complex numbers,
strings, bytes, fixed tuples and lists, dictionaries, sets, slices, collection
iterators, and Python exceptions. Every live reference remains in a typed
pointer or interface visible to Go's collector. Module bindings use a temporary
string-keyed namespace; Python dictionaries use their own value type and
insertion-ordered entries.

The current object operations cover fixed scalar truth, numeric unary
operators, selected arbitrary-precision integer binary operators, scalar and
tuple equality, object identity, fixed and starred tuple/list construction and
unpacking, tuple/list/dictionary/set/text/bytes iteration, tuple/list/text/bytes
subscription, fixed and unpacked dictionary displays, fixed and starred set
displays, dictionary subscription and item mutation, and tuple/list/dict/set/text/bytes
membership. Dictionary key and set element matching are linear until
user-defined hash and equality protocols justify hash tables. Dictionary
iterators detect key insertion and deletion while allowing value replacement.
Text indexes count decoded Python code points over UTF-8/WTF-8 storage; bytes
indexes count raw bytes.
Later user-defined protocols must reuse the VM's outcome and exception paths.

The object model can become the largest compatibility component, so it should
remain feature-driven. It should be designed before a large instruction set
because most bytecodes delegate to it, but Bullsnake does not need to implement
every Python hook before executing useful programs.

The candidate model includes these capabilities as the subset requires them:

- `object` and `type` bootstrapping
- Single and multiple inheritance with Python's method resolution order
- Instance dictionaries and slots
- `__getattribute__`, `__getattr__`, `__setattr__`, and `__delattr__`
- Data and non-data descriptors
- Bound methods, class methods, static methods, and properties
- Metaclass selection and class creation hooks
- Special-method lookup rules
- Callable, iterator, context-manager, numeric, mapping, and sequence protocols
- Arbitrary-precision integers, Python float behavior, Unicode strings, bytes,
  tuples, lists, dictionaries, sets, and their immutable variants
- Stable singletons and explicit handling for `None`, `NotImplemented`, and
  `Ellipsis`

A Python dictionary cannot be a thin `map[Value]Value`. Python hashing and
equality can execute user code, mutate state, and raise exceptions. Dictionaries
also preserve insertion order and must handle hash collisions according to
Python semantics. A Bullsnake dictionary may use Go maps internally for hash
buckets, but it needs its own protocol-aware implementation.

All native types exposed by Go extensions must participate in this same model.
A separate, reduced native-object path would create two subtly different
languages.

## Runtime instances

The initial `Runtime` owns a prepared-code cache, a builtin namespace, and
successfully executed modules. Module execution creates a fresh namespace and
publishes the module in the runtime cache only after normal return. Mutable
state is instance-local; only the immutable `None` singleton is currently
shared process-wide.

As the supported subset grows, a runtime instance will also own:

- The built-in namespace and implementation-specific `sys` state
- `sys.modules`, import paths, finders, loaders, and import locks
- Standard input, output, and error streams
- The Go module registry
- The execution token, Python thread registry, and logical thread IDs
- Shared timer, I/O, and completion services plus registered thread-owned
  event loops
- Pending weak-reference or finalization notifications if those features are
  enabled
- Configuration, environment, and host service adapters
- Interning tables and other mutable caches

`sys.implementation.name` should identify Bullsnake. Packages can then make an
explicit portability decision instead of mistaking Bullsnake for CPython.

A host may construct many isolated runtimes. Each Python thread has a
goroutine, while the runtime execution token serializes access to that
runtime's Python objects. Separate runtimes can execute in parallel. Immutable
process-wide metadata may be shared. Mutable Python objects and module state
may not be shared implicitly between runtimes.

## Import system

Imports are a runtime service, but the baseline does not need the complete
`importlib` protocol. The smallest useful design supports:

- A runtime-local module cache checked before loading
- Absolute and relative names
- Source modules and regular packages on configured filesystem paths
- Built-in and statically linked Go extension modules
- Cache insertion before module execution for circular imports
- Removal of a newly inserted module when its execution fails

The same internal loader path should create source and Go-backed module
objects. Python code should not need a special import statement for a Go
module.

Namespace packages, module specs, meta-path hooks, path-entry hooks, reload,
zip imports, bytecode caches, and a Python implementation of `importlib` are
optional features. Add each one only when the package corpus requires its
observable behavior. The bootstrap importer can remain a concrete Go
component indefinitely.

## Standard library strategy

Pure Python package support depends heavily on the standard library. A package
written entirely in Python may import `math`, `_io`, `socket`, `select`, `ssl`,
`time`, or other modules implemented partly in C by CPython.

Bullsnake should split the standard library into two layers:

1. Core and platform modules implemented in Go expose primitives for built-ins,
   I/O, files, time, operating-system access, networking, selectors, codecs,
   context variables, and other host services.
2. Python modules provide higher-level behavior and should reuse portable
   upstream Python code where licensing and coupling to CPython internals allow
   it.

The exact module order should follow the selected package corpus. Implementing
modules because CPython ships them is less useful than satisfying measured
package dependency graphs.

Vendoring CPython grammar files, tests, or standard-library source requires a
license review and preservation of the Python Software Foundation license and
notices. Bullsnake's MIT license does not erase upstream terms.

## Go embedding and extensions

The public Go API should have two layers:

- A high-level Bullsnake package for constructing a runtime, compiling or
  executing source, importing modules, calling values, and controlling
  lifecycle.
- A low-level leaf package for Python values, native functions, module
  definitions, exceptions, and execution-context contracts. This package must
  not import the compiler or VM.

Go modules should register explicitly with a runtime builder or runtime
instance. Global registration through package `init()` may be offered as a
convenience later, but it should not be the only model because it weakens
runtime isolation and test control.

The extension contract needs to support:

- Defining module constants, functions, exceptions, and types
- Converting arguments with Python-compatible errors
- Returning Python values or raising Python exceptions
- Calling Python values through an execution context
- Holding Python values in GC-visible Go fields
- Releasing host resources deterministically
- Starting host work and returning a Bullsnake future or awaitable
- Posting completion back to the owning runtime from another goroutine

A synchronous native function runs on the calling Python thread's goroutine
while that thread owns the runtime execution token. The extension API needs a
structured way to release the token around blocking Go work and reacquire it
before returning to Python. A background goroutine may complete a future or
submit a message, but it may not touch Python objects directly.

Go errors and Python exceptions should remain distinct. The API can translate a
Go error deliberately, but an arbitrary error must not leak into Python as an
implementation-shaped value.

The first distribution model is static linking. An application imports a Go
extension package, registers its module provider, and builds one binary. Go's
platform-limited plugin mechanism is a separate future decision.

## Async and scheduling

Python language-level async support and `asyncio` compatibility are separate
requirements. Bullsnake may implement a useful async subset without promising
that the complete upstream `asyncio` package runs unchanged.

For async features in the manifest, the VM supplies the language mechanisms:

- Calling `async def` creates a coroutine object without running its body
- `await` follows the `__await__` iterator protocol
- Tasks can send values or inject exceptions into suspended coroutines
- Cancellation is delivered as a Python exception
- `async for` and `async with` use their asynchronous protocols
- Asynchronous generators support iteration, sending, throwing, and closing
- Each task carries its `contextvars` context

The scheduler supplies policy and host integration:

- A ready queue
- Timer management
- Future completion and callbacks
- I/O readiness
- Thread-safe submission from Go goroutines
- Task cancellation and shutdown
- Safe points for pending callbacks, signals, lifecycle events, and host
  cancellation

An event loop belongs to one Python `ThreadState` and runs one Python task at a
time on that thread's goroutine. Awaiting a pending future returns control to
the event loop. Awaiting a coroutine may continue directly through its frame
chain, preserving Python's await protocol rather than forcing a goroutine
switch. Supporting an event loop in several Python threads can be added later
without mapping each async task to another goroutine. When an event loop has no
ready task and waits for I/O or a timer, its Python thread releases the runtime
execution token.

Go goroutines are useful for host operations that cannot use nonblocking I/O.
They should send immutable results or completion messages back to the runtime.
They must not resume a Python frame themselves.

Full compatibility with CPython's `asyncio` package requires much more than
`async` syntax. It also requires futures, tasks, context variables, selectors,
sockets, subprocess and signal behavior where supported, thread bridges, and
many introspection details. Bullsnake should first implement the language
protocols and a small event loop, then test whether upstream `asyncio` can run
against the available platform modules.

## Goroutines, green threads, and Stackless behavior

A goroutine is a green thread in the broad systems sense. Go multiplexes many
goroutines over operating-system threads, starts them with small growing
stacks, preempts them, and keeps other goroutines runnable when one blocks.
That makes one goroutine per Python `threading.Thread` a natural Bullsnake
mapping.

This model provides:

- Cheap creation compared with one operating-system thread per Python thread
- Concurrent blocking I/O without a Bullsnake event loop
- Go scheduler load balancing and preemption
- CPU parallelism up to `GOMAXPROCS` for independent runtimes and detached Go
  work
- A preserved synchronous programming model for Python code

Bullsnake still needs its own Python thread abstraction. Go exposes no
goroutine ID or goroutine-local storage, and a goroutine may move between OS
threads. Each Python thread therefore owns a Bullsnake `ThreadState` containing
its logical ID, name, frame stack, exception state, context variables, current
async event-loop state, thread locals, daemon flag, and lifecycle state. The
runtime maintains the registry needed for `current_thread()`, `enumerate()`,
and `join()`.

`runtime.LockOSThread()` is reserved for a Go extension that calls a genuinely
thread-affine OS or native API. Using it for ordinary Python threads would
throw away much of the M:N scheduling benefit.

Goroutine-backed threads do not automatically implement Stackless Python's
specific tasklet model. Go does not expose APIs to capture or serialize a
goroutine stack, select an arbitrary goroutine and suspend it, or impose a
fully deterministic scheduler. A goroutine can cooperatively block on a
Bullsnake channel or scheduler token, which may cover the practical workload,
but exact Stackless tasklet behavior would still be a separate feature.

Bullsnake should first determine whether cheap `threading.Thread` objects
satisfy the green-thread use cases. A public tasklet API is unnecessary unless
a concrete workload needs tasklet-specific scheduling or continuation
behavior.

## Memory management and lifecycle

### Baseline decision

Bullsnake will rely on Go's tracing garbage collector for Python object memory.
It will not implement Python reference counts, collection generations, or a
second cycle detector in the baseline runtime.

Python objects should be ordinary Go allocations connected by typed,
GC-visible references. Active runtimes, modules, frames, suspended tasks, and
Go extensions naturally form the root set through their Go references. This
keeps memory management out of the Python execution engine and lets Go reclaim
ordinary cycles.

This decision covers memory reclamation. Weak-reference notification,
finalizers, deterministic resource release, and Python-visible collection
controls are separate features and may be omitted from the Bullsnake subset.

### Go GC probe results

The standalone [Go GC probe](../experiments/gcprobe/) models interpreter
objects with references stored through Go interfaces. It was run on Go 1.27.0
for `darwin/arm64` on 2026-08-26. Four normal runs and one race-detector run
produced the same observations:

- Go reclaimed an unreachable two-object cycle. Both weak pointers became
  `nil`, and cleanups attached to both objects ran after one forced collection.
- A cleanup record containing only a `weak.Pointer` did not keep its target
  alive. The cleanup observed a `nil` target, which is enough to enqueue a
  future weak-reference callback without retaining the Python object.
- A deliberately blocked cleanup ran concurrently with the main goroutine.
  `runtime.GC()` returned before the cleanup completed.
- A finalizer on an acyclic object received the object and sent it to another
  goroutine, which resurrected it. The original weak pointer was already
  cleared. A new weak pointer was live and had a different weak identity.
- After the resurrected object became unreachable again, its cleanup ran after
  another forced collection.
- A finalizer attached to a self-referential object did not run after 68 forced
  collections in each run. Its weak pointer continued to return the object.
- The race detector reported no race in the probe's queued-notification model.

The command uses forced collections and bounded waits to make behavior visible.
These observations do not strengthen Go's API guarantees. Go documents that
weak pointers may never clear, cleanups and finalizers may run arbitrarily
late, callbacks may not run before process exit, cleanups run concurrently,
and finalizers in cycles are not guaranteed to run.

### Architectural conclusions

The probe supports these decisions:

1. **Use Go GC for all ordinary Python memory.** Interface-held references and
   cycles do not require a Bullsnake collector.
2. **Keep references visible to Go.** Object representations and future
   optimizations must not hide live references in `uintptr`, unsafe pointers,
   or untraced packed storage.
3. **Omit `__del__` from the baseline subset.** `runtime.SetFinalizer` can model
   acyclic resurrection, but its timing, weak-pointer interaction, goroutine,
   and cycle behavior make it an unsuitable foundation for predictable Python
   finalization.
4. **Omit the `gc` module from the baseline subset.** Bullsnake has no Python
   collector to configure or inspect. Exposing `runtime.GC()` would suggest
   control and completion guarantees that Go does not provide.
5. **Treat `weakref` as optional.** If the package corpus needs it, a
   `weak.Pointer` plus `runtime.AddCleanup` can enqueue a notification. The
   owning runtime must execute any Python callback later at a safe point.
6. **Never execute Python from a cleanup or finalizer goroutine.** Cleanups run
   concurrently and can arrive after arbitrary delay.
7. **Release external resources explicitly.** Context managers, `close()`
   methods, and Go-side lifecycle APIs remain the dependable mechanisms for
   files, sockets, locks, and other scarce resources.
8. **Use cleanups only as a fallback or notification.** Ordinary Python memory
   needs no per-object cleanup registration.

If a future package requires `__del__`, Bullsnake should first decide whether a
smaller, explicitly different contract is acceptable. A custom reachability
layer for finalizable objects would be possible, but it would add substantial
complexity for a feature the baseline intentionally avoids.

### Object identity

If the subset includes `id()`, Bullsnake should assign or derive a stable
logical identity that remains valid if Go ever moves objects. Converting a Go
pointer to an integer is a fragile definition and can tempt code to hide
pointers from the collector. The `is` operator still requires stable identity
for live objects even if numeric IDs are omitted.

### Weak references

A future weak-reference implementation needs Python weak-reference objects,
callback registration, hash behavior, proxy behavior, and a runtime queue. Its
contract must acknowledge Go's nondeterministic notification and shutdown
behavior. Package support should justify this feature before it enters the
runtime.

### The `gc` module

Importing `gc` should fail as an unsupported module in the baseline subset. A
small compatibility module may be added later for a concrete package, but it
must expose Bullsnake semantics and must not imitate CPython generation counts,
reference counts, or deterministic `collect()` completion.

## Concurrency model

### Baseline decision

Each supported Python `threading.Thread` maps to one Go goroutine. Each
runtime also has one execution token. A Python thread must hold that token to
execute bytecode, call Python object protocols, or mutate runtime state.

The token is conceptually similar to a per-runtime GIL, but it exists for
Bullsnake's simpler ownership model rather than for reference counting or a C
API. It gives goroutine-backed Python threads concurrent lifetimes while only
one of them executes Python at a time.

A thread follows this lifecycle:

1. `Thread.start()` allocates a Bullsnake `ThreadState`, assigns a logical ID,
   registers it with the runtime, and starts one goroutine.
2. The goroutine acquires the runtime execution token before entering the VM.
3. The VM releases the token at a periodic safe point so another CPU-bound
   Python thread cannot starve indefinitely.
4. A blocking operation releases the token before waiting and reacquires it
   before touching Python state again.
5. Thread completion records its state and any uncaught exception, unregisters
   the live thread, and wakes `join()` callers.

The execution token should be a scheduler abstraction rather than a mutex
scattered through VM code. Safe-point checks can use a small instruction budget
or elapsed-time budget. When another thread is waiting, the safe point should
hand off the token instead of relying on `sync.Mutex` fairness. Exact CPython
switch intervals are outside the baseline contract.

### What this model provides

- Python threads are cheap enough to use as green threads for many workloads.
- Blocking one Python thread does not block unrelated goroutines or runtimes.
- Synchronous Python code keeps a normal blocking programming model.
- `threading.local` maps cleanly to data stored in `ThreadState`.
- Python locks, events, conditions, semaphores, and joins can wrap Go
  synchronization while retaining Python-visible state and errors.
- Independent Bullsnake runtimes execute Python bytecode in parallel.
- A Go extension can release the runtime token while doing CPU-bound or
  blocking work that does not access Python objects. Several such native Go
  operations can run in parallel.
- The same thread abstraction could support a future free-threaded runtime
  without changing the public `threading.Thread` API.

This is close to default CPython 3.14 behavior. Its normal build lets many
threads exist and releases the GIL around blocking I/O, while only one thread
executes Python bytecode at a time. CPython also has an optional free-threaded
build, but that is a separate and more expensive runtime design.

### What goroutines do not provide automatically

- **Shared-runtime CPU parallelism.** Goroutines can run on several cores, but
  the execution token deliberately serializes Python bytecode in one runtime.
- **Python thread identity.** Go exposes no goroutine ID. Bullsnake must assign
  and track logical IDs itself.
- **Operating-system thread identity.** Goroutines may migrate between OS
  threads. Native thread IDs and thread-affine APIs need explicit treatment.
- **Goroutine-local variables.** Python thread locals and exception state live
  in `ThreadState`, not hidden Go runtime storage.
- **Forced cancellation.** Go cannot safely terminate an arbitrary goroutine.
  Python threads also have no safe general-purpose kill operation, so shutdown
  must be cooperative except for abandoning daemon threads with the runtime.
- **Deterministic scheduling.** Go controls goroutine scheduling. Bullsnake
  controls only ownership of its execution token and explicit safe points.
- **Stackless continuation APIs.** Go does not expose goroutine stack capture,
  serialization, or arbitrary external suspension.

### Why immediate free-threaded execution is expensive

Removing the execution token would allow Python bytecode from several
Bullsnake threads to run simultaneously up to `GOMAXPROCS`. Go makes that
possible, but it does not make shared Python state safe.

A free-threaded runtime would need deliberate synchronization for:

- Lists, dictionaries, sets, byte arrays, instances, classes, and modules
- Import caches, built-in registries, intern tables, and method caches
- Iterators and views observing concurrent mutation
- Frame inspection, tracing, warnings, and runtime shutdown
- Go extension objects and callbacks
- Compound operations whose Python-level atomicity is otherwise ambiguous

The difficult cases involve reentrancy. A dictionary operation may invoke
Python `__hash__` or `__eq__`; attribute access may invoke descriptors; a class
operation may invoke a metaclass. Holding an ordinary object lock while such a
callback re-enters Python can deadlock. Releasing the lock requires version
checks, retries, snapshots, or a more elaborate container algorithm.

Go's collector removes the thread-safety problem of concurrent reference-count
updates. It does not protect fields, slices, maps, invariants, or multi-step
Python operations. Any unsynchronized Go access is also a data race under the
Go memory model.

Free-threaded CPython 3.14 illustrates the cost. Its built-in containers use
internal locks, some objects become immortal, memory reclamation needs extra
coordination, frame inspection is restricted, and single-thread performance
has measurable overhead. Bullsnake can avoid that machinery unless a real
workload needs shared-memory CPU parallelism.

### Alternatives

| Model | CPU parallelism | Shared Python objects | Main tradeoff |
| --- | --- | --- | --- |
| Goroutine per thread with runtime token | Across runtimes and detached Go work | Yes, within one runtime | Selected baseline; simple object model, no same-runtime bytecode parallelism |
| Goroutine per thread with fine-grained locks | Yes, within one runtime | Yes | Highest implementation and testing cost; reentrancy and extension safety are difficult |
| One isolated runtime per goroutine | Yes | No implicit sharing | Simple parallelism with message passing or copied values |
| One cooperative scheduler with explicit tasks | No | Yes | Deterministic and simple, but blocking calls require adapters and `threading` semantics diverge |
| One OS thread per Python thread | Yes if the runtime is thread-safe | Yes | More expensive and discards Go's M:N scheduler without providing a needed benefit |

### Recommended path

1. Implement a goroutine and explicit `ThreadState` for each selected Python
   thread.
2. Serialize Python execution with one runtime token and switch owners at VM
   safe points.
3. Release the token around blocking waits and detached Go extension work.
4. Provide true Python-bytecode parallelism through independent runtimes first.
5. Use package tests and real workloads to decide whether shared-runtime
   free-threading justifies fine-grained synchronization later.

This preserves the main benefit of the proposed approach: Python threads are
lightweight and Go schedules them. It also keeps the baseline consistent with
Bullsnake's priority of the simplest powerful architecture.

## Frames, introspection, and diagnostics

Explicit frames are required internally for calls, suspension, exception
unwinding, and tracebacks. They should retain source and namespace information
needed for actionable errors and the introspection features selected later.

The baseline should provide traceback construction, exception chaining, and
accurate source positions. `inspect`, `sys._getframe()`, tracing, profiling,
breakpoints, coverage hooks, writable frame locals, and generator state
inspection are independent optional features. A package requirement must
justify exposing each one because public frame details constrain VM changes.

Bullsnake does not need a `dis` module. Its bytecode is private and can be
printed by a Go development tool or test helper without making instruction
layout part of the Python runtime contract.

## Proposed repository layout

Start with a few coarse packages. The internal directories can split after
real dependency or ownership pressure appears:

```text
cmd/bullsnake/          command-line interpreter and REPL
py/                     public values and Go extension contracts
internal/compiler/      lexer, parser, AST, symbols, bytecode, and compiler
internal/runtime/       objects, frames, VM, imports, built-ins, and scheduler
stdlib/                  selected Python modules shipped by Bullsnake
experiments/             disposable architecture probes such as gcprobe
tests/compat/           differential and package compatibility tests
testdata/               compiler and runtime fixtures
```

The root `bullsnake` package should provide the high-level embedding API and
compose the compiler and runtime. The `py` package should be a dependency leaf
so native modules can use it without importing internals. Internal packages may
depend on `py`; `py` must not depend on them.

The compiler produces immutable code objects consumed by the runtime. Import,
scheduling, and built-ins can remain files within `internal/runtime` until one
of them has a real reason to become an independent package.

## Verification strategy

Correctness work needs several test layers:

1. Lexer, parser, symbol-table, compiler, object-model, and VM unit tests cover
   supported behavior, local invariants, and error paths.
2. Negative tests prove unsupported syntax, imports, and operations fail at the
   documented boundary with a useful error.
3. Differential tests run selected programs under Bullsnake and the chosen
   CPython 3.14 patch release. They compare only behavior that Bullsnake intends
   to share with Python.
4. Intentional divergences have direct Bullsnake tests. They do not remain as
   unexplained exclusions from a CPython test suite.
5. Carefully selected CPython regression tests exercise adopted language and
   standard-library behavior. Direct lexer cases are checked-in Go tables
   pinned to CPython 3.14.7 and run without an external interpreter. There is no
   goal to maximize the number of CPython tests that run.
6. Parser and compiler fuzzing checks that malformed input fails cleanly and
   valid supported input never corrupts VM state.
7. A pinned package corpus runs each package's upstream tests. Results record
   package version, dependency versions, skipped tests, unsupported modules,
   intentional divergences, and Bullsnake revision.
8. Threading tests cover CPU-bound fairness, token release around blocking
   operations, joins, thread-local state, lock primitives, daemon shutdown,
   and extension callbacks. Stress variants run with Go's race detector.
9. Async tests use a controllable clock and deterministic I/O completions so
   scheduler behavior is reproducible.
10. Cross-runtime tests prove that modules, exceptions, contexts, and extension
    registries do not leak between instances or Python thread goroutines.

Differential tests must account for valid implementation differences. Exact
object IDs, hash values, finalization timing, memory statistics, error wording,
and Bullsnake bytecode are compared only if the feature manifest promises that
behavior.

The feature matrix should use precise states such as `supported`, `partial`,
`unsupported`, `diverges`, and `implementation-specific`. Every `supported` or
`diverges` entry links to tests and a short contract.

## Prior art

The `go-python/gpython` project already demonstrates a Python parser, compiler,
bytecode VM, embeddable runtime, and concurrent interpreter instances in Go.
Its documented target is Python 3.4, and its README identifies missing
C-backed standard-library modules as the main barrier to broader compatibility.
Bullsnake should audit it for lessons and reusable ideas before implementation,
while treating Python 3.14, async support, and Bullsnake's API as new design
constraints.

Stackless Python demonstrates the value of separating logical Python frames
from the host language call stack. Bullsnake can get that property at the
start, with less machinery, because its frame representation is new.

## Open decisions

The following decisions should be resolved before their related implementation
begins:

1. Define the initial feature manifest, including supported syntax, built-ins,
   protocols, modules, and intentional divergences.
2. Choose the first package corpus and define what passing each package means.
3. Decide how much CPython standard-library source to vendor, and establish its
   update and licensing process.
4. Decide whether the package corpus needs `weakref`. `__del__` and `gc` remain
   omitted unless a future decision explicitly reopens them.
5. Define the first goroutine-backed `threading` subset and its safe-point
   fairness policy. Candidate primitives are `Thread`, `Lock`, `RLock`,
   `Event`, `local`, `current_thread()`, and `join()`.
6. Decide which frame and code-object introspection APIs are required by the
   first package corpus.
7. Decide whether Go extensions remain statically linked or need a later
   process-based or plugin-based distribution mechanism.
8. Validate whether goroutine-backed Python threads satisfy the intended
   green-thread workloads before designing any separate tasklet API.
9. Select initial host platforms and decide how unsupported OS, signal,
   subprocess, and networking behavior is reported.

## References

- [Python 3.14 data model](https://docs.python.org/3.14/reference/datamodel.html)
- [Python 3.14 execution model](https://docs.python.org/3.14/reference/executionmodel.html)
- [Python 3.14 import system](https://docs.python.org/3.14/reference/import.html)
- [Python coroutines and tasks](https://docs.python.org/3.14/library/asyncio-task.html)
- [Python context variables](https://docs.python.org/3.14/library/contextvars.html)
- [Python threading](https://docs.python.org/3.14/library/threading.html)
- [Python free-threading behavior](https://docs.python.org/3.14/howto/free-threading-python.html)
- [CPython bytecode is an implementation detail](https://docs.python.org/3.14/library/dis.html)
- [PEP 421, identifying alternate Python implementations](https://peps.python.org/pep-0421/)
- [Python package formats and pure wheels](https://packaging.python.org/en/latest/discussions/package-formats/)
- [Go garbage collector guide](https://go.dev/doc/gc-guide)
- [Go runtime scheduling, OS-thread, finalizer, and cleanup documentation](https://pkg.go.dev/runtime)
- [Go weak pointer documentation](https://pkg.go.dev/weak)
- [Go FAQ on goroutines, parallelism, and goroutine IDs](https://go.dev/doc/faq)
- [Go memory model](https://go.dev/ref/mem)
- [gpython](https://github.com/go-python/gpython)
- [Stackless Python](https://github.com/stackless-dev/stackless/wiki)
