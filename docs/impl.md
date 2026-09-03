# Bullsnake implementation guide

Status: living implementation notes

This document describes the code that exists today. It explains how source
moves through the repository, what each package owns, which Python behavior can
run, and where execution stops. For long-term goals and design reasons, read the
[architecture](architecture.md).

Bullsnake is under active development. Parsing a program does not mean the
compiler can translate it, and compiling it does not mean the runtime can
execute every instruction. Each stage rejects behavior it does not yet own.

All compatibility work uses CPython 3.14.7 at commit
`823f0323ee6ec1402088b73bce1a38473cac36dc` as the exact reference.

## Current state

| Part | What works now |
| --- | --- |
| Source loading | Python encoding cookies, UTF-8 BOM handling, and selected legacy encodings |
| Lexer | Initial Python 3.14 tokenization, including f-strings and template strings |
| Parser | A broad Python 3.14 statement and expression grammar |
| Resolver | Name scopes, closures, contextual checks, annotations, generics, and comprehensions |
| Compiler | An executable subset with functions, classes, imports, exceptions, generators, and coroutines |
| Runtime | Modules, values, collections, Python and Go-backed functions, basic classes, and structured exceptions |
| Imports | Regular and namespace source packages plus cached Go-backed system modules |
| Go API | Explicit host configuration, source/file execution, module lookup, and read-only values |
| Standard library | All 535 core `unittest` and 559 `unittest.mock` tests pass; `test_unary` and `test_contains` also pass unchanged |
| Async and REPL | Coroutine `await`, async iteration/comprehensions, and the `IsolatedAsyncioTestCase` runner work; general scheduling and a REPL do not |

The parser and resolver intentionally cover more language forms than the
compiler. The compiler also defines some bytecode that the runtime still
rejects. This lets each package grow and remain testable without pretending the
whole pipeline supports a feature.

## The pipeline

The current internal path from decoded text to execution is:

```text
parser.Parse
    -> resolver.Resolve
    -> compiler.Compile
    -> runtime.New
    -> Runtime.ExecuteModule
```

Raw file bytes first pass through `internal/compiler/source`. The public
`bullsnake.Interpreter` combines these stages for `ExecuteModule` and
`ExecuteFile`. `internal/importer.FileSystem` uses the same sequence for module
files found under configured roots, then supplies the resulting code through
the runtime's loader callback.

Errors belong to the stage that can explain them. The source loader reports
encoding failures. The lexer and parser report malformed syntax. The resolver
reports invalid name or scope use. The compiler reports supported syntax that
it cannot translate. The runtime reports invalid bytecode as a `BytecodeError`
and uncaught Python exceptions as an `UncaughtException`.

## Source loading

`internal/compiler/source` reads a complete source unit before lexing. `Decode`
accepts bytes, `Read` consumes an `io.Reader`, and `ReadFile` reads a path. Each
returns a `Unit` with the filename, detected encoding, and decoded text.

The loader implements the first-two-line encoding-cookie rules from PEP 263. It
defaults to strict UTF-8, removes an initial UTF-8 BOM, rejects a cookie that
conflicts with that BOM, and converts supported encodings to UTF-8. It preserves
the source's physical newline spelling in the decoded text.

Bullsnake has no Python codec registry. It accepts ASCII-compatible encodings
available through `golang.org/x/text`, plus common Python aliases. Unsupported,
non-ASCII-compatible, and malformed input produces a structured `source.Error`
instead of replacement characters.

The lexer accepts decoded text. It does not read files or remove a BOM, and it
rejects malformed UTF-8 and null bytes.

## Lexer

`internal/compiler/lexer` is a pull-based tokenizer. `Next` advances through one
source string while retaining indentation, delimiter, and formatted-string
state. Tokens keep their exact source spelling and a half-open source span.
Lines start at one. Columns and absolute offsets count UTF-8 bytes, matching the
coordinates used by Python compiler and AST nodes.

The lexer handles Python indentation and tab consistency, physical and logical
newlines, explicit and implicit line joining, delimiter nesting, Python 3.14
operators, number forms, ordinary strings, f-strings, template strings, and
PEP 3131 identifiers. It retains comments and non-significant newlines as
trivia so future source tools do not need a second tokenizer.

Literal evaluation happens later. Number and string tokens still contain source
text, keywords remain name tokens, and operators have exact token kinds. The
parser decides which names are keywords. The compiler converts literal values.

