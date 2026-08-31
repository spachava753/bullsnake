# Bullsnake architecture

Status: living design

This document explains the design Bullsnake is working toward. It focuses on
choices that should remain useful as the code changes. For a description of the
code that exists today, read the [implementation guide](impl.md).

## What Bullsnake is

Bullsnake is an experimental Python interpreter written in Go. It uses Python
3.14 as its language reference and implements a selected subset of that
language.

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

Import loading, a public Go API, async scheduling, and Python threads are later
parts of the design. They should use this same compiler and VM rather than
creating alternate execution paths.

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

The AST and symbol table are internal data structures. The module root retains
the decoded source string so later stages can recover source-backed facts from
byte spans without a second file read. A future Python `ast` module can expose
Python objects through an adapter instead of freezing the Go representation as
a public API.

## Compiler and code objects

The compiler receives an AST and a resolved scope table. It chooses
instructions, constants, local-variable positions, closure positions, jump
targets, and source locations.

The result is an immutable code object. A code object contains the information
the VM needs to run one module, function, generator, class body, annotation
body, or hidden comprehension body. Child functions and comprehensions have
child code objects rather than hidden Go closures.

CPython 3.14 inlines eager comprehensions into the enclosing frame. Bullsnake
currently runs every comprehension in a hidden child frame. The enclosing frame
evaluates the first iterable and creates its iterator. An eager child runs at
once and returns its collection. A generator-expression child stays suspended
until iteration. In both forms, targets, filters, later iterables, and the result
expression use the comprehension scope. This simpler compiler model preserves
name isolation and closure behavior. The extra frame may change when Bullsnake
exposes Python frame introspection.

Function definitions with annotations also create a child code object. Without
the future import, the child captures the names needed to evaluate those
expressions. With the future import, it uses retained source strings and creates
no annotation-driven closure cells. Reading `f.__annotate__` returns the child
callable. The first `f.__annotations__` access calls it with value format `1`,
requires a dictionary, and caches the exact result. A failed call leaves the
cache empty so a later access retries it. Unannotated functions cache one empty
dictionary.

Without `from __future__ import annotations`, class annotations use a child code
object. The class body records which annotation statements ran, while the child
captures the live class namespace and any required enclosing cells. Calling
`C.__annotate__(1)` evaluates those annotations and returns a new dictionary. An
explicit class `__annotate__` method takes precedence, and subclasses do not
inherit the generated callable. The first `C.__annotations__` access calls that
function, requires a dictionary, and caches the exact result. A failed
evaluation is not cached. An explicit class `__annotations__` value takes
precedence.

With the future import, module and class scopes create `__annotations__` at
scope entry. An executed simple-name annotation stores the retained source text
without evaluating it. CPython 3.14 instead unparses each annotation AST, which
normalizes spacing and some parentheses. Bullsnake's exact source spelling is a
temporary observable difference for function, class, and module annotations.
Complete mutation rules for annotation attributes remain future work.

A type alias owns a hidden zero-argument value function. Constructing the alias
does not run that function. The first `Alias.__value__` access runs it through
the ordinary VM frame loop and caches the returned object; a failure is not
cached. The hidden function preserves global, enclosing-function, and
class-visible lookup at the definition site.

A generic alias with ordinary TypeVars adds one outer hidden function. That
function creates fresh inferred-variance TypeVars, stores them in closure cells,
builds the lazy alias, and attaches the same objects as
`Alias.__type_params__`. Calling the hidden function at the alias statement keeps
the parameter names out of the defining namespace.

A bound or tuple constraint owns another hidden evaluator that captures the
same definition scope. Reading `T.__bound__` or `T.__constraints__` runs the
corresponding evaluator once and caches a successful result. A failure remains
uncached so a later access can retry. Defaults, variadic type parameters, public
evaluator callables, and alias subscription remain later work.

The compiler tracks operand-stack depth while it emits instructions. Every
control-flow path that joins another path must agree on that depth. This catches
compiler mistakes before the VM runs them.

Basic structural matching keeps one evaluated subject across case tests.
Literal and dotted-value patterns compare copies of that subject. The compiler
stores capture names only after the complete pattern succeeds, then evaluates
the guard. A false guard leaves those names bound, as Python specifies.

