# Bullsnake implementation guide

Status: living implementation notes

This document describes the code that exists today. It explains how source
moves through the repository, what each package owns, which Python behavior can
run, and where execution stops. For long-term goals and design reasons, read the
[architecture](architecture.md).

Bullsnake is under active development. Parsing a program does not mean the
compiler can translate it, and compiling it does not mean the runtime can
execute every instruction. Each stage rejects behavior it does not yet own.

## Current state

| Part | What works now |
| --- | --- |
| Source loading | Python encoding cookies, UTF-8 BOM handling, and selected legacy encodings |
| Lexer | Initial Python 3.14 tokenization, including f-strings and template strings |
| Parser | A broad Python 3.14 statement and expression grammar |
| Resolver | Name scopes, closures, contextual checks, annotations, generics, and comprehensions |
| Compiler | A synchronous executable subset with functions, classes, imports, and exceptions |
| Runtime | Modules, values, collections, functions, basic classes, and structured exceptions |
| Imports | Regular packages and modules from configured filesystem roots |
| Go API, broad standard library, async, and REPL | Not implemented |

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

Raw file bytes first pass through `internal/compiler/source`. No public package
combines these calls yet. Tests and internal callers compose them directly.
`internal/importer.FileSystem` uses the same sequence for module files found
under configured roots, then supplies the resulting code through the runtime's
loader callback.

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

AST nodes carry source spans and syntax facts needed by later stages. The module
root also retains the decoded source string, allowing a span to recover text
without another file read. Nodes do not contain resolved names, bytecode
positions, or runtime values. `ast.Dump` provides a deterministic description
for tests. The AST is private and may change when a later stage needs a clearer
representation.

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
expressions, deferred annotations, generic scopes, and pattern captures. Under
`from __future__ import annotations`, function annotation scopes still check
context-sensitive syntax but do not create name uses, free variables, or
closure cells. The resolver also records generator and coroutine flags and
checks where control-flow and suspension syntax may appear. Unknown names
remain implicit globals for runtime lookup.

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

The compiler tracks operand-stack depth while emitting. Labels patch forward
jumps and require every incoming control-flow path to agree on the stack depth.
Protected instruction ranges record where an exception handler starts and how
much stack to retain. The runtime validates those claims independently.

The current compiler translates:

- scalar literals, f-strings, and tuple, list, set, and dictionary displays
- eager list, set, and dictionary comprehensions with filters, nested clauses,
  destructuring, isolated targets, closure captures, and enclosing
  assignment-expression targets
- lazy generator expressions with the same synchronous clause and scope rules
- names, attributes, calls, subscriptions, slices, operators, comparisons, and
  conditional expressions
- simple, chained, destructuring, annotated, augmented, and deletion targets
- `if`, synchronous `while` and `for`, loop `else`, `break`, and `continue`
- synchronous `with`, including multiple managers, exception suppression, and
  cleanup during return or loop transfer
- structural matching with literal and dotted-value tests, capture, wildcard,
  AS and OR patterns, guards, fixed or starred tuple/list sequences, dictionary
  patterns with `**rest`, and class patterns with positional or named fields
- synchronous functions, lambdas, every parameter kind, defaults, decorators,
  lexical closures, returns, and lazy function annotations
- synchronous generator functions with lazy calls, `yield`, `yield from`,
  iteration, sent values, closure captures, and cleanup across suspension
- basic classes with decorators, bases, class keywords, methods, enclosing
  closures, lazy class annotations, and the cells used by class-visible
  annotations and `__class__`
- ordinary imports and assertions
- ordinary exception handlers, `else`, `finally`, exception groups, `except*`,
  bare reraising, explicit causes, and cleanup during return or loop transfer

Integer constants use arbitrary precision. Float and imaginary constants use
binary64. String decoding preserves lone surrogate escapes in an internal
WTF-8-compatible form, while bytes constants preserve arbitrary bytes.
Formatted-string compilation retains conversion, format-specification, raw
prefix, and debug-field behavior needed by the current runtime formatter.

Functions, generator functions, class bodies, and comprehensions are child code
objects. Closures contain explicit cell references instead of Go closures.
Calling a generator creates its runtime object without executing its child code.
For every comprehension, the enclosing code evaluates the first iterable and
passes its iterator to the child. Eager children build and return a collection;
generator-expression children yield values lazily. Each child owns its target
names.

Without `from __future__ import annotations`, deferred annotation bodies are
also children. Function annotations do not run during an ordinary definition or
call. The runtime exposes the generated child as `f.__annotate__`; the first
`f.__annotations__` access calls it with format `1`, requires a dictionary, and
caches that object. An unannotated function receives one stable empty
dictionary. Failed evaluation is retried. Class bodies record each executed
simple annotation and publish a lazy `__annotate__` child that can read the live
class namespace, enclosing cells, globals, and builtins. With the future import,
module and class scopes create `__annotations__` dictionaries eagerly. Each
executed simple-name annotation stores its retained source spelling without
evaluating the expression. Complex annotation-only targets do nothing in this
mode.