The scanner exposes an `Incomplete` hint for open delimiters, unfinished triple
strings, and final line continuations. A future REPL must combine that hint with
parser completeness because valid tokens can still form an unfinished
statement.

The lexer follows Python 3.14's Unicode 16 identifier set even though the Go
toolchain and `x/text` use a newer Unicode release. It removes the small newer
identifier addition rather than accepting names Python 3.14 rejects.

## Parser and AST

`internal/compiler/parser.Parse` accepts a filename and one decoded source
string. The hand-written recursive-descent parser requests tokens lazily and
builds nodes from `internal/compiler/ast`.

The grammar covers ordinary simple and compound statements, assignments,
imports, functions, classes, type parameters and aliases, context managers,
exception handling including `except*`, and pattern matching. Expressions cover
comprehensions, lambdas, calls, subscriptions, collection displays, f-strings,
template strings, `await`, and `yield`. This is a grammar claim, not an
execution claim. Later stages still reject unsupported forms.

AST nodes carry source spans and syntax facts needed by later stages. They do
not contain resolved names, bytecode positions, or runtime values. `ast.Dump`
provides a deterministic description for tests. The AST is private and may
change when a later stage needs a clearer representation.

The parser uses one module grammar. It marks an end-of-input error as incomplete
when more source could finish the construct. A future eval API or REPL can add
its own result policy without a second parser mode.

## Resolver

`internal/compiler/resolver.Resolve` walks a parsed module and returns a
separate symbol table. It does not modify the AST.

The first pass creates scopes, records name uses and bindings in source order,
reads future imports, applies private-name rewriting, and checks rules that need
surrounding syntax. The second pass classifies names as local, cell, free,
explicit global, or implicit global. Free-variable requests travel outward so
enclosing functions allocate the required cells.

The scope tree includes modules, functions, lambdas, classes, comprehensions,
annotations, type-parameter lists, type-variable bounds and defaults, and type
alias values. One AST node can own more than one scope, so `Table.ScopeFor`
selects a scope by its purpose.

The resolver handles closure propagation, class scope rules, `__class__` and
`__classdict__` cells, private names, comprehension bindings, assignment
expressions, deferred annotations, generic scopes, and pattern captures. It
also records generator and coroutine flags and checks where control-flow and
suspension syntax may appear. Unknown names remain implicit globals for runtime
lookup.

The resolver decides what a name means. It does not assign bytecode indexes or
choose load and store instructions. Those decisions belong to the compiler.

## Compiler and bytecode

`internal/compiler.Compile` accepts one AST module and its resolver table. It
returns an immutable `internal/compiler/bytecode.Code` or a source-located
compiler error.

A code object describes one module, function, lambda, class body, or deferred
annotation body. It contains instructions, source positions, constants, names,
fast locals, closure cells, free variables, child code objects, argument
metadata, maximum stack size, and exception-handler ranges. Construction copies
mutable input, and accessors return copies of tables. Bytecode is an in-memory
internal format, not a `.pyc` format or a compatibility promise.

Class-private names use the resolver scope's private owner consistently for
symbol indexes and attribute spellings, so the compiler and runtime share one
mangled layout.

The compiler tracks operand-stack depth while emitting. Labels patch forward
jumps and require every incoming control-flow path to agree on the stack depth.
Protected instruction ranges record where an exception handler starts and how
much stack to retain. The runtime validates those claims independently.

The current compiler translates:

- scalar literals, f-strings, and tuple, list, set, and dictionary displays
- names, attributes, calls, subscriptions, slices, operators, comparisons, and
  conditional expressions
- simple, chained, destructuring, annotated, augmented, and deletion targets
- `if`, synchronous `while` and `for`, loop `else`, `break`, and `continue`
- synchronous functions, lambdas, every parameter kind, defaults, decorators,
  lexical closures, returns, lazy function annotations, eager list, set, and
  dictionary comprehensions, synchronous generators, coroutine definitions,
  `await`, and lazy asynchronous-generator creation
- classes with decorators, generic aliases and union bases, C3 multiple
  inheritance, class keywords, methods, enclosing closures, annotated class
  attributes, and the cells used by class-visible annotations and `__class__`
- ordinary imports and assertions
- ordinary exception handlers, `else`, `finally`, exception groups, `except*`,
  bare reraising, explicit causes, and cleanup during return or loop transfer