Sequence patterns currently accept tuples and lists. They check the candidate
kind and length before unpacking fixed or starred elements. Mapping patterns
currently accept dictionaries. They evaluate each literal or dotted key once,
reject duplicate values, and can capture a shallow `**rest` copy. Class patterns
accept current user classes and exception classes. They follow the current
single-inheritance chain, use inherited `__match_args__` for positional fields,
and read named fields through normal instance and class lookup. A missing field
makes the pattern fail; malformed class-pattern metadata raises `TypeError`.
Tentative nested and OR-pattern captures use hidden frame locals, so a failed
pattern exposes none of them. A successful pattern commits those values before
its guard.

Exceptions use protected instruction ranges. A range says where the handler
starts and how much of the operand stack to keep when an instruction raises.
Normal execution pays no setup cost for entering a `try` block. `finally`,
context-manager exit, and handler-name cleanup are compiler-controlled actions
that run before a return, loop transfer, or propagated exception completes.

A synchronous `with` keeps each bound `__exit__` method on the operand stack.
The compiler protects target assignment and the body, and calls exits from inner
to outer for normal flow, exceptions, returns, and loop transfers. Special
method lookup reads the class rather than an instance attribute. Until Python
traceback objects exist, exceptional `__exit__` calls receive `None` for their
third argument.

Code objects stay in memory today. A bytecode cache, if one is ever needed,
will require an explicit format version and must reject stale or foreign data.

## Virtual machine and frames

The VM uses explicit Python frames stored on the Go heap. A frame holds the code
being executed, the next instruction, its operand stack, its local variables,
closure cells, namespaces, active exception handlers, and a link to its caller.

Python calls switch the current frame in one iterative dispatch loop. They do
not use Go recursion as the Python call stack. This choice has several benefits:

- recursive Python code does not require one Go call per Python frame
- exception unwinding can walk Python frames directly
- tracebacks use the same frame chain as calls
- suspended generators retain the same frame representation used by calls

A generator call binds arguments and creates a generator that owns a detached
frame. Iteration attaches that frame to the caller. `yield` detaches it again
and returns one value. `FOR_ITER` and `next` resume a suspended yield expression
with `None`; `send` supplies its argument instead. A new generator accepts only
`None` as its first sent value. `throw` routes an exception through the protected
ranges surrounding the suspended `yield`; throwing into a new generator skips
its body. `close` injects `GeneratorExit`, suppresses it if it escapes, and
returns a value produced while handling it. Yielding during close raises
`RuntimeError` but leaves the generator suspended. A normal return exhausts a
loop; explicit resumption exposes its value through `StopIteration.value`, while
`next` may return a supplied default. Any other escaping exception completes
the generator, and later iteration remains exhausted. The runtime rejects
re-entry and converts an explicit `StopIteration` escaping generator code into
`RuntimeError`. The deprecated three-argument `throw` form accepts `None` as its
traceback because Python traceback objects do not exist yet.

`yield from` uses a send loop in the outer generator frame. It delegates ordinary
iteration and sent values, exposes a generator delegate's return value, and
routes delegate failures through the outer generator's handlers. `throw`
traverses nested generator delegates before recording the injection site. If a
native delegate has no `throw` method, the exception enters the outer generator
at the suspended expression.

`GeneratorExit` first closes active generator delegates from the inside out,
then enters the outer generator at the suspended expression. `close` uses this
path before closing the outer generator; `throw(GeneratorExit())` preserves and
propagates the exception supplied by the caller. A delegate that yields while
closing raises `RuntimeError`. A different delegate failure enters the outer
generator instead. Native iterators have no close operation and are skipped.
The protocol does not yet expose a general `iter` builtin, and garbage
collection does not implicitly close an abandoned generator.

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

Integers use arbitrary precision, but one exact power operation may produce at
most 1,048,576 bits. Larger results raise `OverflowError` before Go's allocator
can terminate the process. CPython has no matching fixed limit and relies on
allocation failure. A runtime-wide resource budget may replace this local limit
later.

A future Go-defined type must use the same attribute, call, iteration, equality,
and exception paths as a Python-defined type. A smaller second object model for
native values would create two subtly different languages.