The compiler rejects template-string execution, future function annotation
strings, generic definitions, async definitions, asynchronous comprehensions,
`async for`, `async with`, and coroutines. Unsupported AST forms return compiler
errors; they are not approximated with similar bytecode.

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

An instruction can advance, call another Python frame, yield, return, or raise.
A call switches the loop to a new frame. A return restores the caller and pushes
the result. Python recursion therefore does not recurse through the Go call
stack.

A generator owns a frame that is detached while created or suspended. `FOR_ITER`
and `next` attach it to the caller and start or resume execution. `YIELD_VALUE`
removes the yielded value, detaches the frame, and gives the value to the caller.
Iteration and `next` supply `None` as the yield expression's result; the bound
`send` method supplies its argument. A newly created generator rejects a first
sent value other than `None`. The bound `throw` method injects an exception at
the suspended yield and lets the existing protected ranges catch it. Throwing
into a new generator skips its body. The bound `close` method injects
`GeneratorExit`; an uncaught exit returns `None`, while a caught return value
becomes the close result. Yielding during close raises `RuntimeError` and leaves
the generator suspended. Other returns and escaping exceptions complete the
generator; repeated iteration then stays exhausted. Explicit resumption raises
`StopIteration` with the return value, while `next` may return its optional
default. Re-entering a running generator raises `ValueError`. The legacy
three-argument `throw` form accepts only `None` for its traceback until Python
traceback objects exist. Garbage collection does not implicitly close abandoned
generators.

`yield from` keeps the delegate below each yielded value on the outer frame's
operand stack. `SEND` forwards `None` or a sent value, falls through when the
delegate yields, and jumps with the delegate's return value when it completes.
Delegate exceptions enter the outer generator's ordinary protected ranges.
`throw` walks nested generator delegates; a native delegate without `throw`
receives the exception in the outer generator instead.

For `GeneratorExit`, the VM closes nested generator delegates from the inside
out and retains the exception that must enter each outer generator. This path
supports both `close` and `throw(GeneratorExit())`. A delegate return or uncaught
`GeneratorExit` continues closing the outer generator. A yielded value becomes
`RuntimeError`, while another exception enters the outer generator at `SEND`.
Native iterators have no close operation and are skipped.

A raised Python exception follows protected ranges in the current code. If a
range matches, the VM trims the operand stack to its recorded depth, pushes the
exception, and resumes at the handler. Otherwise it removes the frame and
continues at the caller's call instruction. An uncaught exception retains
Python traceback entries and crosses the host boundary as
`UncaughtException`.

Frames also track exceptions active inside handlers and final suites. This makes
bare `raise`, implicit context, explicit causes, and replacement by a newer
return, loop transfer, or exception work across nested calls and cleanup.

## Runtime values

Runtime values implement a sealed `Value` interface. Current concrete values
include the Python singletons, arbitrary-precision integers, binary64 floats,
complex numbers, strings, bytes, tuples, lists, dictionaries, sets, slices,
iterators, generators, modules, functions, classes, instances, bound methods,
and exceptions.

The object model implements the behavior needed by the executable subset.
Collections support displays, unpacking, iteration, membership, integer and
slice subscription, and dictionary item mutation. Current values also support
truth testing, selected scalar operations and comparisons, and attribute access
for modules, basic classes, and instances. Dictionaries and sets currently use
ordered linear storage. This keeps Python identity and equality checks explicit
until user-defined hashing and equality can call back into Python.

Strings index and iterate by decoded code point, including preserved lone
surrogates. Bytes index and iterate as integers. Slices use Python-style bound
clipping and positive or negative steps. Dictionary iteration detects key-set
changes; replacing an existing value is allowed. Set display and iteration
order is stable for Bullsnake tests but is not a Python compatibility promise.

Integer arithmetic includes exact addition, subtraction, multiplication,
floor division, modulo, shifts, bitwise operations, and power. A nonnegative
integer exponent returns an integer, subject to the documented 1,048,576-bit
result limit. A negative integer exponent follows Python's real-number behavior
and returns a binary64 float. Integer and boolean operands share these rules;
float and complex power remain unsupported.

Current float arithmetic covers addition, subtraction, multiplication, true
division, and modulo. These operations coerce integer and boolean operands when
a float participates; true division also converts two integer operands.

The builtin namespace contains the current exception classes, scalar numeric
`int`, positional `max` and `min` calls with two or more arguments, and `next`
for generators and existing internal iterators. `next` accepts one optional
default. String and base forms of `int`, the iterable and keyword forms of `max`
and `min`, and the general `iter` builtin remain unsupported.