- synchronous context managers, including nested exit ordering, exception
  suppression, and cleanup during return or loop transfer, plus `async with`
  over the current coroutine subset
- structural pattern matching for value, capture, wildcard, OR, sequence,
  mapping, class, guard, and `as` patterns

The runtime currently executes true division for integer, boolean, and float
operands, always producing a float. Integer floor division and modulo retain
their arbitrary-precision behavior.

Integer constants use arbitrary precision. Float and imaginary constants use
binary64. String decoding preserves lone surrogate escapes in an internal
WTF-8-compatible form, while bytes constants preserve arbitrary bytes.
Formatted-string compilation retains conversion, format-specification, raw
prefix, and debug-field behavior needed by the current runtime formatter.

Functions and class bodies are child code objects. Closures contain explicit
cell references instead of Go closures. Deferred annotation bodies are also
children and do not run during an ordinary function definition or call.
Class bodies that define class-visible method annotations capture a live
namespace mapping; annotation code checks that mapping before its global or
enclosing-cell fallback.

Template strings currently preserve their formatted text but not the complete
interpolation objects exposed by CPython. Class annotations are materialized as
name entries with `None` placeholders rather than deferred PEP 649 annotation
functions. The compiler rejects `from __future__ import annotations`, generic
definitions, asynchronous comprehensions, and `async for`. Unsupported AST
forms return compiler errors; they are not approximated with similar bytecode.

## Runtime preparation

`internal/runtime.Runtime` owns builtins, prepared code, and successful modules.
`ExecuteModule` prepares a complete code tree before creating observable module
state. Prepared code copies the code tables and converts constants to runtime
values. The runtime caches that prepared form by code-object identity.

Preparation checks all child code, including deferred and unreachable code. It
rejects unknown instructions, unsupported operands and metadata, invalid table
indexes, bad jumps, malformed exception ranges, stack underflow or overflow,
control-flow joins with incompatible stack state, reachable fallthrough, and
invalid return shape. A failure is a source-located `BytecodeError`.

Stack validation follows every reachable control-flow edge with a worklist. It
models the different stack results of conditional jumps, iteration, and
exception edges. It also checks unreachable instructions and operands, even
though they do not contribute control-flow edges. No module code runs until the
whole tree passes.

## VM and frames

The VM allocates Python frames on the Go heap and executes them in one loop. A
frame stores prepared code, the next instruction, operand stack, fast locals,
closure cells, local and global namespaces, builtins, active handled
exceptions, and its caller.

An instruction can advance, call another Python frame, return, or raise. A call
switches the loop to a new frame. A return restores the caller and pushes the
result. Python recursion therefore does not recurse through the Go call stack.

Calling synchronous generator code creates a lazy generator with a detached
heap frame. `FOR_ITER` attaches that frame to the active caller, and `yield`
detaches it again after preserving its instruction, operand stack, locals, and
exception state. Basic `yield from` delegates ordinary iteration. The current
slice does not yet expose `send`, `throw`, `close`, or a return value from a
delegated generator.

Calling coroutine and asynchronous-generator code likewise creates distinct
lazy values without executing the body. Awaiting one Bullsnake coroutine from
another attaches its frame and returns its result through the same VM loop.
Coroutines expose `close`. The `asyncio.Runner` compatibility layer drives a
root coroutine, preserves selected `contextvars` state, and supplies the
cancellation behavior exercised by `IsolatedAsyncioTestCase`. It is not a
general event loop; asynchronous iteration and host-backed async I/O are not
implemented.

A raised Python exception follows protected ranges in the current code. If a
range matches, the VM trims the operand stack to its recorded depth, pushes the
exception, and resumes at the handler. Otherwise it removes the frame and
continues at the caller's call instruction. An uncaught exception retains
Python traceback entries and crosses the host boundary as
`UncaughtException`.

Frames also track exceptions active inside handlers and final suites. This makes
bare `raise`, implicit context, explicit causes, and replacement by a newer
return, loop transfer, or exception work across nested calls and cleanup.

Synchronous `with` keeps each entered manager on the operand stack across its
protected suite. Normal and control-transfer exits call
`__exit__(None, None, None)`; exceptional exits receive the exception class and
instance and may suppress propagation with a truthy result. Traceback arguments
are currently `None` because Python traceback objects are not exposed yet.

## Runtime values