Python containers cannot be represented as plain Go maps or slices forever.
Hashing, equality, attribute access, and descriptors may call Python code or
raise exceptions. Container and object implementations must support those calls
when the protocols are added.

## Exceptions

Python exceptions are runtime values, not Go errors. An instruction can
advance, call another Python frame, yield, return, or raise. Go errors are
reserved for invalid bytecode and host-level failures.

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
immutable code and package metadata. Each runtime begins with a small cached
`__future__` module so recognized future statements retain their import and
binding behavior without a loader. For a dotted absolute name, the runtime
loads each parent first, verifies that it is a package, and publishes each child
on that parent. Relative from-imports resolve their level against the executing
module's package name. A from-import also tries a missing package attribute as a
child module. Wildcard imports honor an explicit `__all__` list and load listed
package children. Modules execute in the existing frame loop. Repeated imports
reuse one object. Because the cache entry exists before execution, circular
imports see the names assigned so far. If execution fails, the runtime removes
only that module; dependencies that finished successfully remain cached.

The filesystem loader searches configured roots for top-level modules and
regular packages. Within one location it prefers `name/__init__.py` over
`name.py`. Child lookup uses the parent package's recorded search locations
rather than restarting at global roots. The loader decodes and compiles files
outside the runtime. Source modules and statically linked Go modules should
enter through the same runtime loading path. Namespace packages,
Python-visible `sys.modules`, advanced `importlib` hooks, zip imports, reload,
and bytecode caches should be added only when package tests require their
observable behavior.

Selected CPython standard-library modules are copied unchanged into the
versioned `stdlib` directory. Bullsnake copies only the upstream public test
cases it intends to run and records any adaptation beside the test. These tests
use the same Go runner and interpreter path as other source execution tests.

## Go embedding and extensions

A public Go API is not implemented yet. Its intended shape is small:

- create an isolated runtime
- compile or execute source
- import a module
- exchange supported values
- call Python functions
- expose Go functions, types, and modules

The public value and extension contracts should not expose internal code
objects, frames, or object layouts. Go modules should register explicitly with
a runtime or builder. Static linking is the first distribution model.

A Go callback may call back into Python only through an execution context owned
by the runtime. Background goroutines may finish host work and post a result,
but they must not mutate Python objects directly.

## Async and Python threads

Async execution and Python threads are design directions, not implemented
features.

Synchronous generators already retain suspended Python frames and resume through
the VM's ordinary frame loop. Future coroutine and async-generator work should
extend that state model with delegated iteration, awaiting, cancellation, and
asynchronous iteration. An event loop will manage ready tasks, timers, I/O
completion, cancellation, and task context. Async tasks will not be modeled as
one goroutine each because Python task scheduling and cancellation need explicit
interpreter state.

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
External resources can use explicit `close()` methods and synchronous context
managers. Go-side lifecycle APIs remain necessary for host resources. Weak
references may be added if package tests need them, but Python callbacks would
run later at a safe VM point, never on a Go cleanup goroutine.

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

## Repository boundaries

The current high-level layout is:

```text
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
runtime's loader contract, while neither core package depends on it. A future
public package will configure these pieces without making internal packages
public.

The runtime should remain one coarse internal package until imports, scheduling,
or builtins have an independent API or dependency reason to split.

## Verification

Bullsnake uses several kinds of evidence:

- focused Go tests for package behavior and internal invariants
- checked-in Python source fixtures that pass through the complete pipeline
- negative fixtures for Python errors and unsupported behavior
- direct malformed-bytecode tests for the runtime validator
- CPython-derived conformance cases for selected behavior
- selected vendored CPython standard-library sources and public tests
- fuzz tests for parsers and other input boundaries
- future package tests against pinned package and dependency versions

Comparisons with CPython should cover behavior Bullsnake intends to share.
Object IDs, hash values, finalization timing, private frames, exact error text,
and Bullsnake bytecode should be compared only when the documented contract
promises them.

## Open decisions

The project still needs concrete decisions about:

- the first package compatibility set
- which standard-library module to vendor next
- the public Go embedding and extension API
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