The current function binder supports positional-only, positional, keyword-only,
`*args`, and `**kwargs` parameters, positional and keyword-only defaults, and
keyword unpacking. Defaults retain the objects created when the definition ran.
Calls reject duplicate, missing, unexpected, or non-string keyword arguments
with Python exceptions. Generator calls use the same binding path but retain the
new frame without running its body.

Classes support one base, inherited attribute lookup, bound Python methods,
ordinary `__init__`, instance and class attribute mutation, lazy class annotation
callables, future annotation dictionaries, and user exception subclasses. A
generated `C.__annotate__(1)` returns annotations from statements that ran in a
non-future class body; an explicit class method of that name wins. The first
`C.__annotations__` access requires and caches the generated dictionary.
Explicit class dictionaries, including dictionaries created by the future
annotations compiler path, take precedence. Failed lazy evaluation is retried.
Synchronous context managers look up `__enter__` and `__exit__` on that class
chain, ignoring same-named instance attributes. The object model does not yet
implement complete annotation attribute mutation rules, class keyword
arguments, multiple inheritance, C3 method order, metaclasses, `super`,
`__new__`, or general descriptors. Custom exception initializers and methods
remain unsupported.

The formatter supports current strings, integers, booleans, and floats for the
format forms covered by execution tests. It does not yet provide general
`__format__` dispatch.

## Exceptions

Python exceptions are `Value` implementations. The runtime has the built-in
exception classes needed by current operations and follows their inheritance
when matching handlers. `raise` accepts an exception instance or a supported
exception class. Bare `raise` uses the active handled exception.

`StopIteration` stores its `value` for `next`. A generator return creates that
exception only for explicit `next` calls; loop iteration consumes completion
internally. If generator code lets a `StopIteration` exception escape, the
runtime raises `RuntimeError("generator raised StopIteration")` with the original
exception as its cause and context.

Ordinary `try` statements support ordered typed or bare handlers, `as` bindings,
`else`, and `finally`. Handler bindings clear on every exit. Final suites and
context-manager exits run for normal completion, propagation, return, break,
and continue. A transfer started in a final suite or exit method replaces the
pending transfer. An exceptional `__exit__` call receives the exception class
and instance, but receives `None` for its traceback argument until Python
traceback objects exist.

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
many components raises the beyond-top-level form. Each runtime also starts with
a cached `__future__` module. It exposes marker values for the feature names the
resolver accepts, so future statements retain ordinary import and binding
behavior without a filesystem loader.

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

A missing callback, missing file, or missing callback result raises
`ModuleNotFoundError`. Filesystem and frontend failures remain Go host errors
until the runtime has the corresponding Python exception values. Namespace
packages, dynamic `__path__` changes, `sys.modules`, finder and loader hooks,
reload, import locks, and a general standard-library distribution remain
unimplemented.

Bullsnake vendors selected CPython 3.14.7 standard-library modules under
`stdlib/3.14`. The first module is the unchanged `colorsys.py`. Its Go test
executes a selected upstream public-behavior case through the filesystem loader
and complete interpreter pipeline.

## Deliberate boundaries

The largest current gaps are:

- no public Go embedding or extension API
- no namespace packages, broad standard library, or native extension loading
- no general `iter` builtin or automatic generator closing during Go garbage
  collection
- no asynchronous comprehensions, coroutines, async execution, or Python
  threads
- no asynchronous context managers
- no complete Python object protocol, descriptors, user hashing, or multiple
  inheritance
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

Bullsnake does not implement CPython reference counting, `__del__`, weak
references, or a compatible `gc` module. The experiments under
`experiments/gcprobe` inform these boundaries but are not production runtime
code.

## Testing

The repository uses several test layers:

- focused Go tests for package APIs, spans, dumps, copying, and invariants
- CPython-derived lexer and parser cases pinned to Python 3.14.7
- selected vendored CPython standard-library modules and public test cases
- resolver cases that compare complete deterministic scope dumps
- compiler cases that compare complete deterministic code-object dumps
- chunked Python execution fixtures that pass through parser, resolver,
  compiler, validator, and VM
- expected-error fixtures that check Python exception type and message
- hand-built malformed-bytecode cases that exercise runtime validation
- fuzz tests at lexer, parser, and resolver input boundaries

The checked-in conformance data pins CPython 3.14.7 at commit
`823f0323ee6ec1402088b73bce1a38473cac36dc`. It runs offline and requires no
Python binary, CPython checkout, network access, or generation step.

Run the repository checks with:

```sh
go test ./...
go vet ./...
go tool laas -funcdoc.limit=5 -exclude-packages='(^|/)experiments($|/)' ./...
git diff --check
```

Behavior changes should start with a focused source or package test, update the
relevant implementation notes, and land as one coherent vertical slice.