Runtime values implement a sealed `Value` interface. Current concrete values
include the Python singletons, arbitrary-precision integers, binary64 floats,
complex numbers, strings, bytes, tuples, lists, dictionaries, sets, slices,
iterators, generators, coroutines, asynchronous generators, modules, functions,
classes, instances, Python and Go-backed bound methods, built-in type markers,
and exceptions.

The internal class-namespace mapping is a live `Value` wrapper around the
class body's namespace. It exists for lazy annotation lookup and does not copy
or bypass later class-body assignments.

The object model implements the behavior needed by the executable subset.
Collections support displays, eager comprehensions, unpacking, iteration,
membership, integer and slice subscription, and dictionary item mutation.
Comprehensions run in isolated child scopes, evaluate their first iterable in
the enclosing scope, and retain nested iterators above one collection
accumulator. Current values also support
truth testing, selected scalar operations and comparisons, and attribute access
for modules, basic classes, and instances. Dictionaries and sets currently use
ordered linear storage. This keeps Python identity and equality checks explicit
until user-defined hashing and equality can call back into Python.

Strings index and iterate by decoded code point, including preserved lone
surrogates. Bytes index and iterate as integers. Slices use Python-style bound
clipping and positive or negative steps. Dictionary iteration detects key-set
changes; replacing an existing value is allowed. Set display and iteration
order is stable for Bullsnake tests but is not a Python compatibility promise.

The current function binder supports positional-only, positional, keyword-only,
`*args`, and `**kwargs` parameters, positional and keyword-only defaults, and
keyword unpacking. Defaults retain the objects created when the definition ran.
Calls reject duplicate, missing, unexpected, or non-string keyword arguments
with Python exceptions.

Classes support multiple bases with C3 method resolution, inherited attribute
lookup, bound Python and built-in methods, ordinary `__init__`, zero- and
two-argument `super`, instance and class attribute mutation, user exception
subclasses, and built-in `classmethod`, `staticmethod`, and `property`
descriptors including getter/setter/deleter cloning. The `metaclass=` keyword
records a Python metaclass and binds its methods on constructed classes; full
metaclass construction hooks are not yet invoked. The object model does not yet
implement other class keyword arguments, `__new__`, general user-defined
descriptors, or property setter dispatch during assignment. Custom exception
initializers and methods remain unsupported.

The formatter supports current strings, integers, booleans, and floats for the
format forms covered by execution tests. It does not yet provide general
`__format__` dispatch.

Pattern matching evaluates one subject and retains it across failed cases.
Sequence patterns support one starred remainder, mapping patterns support a
dictionary remainder, and class patterns use `__match_args__`, keyword
attributes, and the built-in self-matching scalar and collection types. Missing
structure produces a pattern miss while invalid pattern metadata raises the
corresponding Python exception.

## Exceptions

Python exceptions are `Value` implementations. The runtime has the built-in
exception classes needed by current operations and follows their inheritance
when matching handlers. `raise` accepts an exception instance or a supported
exception class. Bare `raise` uses the active handled exception.

Ordinary `try` statements support ordered typed or bare handlers, `as` bindings,
`else`, and `finally`. Handler bindings clear on every exit. Final suites run
for normal completion, propagation, return, break, and continue. A transfer
started in the final suite replaces the pending transfer.

Fresh exceptions record an active handled exception as `__context__`.
`raise ... from ...` records `__cause__` and suppression state. Reraising
preserves the original source location and traceback.

`BaseExceptionGroup`, `ExceptionGroup`, and `except*` support nested groups,
recursive splitting, ordered clauses, unmatched remainder propagation, and
combination of handler failures. The runtime does not yet call a custom
exception group's `derive` override.

The host can inspect an uncaught exception's copied traceback and formatted
backtrace. Python `__traceback__` objects, frame objects, and broad introspection
are not implemented.

## Modules and imports

A module has one string-keyed namespace used as both locals and globals.
`Runtime.ExecuteModule` executes bare code as an ordinary module.
`Runtime.ExecuteModuleSpec` preserves a loader's package, origin, and search
metadata for a file or package entry. Both create and cache the module before
its body starts. `Runtime.Module`, `Module.Get`, module attribute access, and
imports all observe the same object and namespace, including names assigned
during partial initialization.

`NewWithLoader` accepts a `ModuleLoader` callback. A `ModuleRequest` carries the
absolute name and, for a child, a copy of its parent package's search locations.
Each result is a `ModuleSpec` with immutable code, a package flag, an optional
origin, and package search locations. A cache miss prepares the complete
returned code tree, creates the module, and switches the existing dispatch loop
to a module frame. Normal return leaves the module cached. Repeated and circular
imports reuse that identity without another loader call.

A dotted absolute import loads its prefixes in order. Every intermediate module
must be a package. After a child returns, the runtime publishes it on its parent
and resumes the suspended `IMPORT_NAME` instruction for the next component. An
ordinary `import package.child` returns the top package. A nonempty from-list
returns the requested child module, matching the stack contract used by the
compiler. When that requested module is a package, each missing from-list
attribute is tried as a child module. An exact missing child is left for
`IMPORT_FROM` to report as `ImportError`; a failure raised while executing a
found child propagates normally. Modules initialize `__name__` and
`__package__`; loader metadata adds `__file__` and a package `__path__` list.

Wildcard imports use a module's `__all__` list or tuple when present. Package
handling first attempts to load missing names from that sequence as child
modules. `IMPORT_STAR` then copies exactly the listed names, including leading
underscores. Without `__all__`, it copies namespace names that do not begin with
an underscore. Other Python sequence implementations are not yet accepted for
`__all__`.

`IMPORT_NAME` resolves a positive relative level against the executing frame's
`__package__` global, then uses the same absolute loading path. Level one keeps
the complete package name; each additional level removes one component. An
empty package name raises the no-known-parent `ImportError`, while removing too
many components raises the beyond-top-level form.

If a Python exception leaves an imported module, the frame unwind removes its
cache entry before checking the importer's handler. A later import may retry it.
Modules that completed as side effects remain cached. A host loader error also
removes every module frame still initializing. An explicit `ExecuteModule`
failure restores any older module that the execution temporarily replaced.

`internal/importer.FileSystem` implements the callback contract while keeping
source decoding and compilation outside `internal/runtime`. It searches roots
in order. At each location it prefers `name/__init__.py` over `name.py`.
Submodule requests search only the locations retained from the parent package's
specification. Found files use the ordinary source loader, parser, resolver, and
compiler, and preserve typed frontend errors under a module-loading wrapper.
Every source read uses its configured `host.FileSystem`; the default constructor
selects the operating-system implementation, while `NewFileSystemWithHost`
accepts mocks and policy wrappers.

A missing callback, missing file, or missing callback result raises
`ModuleNotFoundError`. Filesystem and frontend failures remain Go host errors
until the runtime has the corresponding Python exception values. The loader
combines namespace-package directories and publishes namespace `ModuleSpec`
metadata. General dynamic `__path__` changes, finder and loader hooks, reload,
and import locks remain unimplemented.

## Host services and system modules

The public `host.Services` value contains independent read-only filesystem,
filesystem-mutation, clock, time-zone, entropy, network, process,
working-directory, and standard-stream capabilities. `bullsnake.New` and
`runtime.NewWithConfig` do not fill nil capabilities. `NewDefault` and the
legacy internal constructors explicitly select current-process defaults.

The filesystem importer uses `host.FileSystem.ReadFile`. `os.listdir`,
`os.path.exists`, `isfile`, `isdir`, and `realpath` use that same filesystem.
`os.remove`, `os.unlink`, and `os.rmdir` use the separately grantable
`host.FileMutator`.
Working-directory and environment reads use `host.Process`, while `os.chdir`
uses the separately grantable `host.WorkingDirectory`. `time.time`,
`monotonic`, `perf_counter`, and `sleep` use `host.Clock`; `localtime` and
`ctime` delegate host-local conversion to `host.TimeZone`. `os.urandom` and
implicit random seeding use `host.Entropy`. `sys.stdin`, `stdout`, and `stderr`
retain only their configured streams. A missing or denied system-module
capability raises `PermissionError`; other host failures become the current
`OSError` subset.

Go-backed calls are ordinary runtime values dispatched by the VM. They share
Python call-stack cleanup and exception routing and may opt into keyword-aware
argument handling. Implemented bootstrap module behavior is intentionally
narrow:

- `sys` exposes version 3.14.7 metadata, `argv`, `path`, standard streams,
  `modules`, `exc_info`, and `exit`
- `time` exposes wall time, local/UTC tuples, monotonic/performance time, and sleeping
- `io` and `_io` expose the `StringIO` and `BytesIO` operations used by test streams
- `os` exposes host data, directory listing, metadata, separately authorized
  removal operations, and the first `os.path` operations
- `builtins` can be imported and shares the runtime's current built-in values
- `__future__` exposes the feature names used by the pinned source tests
- `_abc` provides the registry operations used by the pinned `abc` module
- `_ast`, `_contextvars`, `_functools`, `_opcode`, `_random`, `_signal`,
  `_thread`, `_weakref`, `asyncio`, `enum`, `errno`, `importlib`, `itertools`,
  `math`, `operator`, `pkgutil`, `re`, `reprlib`, `typing`, and `warnings`
  provide the bootstrap surface required while executing the pinned
  `unittest` package

The runtime preloads `sys` and `builtins`, then creates other system modules on
demand through the ordinary module cache. `sys.modules` follows runtime cache
updates and rollback, and Python assignments can publish aliases consumed by
later imports. The regular CPython 3.14.7 `Lib/unittest` package now imports
successfully from the pinned checkout. Its `TestCase`, assertion context
managers, fixtures, cleanups, subtests, skips, discovery, program interface,
interrupt handling, class loader, suites, results, `TextTestRunner`,
`IsolatedAsyncioTestCase`, `mock`, `patch`, autospeccing, magic-method mocks,
`ThreadingMock`, and `AsyncMock` execute without a native runner or mock
substitute.
Several dependencies above deliberately expose only the surface reached by
that work: regular expressions use Go's engine, `_signal` keeps
interpreter-local handlers without touching the process, and `_ast`, `_opcode`,
`enum`, and `importlib` are compatibility shims rather than complete modules.
General native object protocols, writable files, bytes streams, and a socket
module remain future work.

The repository keeps unchanged CPython test files as conformance targets. The
pinned pure-Python `Lib/unittest` package and its transitive import dependencies
run through the ordinary source-import path; there is no Go-native `unittest`
substitute. The opt-in integration target passes all 535 core framework tests,
all 559 `unittest.mock` tests, and the unchanged `test_unary` and
`test_contains` modules, for 1,104 tests.

## Deliberate boundaries

The largest current gaps are:

- no Go callback/type/module extension API
- no broad standard library or native extension loading
- no generator `send` and only the exception injection and close behavior
  needed by context managers; no general event loop or Python threads
- no complete Python object protocol, general descriptors, or user hashing
- no Python frame and traceback objects, tracing, profiling, debugger hooks, or
  execution budgets
- no REPL or eval-specific entry point

Earlier stages may already understand syntax associated with these gaps. The
later-stage rejection is intentional and remains until that stage has behavior
tests and an implementation.

## Memory management

Go's garbage collector owns runtime memory. Frames, stacks, namespaces, cells,
and container entries keep Python references in typed Go pointers and
interfaces so the collector can trace cycles.

Bullsnake does not implement CPython reference counting, `__del__`, weakref
callbacks, or a compatible `gc` module. Its current callable weak references
are strong compatibility wrappers used during standard-library bootstrap. The experiments under
`experiments/gcprobe` inform these boundaries but are not production runtime
code.

## Testing

The repository uses several test layers:

- focused Go tests for package APIs, spans, dumps, copying, and invariants
- CPython-derived lexer and parser cases pinned to Python 3.14.7
- resolver cases that compare complete deterministic scope dumps
- compiler cases that compare complete deterministic code-object dumps
- chunked Python execution fixtures that pass through parser, resolver,
  compiler, validator, and VM
- expected-error fixtures that check Python exception type and message
- hand-built malformed-bytecode cases that exercise runtime validation
- fuzz tests at lexer, parser, and resolver input boundaries
- unchanged CPython source files retained as executable compatibility targets
- an opt-in external-checkout test for the complete `unittest` self-suite and
  selected language test modules

The checked-in conformance data pins CPython 3.14.7 at commit
`823f0323ee6ec1402088b73bce1a38473cac36dc`. Those default tests run offline and
require no Python binary, CPython checkout, network access, or generation step.
Setting `BULLSNAKE_CPYTHON` to that checkout additionally enables the 1,104-test
standard-library integration target.

Run the repository checks with:

```sh
go test ./...
go vet ./...
go tool laas -funcdoc.limit=5 -exclude-packages='(^|/)experiments($|/)' ./...
git diff --check
```

Behavior changes should start with a focused source or package test, update the
relevant implementation notes, and land as one coherent vertical slice.
