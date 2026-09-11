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
| Compiler | Executable functions, classes, imports, exceptions, generators, and basic coroutines |
| Runtime | Modules, values, collections, functions, basic classes, exceptions, and suspended frames |
| Imports | Regular packages and modules from configured filesystem roots |
| Go API, broad standard library, async scheduling, and REPL | Not implemented |

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
template strings, `await`, and `yield`. `match` and `case` remain ordinary names
when their surrounding tokens form a valid simple statement. This is a grammar
claim, not an execution claim. Later stages still reject unsupported forms.

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

The resolver decides what a name means. It also exposes each scope's private-name
rewrite so the compiler uses one spelling for bare names, attributes, keyword
arguments, annotation keys, and class-pattern fields. The resolver does not
assign bytecode indexes or choose load and store instructions. Those decisions
belong to the compiler.

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
- eager list, set, and dictionary comprehensions with synchronous or asynchronous
  clauses, filters, nested clauses, destructuring, isolated targets, closure
  captures, and enclosing assignment-expression targets
- lazy synchronous and asynchronous generator expressions with filters, nested
  clauses, closure capture, and isolated targets
- names, attributes, calls, subscriptions, slices, operators, comparisons, and
  conditional expressions
- simple, chained, destructuring, annotated, augmented, and deletion targets
- `if`, synchronous `while`, synchronous and asynchronous `for`, loop `else`,
  `break`, and `continue`
- synchronous and asynchronous `with`, including multiple managers, exception
  suppression, awaited async entry and exit, and cleanup during return or loop
  transfer
- structural matching with literal and dotted-value tests, capture, wildcard,
  AS and OR patterns, guards, fixed or starred tuple/list sequences, dictionary
  patterns with `**rest`, and class patterns with positional or named fields
- synchronous functions, lambdas, every parameter kind, defaults, decorators,
  lexical closures, returns, lazy function annotations, and generic functions
  with `TypeVar`, `TypeVarTuple`, and `ParamSpec` parameters, including lazy
  bounds, tuple constraints, and defaults
- type aliases with lazy values and definition-scope captures, including
  `TypeVar`, `TypeVarTuple`, and `ParamSpec` parameters with lazy defaults;
  ordinary TypeVars also support lazy bounds and tuple constraints
- synchronous generator functions, including generic functions, with lazy calls,
  `yield`, `yield from`, iteration, sent values, closure captures, and cleanup
  across suspension
- basic coroutine functions, including generic definitions, with lazy calls,
  ordinary argument binding, closure captures, direct protocol execution, and
  `await` between native Bullsnake coroutines
- asynchronous generator functions, including generic definitions, with lazy
  calls, arguments, closure captures, `yield`, inner `await`, direct `__aiter__`
  and `__anext__`, `async for` consumption, and cleanup across suspension
- basic classes with decorators, bases, class keywords, methods, enclosing
  closures, lazy class annotations, all three PEP 695 parameter kinds with lazy
  metadata, and cells for class-visible annotations and `__class__`
- ordinary imports and assertions
- ordinary exception handlers, `else`, `finally`, exception groups, `except*`,
  bare reraising, explicit causes, and cleanup during return or loop transfer

Integer constants use arbitrary precision. Float and imaginary constants use
binary64. String decoding preserves lone surrogate escapes in an internal
WTF-8-compatible form, while bytes constants preserve arbitrary bytes.
Formatted-string compilation retains conversion, format-specification, raw
prefix, and debug-field behavior needed by the current runtime formatter.

Functions, generator functions, coroutine functions, async generator functions,
class bodies, and comprehensions are child code objects. Closures contain
explicit cell references instead of Go closures. Calling a generator, coroutine,
or async generator creates its runtime object without executing its child code.
For every comprehension, the enclosing code evaluates the first iterable and
passes its iterator to the child. A synchronous eager child builds and returns a
collection; an asynchronous eager child is a coroutine that the enclosing
coroutine awaits. A synchronous generator-expression child returns a generator;
an asynchronous one returns an async generator. Both yield values lazily, and
each child owns its target names.

Without `from __future__ import annotations`, deferred annotation bodies are
also children. Function annotations do not run during an ordinary definition or
call. The runtime exposes the generated child as `f.__annotate__`; the first
`f.__annotations__` access calls it with format `1`, requires a dictionary, and
caches that object. An unannotated function receives one stable empty
dictionary. Failed evaluation is retried. Class bodies record each executed
simple annotation and publish a lazy `__annotate__` child that can read the live
class namespace, enclosing cells, globals, and builtins.

With the future import, function annotation children return retained source
spellings without evaluating or capturing names. Module and class scopes create
`__annotations__` dictionaries eagerly. Each executed simple-name annotation
stores the same source-backed string. Complex annotation-only targets do
nothing in this mode.

A type alias stores a hidden zero-argument child function instead of evaluating
its value at the statement. That child uses the alias definition's globals,
enclosing cells, and visible class namespace. The runtime evaluates it on the
first `Alias.__value__` access and caches the returned object. A raised exception
leaves the alias unevaluated so a later access retries it. A generic alias adds
an outer hidden child that creates fresh type parameters and closes the value
child over them. Bound, tuple-constraint, and default expressions use their own
lazy children with the same scope rules.

A generic function also uses an outer hidden child. It creates `TypeVar`,
`TypeVarTuple`, and `ParamSpec` objects. Parameters used by the function body or
lazy annotation callable live in cells. The child attaches the same objects as
one stable `f.__type_params__` tuple and returns the function. Future annotations
retain source strings and require no type-parameter capture.

Default expressions run in the defining scope before type-parameter creation.
Their completed tuple and keyword map pass into the hidden child without
reevaluation. Decorator expressions run first in source order, then their values
wrap the completed generic function in reverse order. Decorators, defaults, and
annotations use their ordinary function code paths. Every parameter kind keeps
the ordinary local layout and call binding.

An ordinary TypeVar bound, tuple constraint, or default uses the same lazy child
and cache behavior as a generic alias. Type parameter names do not enter the
defining namespace. The hidden child's name does not alter user-facing function
or annotation qualified names.

Template strings compile to immutable template and interpolation values. Each
interpolation retains its evaluated value, exact expression text from the source
span, conversion marker, and evaluated format-spec string. Literal segments form
one tuple with exactly one more entry than the interpolation tuple. Debug fields
append their source text to the preceding literal and use the same default `r`
conversion as f-strings.

Unsupported AST forms return compiler errors; they are not approximated with
similar bytecode.

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

Native coroutine objects reuse the detached-frame state machine but remain
separate from Python iteration. Calling a coroutine function binds its arguments
without running the body. Its `send` method starts or resumes the frame, and a
normal return raises `StopIteration` with the returned value. A first send must
be `None`; a completed coroutine cannot be reused. `next`, `for`, and `GET_ITER`
reject coroutines.

`await` evaluates its operand, requires a native Bullsnake coroutine or an
async-generator next awaitable, and uses a `SEND` loop to run it. A nested return
becomes the await-expression value; exceptions enter the awaiting coroutine's
ordinary handlers. User-defined `__await__` methods, scheduler-facing awaitables,
and task execution remain unsupported.

An async generator owns a third kind of detached frame. Calling its function is
lazy and returns an `async_generator`, which is an async iterator but neither a
synchronous iterator nor an awaitable. `__anext__` returns a one-shot awaitable;
`asend(value)` creates the same awaitable with a value for the suspended `yield`
expression. `athrow(exception)` creates a separate one-shot awaitable that
injects a normalized exception at the suspended yield. `aclose()` uses that
awaitable to inject `GeneratorExit`, run cleanup, and suppress normal close
completion. A caught injection may yield another item; an uncaught injection
completes the generator and propagates. Yielding while handling close raises
`RuntimeError`; a replacement close exception propagates. A wrapped user `yield`
completes the active protocol awaitable with the yielded item, while an ordinary
suspension from an inner `await` continues through the caller. A new async
generator accepts only `None`; a rejected first send closes that awaitable
without closing the generator. Throwing or closing a new async generator skips
its body and closes it. Normal return completes iteration with
`StopAsyncIteration`. An explicit `StopIteration` or `StopAsyncIteration`
escaping the body becomes `RuntimeError`.

`async for` calls class-level `__aiter__` synchronously, requires its result to
provide `__anext__`, and awaits each native-coroutine next result. A protected
range surrounds only that next-item operation. `StopAsyncIteration` there ends
the loop and enters `else`; the same exception from a target or body propagates.
Break skips `else`, while continue and nonlocal cleanup retain the iterator.

An eager asynchronous comprehension uses a hidden coroutine child. The
surrounding coroutine evaluates and converts the first iterable, calls the
child, and awaits its collection result. Each asynchronous clause awaits
`__anext__` with the same narrow `StopAsyncIteration` protection as `async for`.
Synchronous and asynchronous clauses may be nested in either order. Filters and
result expressions run inside the child, so they may also contain `await`.

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

### Python frame inspection

`sys._getframe` returns stable Python identities for actual VM frames and walks
the Python caller chain for an optional integer/index-protocol depth. Frames
expose read-only `f_back`, `f_lineno`, `f_globals`, `f_builtins`, and `f_locals`.
Module and class locals return the actual namespace dictionary; retained class
frames keep the prepared dictionary, not the finished class's copied namespace.

Optimized functions return fresh FrameLocalsProxy wrappers over their live fast
locals and closure cells. Writes change lexical slots, including unbound slots;
extra keys are shared across proxies without becoming lexical variables. Deleting
lexical slots through the proxy raises ValueError. Ordinary lexical deletion
remains visible. Iteration and keys/items/values return snapshots; copy, get, pop,
setdefault, and dict/proxy update forms work. Frames and proxies retain typed Go
references after return. General mapping updates, proxy comparison/union,
frame/code constructors, `f_code`, clearing, traceback links, tracing, and debugger
mutation remain later work.

## Runtime values

Runtime values implement a sealed `Value` interface. Current concrete values
include the Python singletons, arbitrary-precision integers, binary64 floats,
complex numbers, strings, bytes, tuples, lists, dictionaries, sets, frozen sets,
slices,
iterators, generators, modules, functions, native and user classes, instances,
bound methods, and exceptions.

The one-argument `type` form returns stable native class objects for Go-backed
values, the defining class for a user instance, and the concrete class for an
exception. Native types and built-in exception classes expose `__name__`,
`__qualname__`, and `__module__`; user classes expose the corresponding compiler
and class-builder metadata. `type(type) is type`. The native `object` constructor
returns a distinct root instance and accepts no arguments. Native values,
built-in exception classes, and ordinary user classes are instances or
subclasses of `object`. A user class with no named base, or with `object` as its
sole base, exposes `object` through `__base__`, `__bases__`, and `__mro__`.
A user class may instead use `classmethod`, `staticmethod`, or `property` as its
sole native base. That native ancestry appears in class metadata and
`issubclass`. Descriptor subclass construction and Python initialization now
work for the documented wrapper subset. Other native bases and mixed native/user direct bases remain
unsupported. The `bool`, `int`, `str`, and `range` bindings are native type
objects and retain their implemented constructor behavior. `range` accepts one
to three integer or boolean arguments,
rejects a zero step, and retains arbitrary-precision `start`, `stop`, `step`, and
length values. Iteration is lazy and gives each iterator independent state.
`len` raises `OverflowError` when the range length does not fit the host index
size. `enumerate` accepts one iterable and an optional integer or boolean start,
resolves the iterator during construction, and retains an arbitrary-precision
index. It is its own iterator. Pulling an indexed pair may suspend in a generator
or user `__next__` frame. `map` currently accepts one iterable and applies its
callable lazily. `filter` accepts one predicate and iterable, retains the original
item while the predicate and its truth method run, and treats `None` as the
identity predicate. Pulling either kind of item may suspend in the source
iterator or in Python call and truth frames. `zip` accepts any number of
positional iterables, resolves their iterators from left to right at construction,
and lazily yields tuples until the shortest source is exhausted. Multiple map
iterables and Python 3.14's `strict` modes for map and zip remain unsupported.
User `__index__` conversion, range subscription, and range-specific methods also
remain unsupported.

The `list`, `tuple`, `set`, `frozenset`, and `dict` type objects accept zero or
one positional source. Sequence, set, and frozen-set constructors collect
native, user, or generator iterators through the frame loop.
`tuple(existing_tuple)` and `frozenset(existing_frozenset)` preserve identity;
list and set construction returns a new value. Set and frozen-set finalization
uses the same hashability and duplicate rules as set displays. Dict construction
copies a native dictionary or consumes tuple/list key-value pairs, then applies
keyword values. User-defined mapping objects and arbitrary iterable inner pairs
are not implemented yet. `isinstance` checks native identity, the C3 ancestry of
a user instance, and built-in or user exception ancestry. `issubclass` applies
those same ancestry rules directly to class objects unless a metaclass hook overrides them. A tuple of candidates is
processed left to right and may contain nested tuples; a match suppresses errors
from later entries. `bool` is a native subclass of `int`. Type unions remain unsupported. Metaclass `__instancecheck__` and
`__subclasscheck__` methods now run through resumable calls and truth conversion. Three-argument `type`
uses the same class builder as a class statement. It accepts a string name, a
tuple of currently supported bases, and a dictionary with string keys. It
supplies default module and qualified-name metadata, honors explicit values,
and supports methods, C3 inheritance, and user exception classes. `__mro_entries__` and non-string namespace keys remain unsupported.
The `dir` builtin returns sorted bound names from the current frame or from a
module, user class MRO, or instance namespace. It includes computed class
metadata and does not yet invoke custom `__dir__`. The remaining built-in type
constructors are not implemented yet.

The object model implements the behavior needed by the executable subset.
Collections support displays, unpacking, iteration, membership, integer and
slice subscription, and dictionary item mutation. String instances expose bound
`capitalize`, `count`, `endswith`, `format`, `join`, `lower`, `removeprefix`,
`replace`, `split`, `splitlines`, `startswith`, and `strip`.
Join collects through the resumable iterator path before validating all items.
Format supports automatic and numbered positional fields, attribute paths,
escaped braces, and `!s`, `!r`, or `!a` conversion. It rejects mixing automatic and
manual numbering. User attribute, string, and representation methods resume
through the frame loop. Named, indexed, nested,
specified, and custom `__format__` fields remain unsupported. Count uses
non-overlapping matches inside integer-or-boolean code-point slice bounds.
Replace accepts an integer-or-boolean positional or keyword count, handles empty
patterns at code-point boundaries, and retains no-op identities. Removeprefix
removes one exact nonempty prefix and otherwise retains the original string
object. Split
handles explicit separators and Python whitespace with the integer-or-boolean
`maxsplit` subset. Splitlines recognizes Python's Unicode line-boundary set,
folds CRLF into one boundary, and resolves `keepends` through ordinary truth
testing. Prefix and suffix matching apply code-point bounds from the current
integer-or-`None` index subset to a string or an ordered tuple. Strip removes
Python whitespace or a supplied code-point set. Capitalization titlecases the
first code point and applies contextual lowercase mappings to the remainder.
Lowercase conversion uses full Unicode mappings and preserves lone-surrogate
bytes. Both operations use `golang.org/x/text` Unicode 17, so code points whose
casing changed after CPython 3.14's Unicode 16 baseline may differ.

List instances expose bound `append`, `pop`, `extend`, `remove`, and `sort`
methods. Extend consumes native, generator, or user iterators through the frame
loop and mutates the target as each item arrives. Remove scans left to right,
prefers identity, and resumes user `__eq__` and truth methods through the same
frame loop. The `sorted` builtin collects any current iterable into a new list,
evaluates an optional key once per item from left to right, and supports reverse
ordering via ordinary truth conversion. Sorting is stable in both directions.
User key, `__lt__`, and comparison-truth methods may suspend through the frame
loop. The cached `_functools.cmp_to_key` helper creates callable `KeyWrapper`
values. Their six rich comparisons call the saved comparator and compare its
result with zero through that same frame loop. `list.sort` uses the shared sort
behavior, returns `None`, and replaces the target contents only after success.
Bullsnake does not yet expose an empty target to sort callbacks or detect target
mutation during sorting as CPython does.

List item assignment now accepts native integer and boolean indexes, including
negative indexes, and preserves list and assigned-element identity. Invalid
indexes leave the list unchanged. Slice assignment, deletion, and user-defined
`__index__` conversion are still unsupported. This closes the indexed replacement
operation used by unchanged `heapq.heapify` and `heappop`.

Dictionary instances expose bound `clear`, `copy`, `get`, `pop`, `items`,
`keys`, `update`, and `values` methods. Copy clones ordered entry storage while
retaining key and value identities. The view methods return live `dict_items`,
`dict_keys`, and `dict_values` values with independent iterators. Replacing a
value remains visible. Key-set changes, including clearing a nonempty dictionary,
raise `RuntimeError` in an active iterator. Update accepts a native dictionary
and keyword entries, preserving existing key positions; iterable pairs and user
mappings remain unsupported. Set instances expose bound `add`, `difference`, and
`discard` methods; frozen sets expose `difference`. Difference returns a new
collection after draining each argument through the resumable iterator path.
Set and frozen-set instances expose bound `__contains__`; their native type
objects expose matching method descriptors. These methods use the same fixed
hashability and equality rules as displays and membership expressions.
The containment methods are enough for CPython's generated `keyword` module to
bind its `iskeyword` and `issoftkeyword` helpers directly from frozen sets.
Set-like view operations and other native collection or text methods are not
implemented.
Dictionary equality ignores insertion order, compares corresponding values with
identity preference, and resumes direct Python value equality and truth callbacks.
Inequality negates that equality result rather than calling each value's `__ne__`.
Nested dictionaries, lists, and tuples use the same resumable equality path.
Recursive container pairs and nesting beyond 1000 comparisons raise RecursionError;
this local bound is not a global Python recursion-limit API. Key lookup retains
fixed hashability/equality rules.

List and tuple equality also resumes element equality and truth callbacks,
prefers identical elements, and negates equality for inequality rather than
calling element `__ne__`. Lists reject unequal lengths before element comparison
and reread live storage after callbacks; tuples compare their shared prefix first.
Set and frozen-set equality and native membership retain fixed recursive rules
and do not yet suspend for user `__eq__`.
Built-in values use fixed truth and length rules. `len` supports strings, bytes,
ranges, tuples, lists, dictionaries, and sets; string lengths count decoded code
points, including preserved lone surrogates. A user instance looks up `__bool__`
on its class for truth and falls back to class `__len__`; same-named instance
attributes do not participate. The `bool` builtin uses the same lookup and
returns the resolved boolean singleton; a direct `len` call uses the same class
`__len__` path. `__bool__` must return a boolean. `__len__` must return a
nonnegative integer that fits the host index size. The special-method call can
suspend in another Python frame before the requesting operation continues.
Dictionaries and sets currently use ordered linear storage. The `hash` builtin
uses fixed hashes for current immutable native values and calls class `__hash__`
for a direct user instance. User methods must return an integer. Signed 64-bit
results are preserved except for reserved -1; larger results use integer hashing.
Defining `__eq__` without an explicit `__hash__` inserts `__hash__ = None` during
statement or dynamic class construction; later attribute assignment does not
silently change the hash slot. This keeps mutable Set/Mapping ABC subclasses
unhashable unless they explicitly supply a hash.
Frozen-set mixing matches the unchanged Set._hash algorithm, including negative
and large element hashes. `sys.maxsize` exposes the runtime's native index limit
without reading host configuration. Tuple and frozen-set hashing now use an
explicit native worklist and resume Python element hashes through the VM. Tuple
mixing and successful-result caching follow the pinned CPython 3.14 behavior;
failed hashes retry. The shared empty frozenset remains immutable. Dictionary
and set key admission still uses fixed hashability and equality checks, so this
does not yet enable arbitrary Python keys.

The `repr` builtin calls class `__repr__` for a direct user instance and requires
a string result. The object form of `str` returns strings unchanged, uses an
exception's message, calls class `__str__`, and falls back to `__repr__`. The
bytes decoding form with encoding and errors remains unsupported. Other values
use their fixed runtime representation. Rendering a built-in container does not
yet suspend to call `__repr__` on nested user values.

Strings index and iterate by decoded code point, including preserved lone
surrogates. Bytes index and iterate as integers. Slices use Python-style bound
clipping and positive or negative steps. Template strings retain parallel
`strings` and `interpolations` tuples; each interpolation exposes `value`,
`expression`, `conversion`, and `format_spec`, and a template derives its `values`
tuple from them. Iteration alternates non-empty literal strings with the original
interpolation objects and skips empty literal strings. Adding two templates joins
the touching literal strings in a new template and preserves interpolation
identity and order. The cached `string.templatelib` module exports `Template`
and `Interpolation` constructors with positional and keyword metadata binding. Its
`convert` helper returns the input unchanged for `None` and otherwise applies the
same `s`, `r`, or `a` conversion used by formatted strings.

Dictionary iteration detects key-set changes; replacing an existing value is
allowed. Set display and iteration order is stable for Bullsnake tests but is not
a Python compatibility promise.

A user iterable resolves `__iter__` on its class and requires the returned value
to have class `__next__`. The one-argument `iter` builtin and loop iteration use
the same path. The `reversed` builtin lazily traverses native lists, tuples,
strings, bytes, and ranges. List reverse iterators retain their initial index,
ignore later appends, and exhaust if a shrink makes the current index invalid.
User `__reversed__` methods now run through the VM, returning their result
unchanged and ignoring instance-level replacements. Lists and ranges expose
matching native reverse descriptors. The `__len__` plus `__getitem__` sequence
fallback remains unsupported. Each loop step or user-iterator `next()` call may run a Python
frame. `enumerate` retains that iterator and applies its index only after an item
is produced, so failed pulls do not advance the count. `map` retains its source
iterator and calls its function only after a source item is available. `filter`
retains a candidate until its predicate result has been truth-tested. All three
remain self-iterators and preserve their state across `for`, `next`, and
collection construction. `all` and `any` truth-test each item through the same
resumable protocol and stop when the result is known. `all` returns true and
`any` returns false when the iterator is exhausted. `StopIteration` leaving that
active `__next__` call means exhaustion; the same exception raised by loop body
code remains an ordinary exception. Bullsnake does not yet use `__getitem__` as
the legacy iteration fallback.

A user container resolves `__contains__` on its class and truth-tests the result,
including another user `__bool__` or `__len__` call. A class attribute set to
`None` disables containment. If the class omits `__contains__`, Bullsnake does
not yet search an iterator for an equal item.

User item reads, writes, and deletes resolve `__getitem__`, `__setitem__`, and
`__delitem__` on the class. Keys and assigned values pass through unchanged.
Mutation waits for the Python method but discards its return value, matching the
statement operation. Same-named instance attributes do not participate.

User equality and inequality resolve class `__eq__` or `__ne__`. A strict right
subclass gets the first attempt. Returning `NotImplemented` tries the other
operand, then falls back to identity. If `__ne__` is absent, Bullsnake
truth-tests and inverts `__eq__`. Ordering resolves the matching left method or
the swapped right method, such as `__lt__` and `__gt__` for `<`, and raises
`TypeError` if both decline. An explicit rich-comparison result otherwise
passes through unchanged.

Unary `+`, `-`, and `~` resolve class `__pos__`, `__neg__`, and `__invert__` for
user instances and keep the returned object unchanged. Binary operators resolve
the matching normal and reflected methods for all compiler operands, including
matrix multiplication. In-place operators first try `__iadd__` and its peers,
then use the ordinary pair. `NotImplemented` advances to the next candidate;
exhausting the candidates raises the operator-specific `TypeError`.

User descriptor instances support class `__get__`, `__set__`, and `__delete__`.
Reads apply data-descriptor, instance-attribute, non-data-descriptor, then class
attribute precedence. Class access calls `__get__` with `None` and the accessed
class. The `getattr` and `hasattr` builtins apply the same implemented module,
class, instance, and function lookup rules to a runtime string. An optional
`getattr` default and `hasattr` catch only `AttributeError`, including one
escaping a descriptor method. The `setattr` and `delattr` builtins share
ordinary module, class, instance, and function mutation rules. Python functions
retain arbitrary assigned attributes; their current annotation and type-parameter
metadata remains read-only. Instance writes and deletes invoke data descriptors
before the instance namespace and discard the descriptor method's result.

The built-in `property`, `classmethod`, and `staticmethod` names are callable
native type objects. Property supports direct construction and decorator-style
`getter`, `setter`, and `deleter` copies. Properties expose their accessor fields,
explicit documentation, and the class-assigned name. Function-docstring
inference remains unsupported. A user class may inherit one descriptor type and construct specialized wrapper
values. The wrappers retain their user-class identity and attributes, run Python
initializers, and support native initialization through `super`. Property
accessor copies reconstruct the subclass and run its initializer. General
custom descriptor-subclass special methods and data-descriptor mutation overrides
remain outside this initial wrapper subset. `classmethod` binds its
wrapped callable to the accessed class from either class or instance lookup;
`staticmethod` returns its wrapped callable unchanged. Both wrappers expose
`__func__` and `__wrapped__`.

Calling a user instance resolves class `__call__`, ignores a same-named instance
attribute, and forwards arguments through the ordinary function binder. The
`callable` builtin reports whether that class slot exists without running it.

Integer arithmetic includes exact addition, subtraction, multiplication,
floor division, modulo, shifts, bitwise operations, and power. A nonnegative
integer exponent returns an integer, subject to the documented 1,048,576-bit
result limit. A negative integer exponent follows Python's real-number behavior
and returns a binary64 float. Integer and boolean operands share these rules;
float and complex power remain unsupported.

Current float arithmetic covers addition, subtraction, multiplication, true
division, and modulo. These operations coerce integer and boolean operands when
a float participates; true division also converts two integer operands. The
`abs` builtin returns native integer, float, or complex magnitudes and dispatches
a user instance's class `__abs__` method through the frame loop. The `round`
builtin performs arbitrary-precision integer and exact-rational float half-even
rounding for an optional decimal digit count. User instances dispatch class
`__round__` through the frame loop.

The builtin namespace contains the current exception classes; native `bool`,
`int`, `str`, `range`, `enumerate`, `map`, `filter`, `zip`, `list`, `tuple`,
`set`, `frozenset`, `dict`, `object`, `type`, `classmethod`, `property`, and
`staticmethod` objects; `abs`; `all`; `any`; `callable`; `delattr`; `dir`;
`getattr`; `hasattr`; `hash`; `isinstance`; `issubclass`; one-argument `iter`;
`len`; positional `max` and `min` calls with two or more arguments; `next`;
`repr`; native-sequence `reversed`; `round`; `setattr`; `sorted`; and `sum`. The `next`
builtin accepts one optional default for generators, internal iterators, and
user iterators.
String and base forms of `int`, the iterable and keyword forms of `max` and
`min`, the encoding form of `str`, and callable-sentinel `iter` remain
unsupported.

The `sum` builtin consumes current native/user iterators and generators, accepts
an optional positional or keyword start, and performs ordinary addition through
VM callbacks without invoking in-place addition. Integer prefixes remain exact;
native float accumulation uses compensated summation after the supported integer
to float transition. String, bytes, and bytearray starts are rejected after
iterator setup. Empty input preserves the supplied start object. Complex sums
and native addition forms not yet implemented by the arithmetic layer remain
unsupported. Unchanged Sequence.count now runs through this fold, including
Python value equality in its generator.

The current function binder supports positional-only, positional, keyword-only,
`*args`, and `**kwargs` parameters, positional and keyword-only defaults, and
keyword unpacking. Defaults retain the objects created when the definition ran.
Calls reject duplicate, missing, unexpected, or non-string keyword arguments
with Python exceptions. Generator calls use the same binding path but retain the
new frame without running its body. Every function exposes one stable
`__type_params__` tuple. A generic function's tuple contains the same parameter
objects captured by its body and annotations; an ordinary function's tuple is
empty.

A type alias has runtime type name `typing.TypeAliasType`. Its repr is its
declared name. It exposes `__name__`, `__module__`, `__type_params__`, and lazy
`__value__`. A generic alias's parameter tuple contains the same parameter
objects used by its lazy value. Current ordinary TypeVars have runtime type name
`typing.TypeVar`, bare-name repr, and inferred variance. `__bound__`,
`__constraints__`, and `__default__` lazily evaluate and cache their hidden
functions; failures retry. `TypeVarTuple` and `ParamSpec` expose their names,
bare-name repr, lazy defaults, and the shared no-default marker. `ParamSpec` also
exposes inferred variance, `args`, and `kwargs`. A missing default returns one
`NoDefaultType` singleton whose repr is `typing.NoDefault`. Alias calls,
subscription, unions, public evaluator callables, direct `typing.NoDefault`
imports, and mutation of these attributes remain unsupported.

Classes support multiple user-class bases with C3 method resolution, inherited
attribute lookup, bound Python methods, ordinary `__init__`, instance and class
attribute mutation, lazy class annotation callables, future annotation
dictionaries, and generic classes with ordinary, variadic tuple, and
parameter-specification type parameters. Those parameters
support the same lazy bounds, tuple constraints, and defaults that their kinds
allow. Classes also support user exception subclasses. Zero-argument `super()`
reads the existing `__class__` cell and first positional argument. One- and
two-argument forms retain an explicit starting class and optional receiver.
Inherited lookup skips the starting class, ignores the instance namespace, and
binds methods and descriptors against the actual receiver class. Classes expose
immutable, read-only `__bases__`, `__base__`, and `__mro__` metadata. A built-in
exception class may still be the sole direct base; multiple user exception
classes use ordinary C3 ancestry.

Ordinary user-class allocation now enforces the abstract flag set by assigning
`__abstractmethods__`. Assignment evaluates Python truth before storing the
original object and the flag; a failed truth callback leaves both unchanged.
Deletion clears the flag. Reading this metadata uses only the class's own
namespace, and assignment does not update subclasses. Mutating the retained
object does not recompute the flag. A blocked constructor collects and sorts
its current names through the ordinary iterator and comparison continuations,
then raises TypeError before running `__init__`. Names must be strings.
Class-body and three-argument `type` namespace entries alone do not set the
flag. Built-in exception allocation keeps its separate behavior. This is the
allocation prerequisite for ABCMeta; virtual subclass registration is described below.

A generic class stores one stable `__type_params__` tuple in its own namespace.
Class statements and methods capture the same parameter objects. Bullsnake does
not yet add an implicit `Generic[...]` base or support class specialization. A
generated `C.__annotate__(1)` returns annotations from statements that ran in a
non-future class body; an explicit class method of that name wins. The first
`C.__annotations__` access requires and caches the generated dictionary.
Explicit class dictionaries, including dictionaries created by the future
annotations compiler path, take precedence. Failed lazy evaluation is retried.
Synchronous context managers look up `__enter__` and `__exit__` through that MRO,
ignoring same-named instance attributes. Asynchronous managers use the same
class-only rule for `__aenter__` and `__aexit__`; the runtime requires native
coroutines from both methods. Asynchronous iteration also looks up `__aiter__`
and `__anext__` on the class, and requires a native coroutine from each
`__anext__` call. The object model does not yet implement complete annotation
attribute mutation rules, generic instance `__new__`, or
custom `__getattribute__`, `__getattr__`, and `__setattr__`. Custom exception
initializers and methods remain unsupported.

The formatter supports current strings, integers, booleans, and floats for the
format forms covered by execution tests. It does not yet provide general
`__format__` dispatch.


### Abstract-method computation

The private Go `_abc` module now exposes `_abc_init` for the implemented
abstract-method computation subset. It snapshots direct class attributes,
resolves their live `__isabstractmethod__` markers, then iterates inherited names
and resolves overrides through ordinary class lookup. A successful computation
stores a frozen set and updates the allocation flag. Attribute, iterator, and
truth callbacks run in the VM; a failure leaves the previous abstract metadata
unchanged. Properties check getter, setter, and deleter markers in order;
classmethod and staticmethod markers follow their wrapped values. Bound methods
expose the underlying function's marker.

`_abc_init` now also installs fresh `_abc_impl` state. Virtual registration,
instance/subclass checks, cache tokens, and registry/cache reset helpers are
implemented. Registries and both caches hold Go weak pointers to class
allocations, with lazy pruning. Tokens are isolated per runtime; new registration
invalidates negative caches across ABCs. Checks honor subclass hooks before
nominal inheritance, then registered classes and immediate subclasses, through
ordinary VM continuations. Hooks must return bool or NotImplemented.

Source tests cover transitive registration, cycle rejection, native and exception
class registration, cache resets, mutations during callbacks, reported versus
actual instance classes, and errors. Go tests verify that actual ABC registries
and caches do not retain discarded classes and that runtime tokens are isolated.
General metaclass hashing/equality and native-base subclass enumeration remain
outside this subset.

`_get_dump` returns independent sets sharing callback-free weak class references.
They are callable, return None after collection, cache their target's hash, and
compare by live class identity; distinct dead references compare unequal. Saved
dumps do not retain their target classes. Their `__callback__` is None, unlike
CPython's private registry-removal callbacks: Bullsnake prunes on access. These
references have no public constructor and do not expose `_weakref` or `weakref`.
Python callbacks and regex compatibility remain deferred.

Unchanged `abc.py` now imports through the native helpers. The project-owned
standard-library regression test executes ABC/ABCMeta construction, modern and
legacy abstract decorators, concrete overrides, virtual and transitive
registration, structural hooks, instance checks, and cache resets. It is not
CPython's full test_abc suite. `update_abstractmethods` now runs unchanged after
method replacement and deletion, including explicit subclass recomputation.
`_dump_registry` now executes unchanged with explicit or redirected streams;
its complete report is checked against the runtime's weak-reference repr.

### Metaclass construction checkpoint

User classes may now inherit `type`. Class statements select the most-derived
compatible metaclass before executing the body, call `__prepare__`, and pass its
exact dictionary to `__new__` and `__init__`. Python factory functions are also
accepted as metaclasses. Dictionary namespaces retain body writes and deletions;
custom mapping namespaces remain unsupported. `type.__new__`, metaclass `super`,
class-cell propagation, inherited metaclass identity, and dynamic `type`
construction share the same class builder. Construction exceptions propagate
through the existing VM and are catchable at the class statement.

Native operations can retain ordered result continuations on their Python caller
while a child frame executes. They resume after the child's existing protocols
complete; error continuations run before the caller's Python exception handlers.
This supports metaclass call sequences without using the Go stack for Python
calls. Generic instance `__new__`, custom metaclass `__call__`, `__init_subclass__`,
and general metaclass descriptor precedence remain separate gaps. The tested ABC
subset and remaining API gaps are described above.

Metaclass instance/subclass hooks and their return-value truth callbacks execute
in the VM. Nested tuple candidates short-circuit in order. An exact instance
type match bypasses `__instancecheck__`; `__subclasscheck__` can override an
identical candidate and receives even a non-class first argument. Native type
check descriptors support `super` without redispatching to the override.
Non-type checker objects and custom descriptor-valued hooks remain unsupported.

### Class namespace views

Ordinary classes, including runtime-owned I/O classes, expose live read-only
`__dict__` mapping proxies. Lookup returns raw descriptors without binding and
excludes inherited attributes. Proxies support length, truth, membership,
subscription, iteration, get/copy, and live keys/items/values views. Class writes
and deletions update retained proxies; reinsertion moves a name to the end.
Iterators reuse dictionary key-change checks. Annotation evaluation publishes
its cached `__annotations__` dictionary through the same mutation path.
Native types expose per-runtime dictionaries for their currently published
metaclass methods, collection protocols, class subscription, and object subclass
hook. Reads retain descriptor identity, and dictionary reads return unbound
methods. Implemented native collection length, iteration, iterator-next, and
containment operations are exposed as receiver-checked descriptors and bound
methods. These run through the existing operations and enable unchanged
Iterable/Iterator/Sized/Container/Collection structural hooks. Methods not backed
by an implemented operation remain absent.
Hash and call slots are also published for the supported native values. Disabled
hashes remain actual None entries, and dict views/frame-local proxies reject
hashing. Object hash descriptors are inheritable and bypass Python overrides
when called explicitly; native call descriptors preserve ordinary argument and
keyword binding. These enable Hashable/Callable checks without placeholder slots.
Other native protocol descriptors, exception namespaces, proxy
construction, comparison, reverse iteration, and union operations remain later
slices.

### Class subscription

Class item access first calls the metaclass `__getitem__` slot when present, then
falls back to the class's `__class_getitem__` attribute. Python functions named
`__class_getitem__` become classmethods at class construction, including dynamic
`type` construction; later assignments retain ordinary descriptor behavior.
Both callbacks use VM continuations. Missing or disabled subscriptions raise
TypeError.

`list`, `tuple`, `dict`, `set`, `frozenset`, and `type` subscriptions construct
runtime-owned GenericAlias instances. They retain the origin and one stable
argument tuple, expose read-only `__origin__` and `__args__`, render nested
concrete aliases, and provide `__mro_entries__`. Calling the alias type directly
accepts two positional arguments. Alias subclasses run their Python `__new__`
and may delegate to GenericAlias.__new__ for typed native state. Compatible
results run the actual instance class's initializer, which must return None;
foreign __new__ results bypass initialization. Unchanged Callable aliases now
construct flattened argument metadata and round-trip their reduction tuples.
Alias calling forwards arguments to the origin and stores `__orig_class__` on
writable results. Only AttributeError/TypeError from that storage are suppressed;
origin and other descriptor failures propagate. Subclass initialization uses the
class slot, ignoring same-named instance attributes. Alias equality truth-tests
origin equality then compares the argument tuple through resumable element
callbacks; inequality negates that result and unrelated operands are declined.
Hashing combines origin and argument hashes with XOR, preserving callback errors
and the argument tuple's successful-hash cache. Alias subclasses, including
unchanged Callable aliases, inherit these operations. `__parameters__` discovers
PEP 695 parameters and nested alias/list/tuple parameters in first-occurrence
identity order, ignores bare classes, and honors Python typing metadata
descriptors. Successful discovery caches one tuple; errors retry. Sequence
traversal uses a typed worklist with a local recursion guard. Subscription
substitutes ordinary TypeVars, lazily evaluates omitted defaults, and recursively
reconstructs sequence and nested alias arguments. Custom typing preparation,
substitution, and nested item callbacks run through the VM. Unchanged Callable
rewraps specialized arguments in its own subclass. None becomes NoneType;
string forward references and variadic parameter substitution remain explicitly
unsupported. Base rewriting remains a later slice.

## Exceptions

Python exceptions retain constructor objects in a stable `args` tuple, including
empty and multiple arguments. Ordinary exception text observes subsequent
mutations of retained native-container arguments. `SystemExit` retains its `code` and derives from
`BaseException`, not `Exception`. The OSError family exposes `errno`, `strerror`,
`filename`, and `filename2`; filename-bearing constructors keep the first two
arguments in `args`. Exact `OSError` construction selects a subclass using a
fixed POSIX/Linux errno vocabulary; explicit subclasses retain their type.
`BlockingIOError` supports an integer `characters_written` third argument.
`IOError` and `EnvironmentError` are aliases of `OSError`. General exception
attribute mutation and user argument string-method dispatch remain unsupported.

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
backtrace. `BaseException.with_traceback(None)` clears retained entries and
returns the same exception; non-`None` values are rejected. Python
`__traceback__` objects and broad introspection are not implemented. The initial
Python frame and writable-locals subset is described above.

## Modules and imports

A module has one string-keyed namespace used as both locals and globals.
`Runtime.ExecuteModule` executes bare code as an ordinary module.
`Runtime.ExecuteModuleSpec` preserves a loader's package, origin, and search
metadata for a file or package entry. Both create and cache the module before
its body starts. `Runtime.Module`, `Module.Get`, module attribute access, and
imports all observe the same object and namespace, including names assigned
during partial initialization.

`__dict__` exposes the module's actual writable dictionary. Attribute writes,
source-level global stores/deletes, imports, dictionary methods, and name lookup
stay synchronized. Python stores retain dictionary insertion order; initial
Go-supplied namespace entries are sorted when their dictionary is first exposed.
Dictionary copies are independent. The `__dict__` attribute itself is read-only.

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
many components raises the beyond-top-level form. Each runtime starts with a
cached `builtins` module backed by the same namespace used for bare builtin
lookup, so module mutation and name resolution remain in sync. It also caches
`__future__`, whose marker values cover the feature names the resolver accepts.
A cached `_functools` module exports `cmp_to_key`; its other CPython accelerator
exports remain absent so pure-Python fallbacks can handle them. A cached `string`
package and `string.templatelib` child expose the native template constructors
and conversion helper. These modules retain ordinary import and binding behavior
without a filesystem loader.

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
`stdlib/3.14`. Unchanged `operator`, `keyword`, and `heapq` now run selected regression tests
for calls, classification, and heap operations. The synchronous unittest sources
and the initial io/abc dependency files are vendored for offline import probes;
unittest now reaches missing `traceback` after unchanged io and _collections_abc
import successfully,
while abc imports and has a project-owned source regression test. The full
transitive dependency closure is not present. The original first executable
module remains unchanged `colorsys.py`. Its adapted test
module executes all eight upstream public test methods through the filesystem
loader and complete interpreter pipeline. The
[unittest compatibility roadmap](unittest.md) records the remaining work needed
to replace those assertions with the unchanged synchronous CPython test
framework.

## Internal host configuration

`runtime.NewWithConfig` copies UTF-8 arguments and retains the supplied source
loader. Empty arguments become `[""]`. Invalid argument encoding returns a Go
construction error. There is no ambient host access. The private per-runtime
constructor registry sits between the cache and source loader. Constructors
populate a cached module without dummy code; failure removes that module while
completed dependencies remain cached. Duplicate registrations are rejected.

`sys` provides `argv`, `exit`, and ordinary/original standard-stream attributes.
Unconfigured streams initialize to `None`. Configured streams receive fresh
`bullsnake.HostTextStream` wrappers; original attributes retain their identities
after ordinary attributes are replaced. `sys.exit` normalizes None and tuple
statuses as CPython does, preserves a supplied SystemExit instance, and raises
through the VM without exiting Go.

Host streams borrow `io.Reader` or `io.Writer`. They use strict UTF-8, count
characters rather than bytes, preserve newlines, and delimit `readline` on LF
(the equivalent of fixed `newline="\n"`). They expose `read`, `readline`,
`write`, `flush`, `close`, context management, stream status queries, and fixed
`encoding`, `errors`, and `closed` attributes. Size arguments currently accept
native integers/booleans; `read` also accepts None. Custom `__index__` size
conversion, iteration, `writelines`, and broader TextIOBase behavior remain later
work. These wrappers do not yet inherit the not-yet-implemented io ABCs.

Only output `Flusher` and either direction's `Terminal` are recognized as
optional interfaces. No output buffer is added. Missing Flush means no work;
missing IsTerminal means false. Seeking, telling, truncating, descriptors, and
the wrong read/write direction raise the internal `io.UnsupportedOperation`
class, which matches OSError and ValueError and is exported by `_io`.
The vendored `io` module now imports successfully. Its project-owned regression
test exercises public stream ABC registration, Reader/Writer structural checks,
abstract construction and aliases, a RawIOBase subclass, and text/binary stream
round trips through the unchanged public module.
Closing flushes output and closes the wrapper even if flushing raises. It never
calls a borrowed provider's Close, and repeated close does nothing. Other I/O
and status operations on a closed wrapper raise ValueError.

Input buffering retains bytes returned alongside an error or EOF. A provider
failure after decoded text is delivered on the next nonzero read. EOF ends the
current read without raising. Short writes, invalid provider counts, and write
errors cannot report full success; BlockingIOError retains the count of complete
characters accepted. Wrapped provider errors map to the fixed errno vocabulary;
provider-only PathError paths are omitted. UnicodeEncodeError and
UnicodeDecodeError preserve codec arguments and offsets, including user subclasses.
Bare raises of those classes enforce their required constructor arguments. Strict decoding errors
identify the first invalid byte consumed by the incremental reader; they do not
claim CPython's internal buffer size or offsets within its buffer.

`time.perf_counter` converts a caller-supplied `PerfCounter` duration to seconds.
Missing counters raise PermissionError; typed nil providers fail construction.
The provider promises nondecreasing values from a fixed arbitrary origin. There
is no clock fallback, wall time, sleeping, or scheduling. Provider calls run
synchronously and may block indefinitely. No cancellation guarantee is made.
`sys.modules`, active exception helpers, and Python traceback objects remain
unimplemented. Unchanged `io.py` now imports through the supplied `_io` classes.

The private `_io` constructor supplies the synchronous in-memory stream classes,
newline decoder, shared exceptions, and permission-denied filesystem entry
points. See the I/O sections below for tested behavior and compatibility limits.
No `_io` operation acquires an ambient host capability.

## Deliberate boundaries

The largest current gaps are:

- no public Go embedding or extension API
- no namespace packages, broad standard library, or native extension loading
- no automatic generator closing during Go garbage collection
- no custom awaitable protocol, `aiter` or `anext` builtins, async scheduling,
  automatic async-generator finalization, or Python threads
- no complete Python object protocol or custom attribute interception; collection
  hashing and equality cannot yet invoke arbitrary user methods
- no Python traceback objects, broad frame inspection, tracing, profiling, debugger hooks, or
  execution budgets
- no REPL or eval-specific entry point

Earlier stages may already understand syntax associated with these gaps. The
later-stage rejection is intentional and remains until that stage has behavior
tests and an implementation.

## Memory management

Go's garbage collector owns runtime memory. Frames, stacks, namespaces, cells,
and container entries keep Python references in typed Go pointers and
interfaces so the collector can trace cycles.

User classes keep immediate subclass links as Go weak pointers to the actual
class allocations. `C.__subclasses__()` and `type.__subclasses__(C)` return a
fresh strong list in definition order for user classes, pruning dead links on
access. Native-base subclass enumeration is not implemented yet. Tests verify
that live parents do not retain dead subclass cycles and that instances and
promoted references keep their classes alive.

Bullsnake does not implement CPython reference counting, `__del__`, general Python
weakref construction/callbacks, or a compatible `gc` module. The callback-free
class references returned by ABC diagnostics are the only Python-visible weak
references currently supported. The experiments under
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

### Printing to Python streams

`print` accepts arbitrary positional values and keyword-only `sep`, `end`,
`file`, and `flush`. It converts flush truth first, resolves the current
`sys.stdout` when file is omitted or None, and retains that stream for the call.
Each write method is resolved before converting its value through Python str.
Separators, values, and the ending are written separately; earlier output is
preserved if conversion, writing, or flushing fails. Return values from stream
methods are ignored. Python callbacks use VM continuations, and host wrappers
retain their existing Unicode, error, and borrowed-ownership contracts.

Under Bullsnake's explicit host policy, a None stdout raises PermissionError
instead of CPython's disconnected-stdout no-op. There is no ambient output or
silent sink. A deleted sys.stdout raises RuntimeError. Explicit Python streams
work without host output providers. print currently inherits the existing str
limitations, including representations of containers holding user objects.


### Synchronous in-memory I/O

The native `_io` classes use per-runtime ordinary class allocations and private
Go instance storage. Native methods bind through normal attribute lookup,
`super`, C3 inheritance, and ABC checks. Supplied classes are immutable; Python
subclasses retain their own initializers, methods, properties, and attributes.
Instance `__dict__` and general custom instance `__new__` remain deferred;
class namespace proxies are available.

| Surface | Tested behavior |
| --- | --- |
| `_IOBase`, `_TextIOBase` | Closed state, flush/close, context management, status and unsupported operations, and Python overrides. Base close marks the object closed even when flush raises. |
| Base line helpers | Binary readline with optional peek; iteration, readlines hints, and incremental writelines through Python callbacks; partial failures and EINTR. |
| `_RawIOBase` | read/readall delegation to writable readinto buffers, short counts, None before or after progress, and invalid counts. Abstract readinto/write retain CPython's abstract defaults. |
| `_BufferedIOBase` | readinto/readinto1 delegation through Python read/read1, writable-buffer validation, result bounds, and scoped leases. |
| `StringIO` | Character-based reads and writes, all newline modes, line iteration, seeks, tell, truncation, snapshots, NUL-filled gaps, reinitialization, close, and subclass overrides. Lone surrogate code points remain valid text. |
| `BytesIO` | Byte-based reads/writes, relative seeks, truncation, snapshots, readinto, getbuffer, native line behavior, reinitialization, and exported-buffer restrictions. Native iteration and writelines bypass subclass readline/write overrides. |
| `IncrementalNewlineDecoder` | Python decoder delegation or direct text, pending CR, split CRLF, final flushing, newline history, reset, and packed getstate/setstate flags. |
| `BufferedReader` | Read-ahead, read/read1/readinto/readinto1, peek, lines, logical tell, seek, detach, close, and raw metadata. Unbounded reads prefer raw readall and otherwise use read. |
| `BufferedWriter` | Small-write buffering, direct large writes, short writes, nonblocking capacity, exact characters_written retry counts, flush, seek, truncate, detach, and close after flush failure. |
| `BufferedRandom` | Alternating reads and writes over a seekable raw object, logical cursor synchronization, explicit flush/rewind, and recovery after failed seeks. |
| `BufferedRWPair` | Independent reader/writer composition, constructor conversion order, terminal status, and closing both sides after failure. |
| `TextIOWrapper` | Encoded output, incremental input, character limits, lines, newline translation/history, buffering controls, positions, truncation, reconfigure, detach, and close. |

Immediate native callback steps use trampolines; Python callbacks suspend in
heap VM frames. Input size does not grow the Go stack. Raw buffer counts accept
Python's index protocol. Reentrant buffered operations raise RuntimeError, and
frame-owned guards release on Python exceptions and Go failures. Explicit close
and context management govern stream lifetime; Go GC never invokes Python I/O
methods or closes a supplied provider.

Integrated source fixtures run print through TextIOWrapper and BufferedRandom
to BytesIO, then decode the output, restore positions, and close the entire
stack. Separate fixtures test native/subclass behavior, partial and nonblocking
operations, EINTR without duplicated output, Unicode counts, close failures,
and ownership. These are source-to-result tests, not import or fixture counts.

### Binary buffers and ownership

`bytes` and `bytearray` constructors accept no argument, an integer zero-fill
size, or a supported binary buffer. Bytearrays provide length, truth, iteration,
content comparison, integer/slice reads, and contiguous or extended mutation.
Binary concatenation preserves immutable snapshots; bytearray += preserves
identity and rejects resizing with active exports, including self-concatenation.
Iterable and encoded-string construction and the full bytearray method set are
not implemented.

`memoryview` supports one-dimensional unsigned-byte views of bytes, bytearray,
and other views, including strided slicing, same-shape mutation, readonly views,
metadata, iteration, conversion, content comparison, hashing where valid, and
explicit release. Released operations raise ValueError; release is idempotent
and an already cached hash survives release. Multidimensional formats, casts,
and arbitrary Python buffer exporters remain unsupported.

`bytes`, `bytearray`, and `memoryview` expose real `__buffer__` descriptors.
Bytearray and memoryview also expose `__release_buffer__`. Flags use the index
protocol and enforce writable and contiguous requests for the implemented byte
layout. Explicit memoryview exports retain their source view as `obj`, unlike
ordinary view copies, and prevent releasing that source until every live child
export releases. Weak export records are pruned on access, so unreachable exports
do not pin the source and no Python runs during Go GC. Release checks ownership.
These descriptors enable unchanged Buffer ABC structural checks. General Python
exporter dispatch through memoryview() remains unimplemented.

A view retains its exporter strongly. Export tables hold Go weak pointers to
actual Python view allocations and prune dead or released entries on the VM
goroutine. Child views retain independent exports. Resizing bytearray is denied
while exported; BytesIO also denies writes, truncation, and close. Go GC tests
verify non-retention by export tables and retention by live views. No finalizer
or weak-reference callback calls Python.

Operations borrowing writable storage across callbacks register frame leases.
Leases prevent exporter resizing and source-view release until completion.
Success and Python exceptions release directly; Go-error unwinding releases
remaining leases and guards. Source-loader failure/recovery tests exercise
this contract without depending on GC.

### Text codecs and positions

TextIOWrapper defaults to deterministic UTF-8 and also supports explicit ASCII
and Latin-1, with strict, ignore, and replace error handling. Invalid input or
unencodable text raises structured UnicodeDecodeError or UnicodeEncodeError.
Malformed UTF-8 consumes the valid prefix before an invalid continuation as one
error span or replacement character. Restricted second bytes reject only the
lead byte, preserving Python's overlong, surrogate, and range-error boundaries.
Split-prefix fixtures also check newline handling and position restoration.
Failed decoding discards the new chunk without publishing partial characters or
newline history. Previously buffered incomplete bytes and pending CR survive;
unbounded read failures also preserve decoded read-ahead from earlier calls.
Unknown codecs and unavailable error handlers raise LookupError. Requesting
`locale` raises PermissionError; codec selection never reads ambient locale,
environment variables, files, or a process-wide registry.

Incremental input retains incomplete UTF-8 sequences and CRLF across binary
reads. Each decoded character records its source-byte width. The implemented
stateless codecs use byte offsets as restore positions; CPython's opaque cookie
encoding is not reproduced. Tests verify seeking back across multibyte text and
CRLF cuts and reading the same result. Iteration disables tell until flush or
exhaustion. Inherited TextIOWrapper iteration calls a subclass's readline through
VM continuations and validates text results. Exact native instances bypass an
instance-level readline replacement. Nonzero relative/end text seeks are unsupported.

Reconfigure flushes old encoded output before applying keyword-only options.
Codec/newline changes are rejected while the decoded buffer remains installed;
exhausting a bounded read alone does not clear that state. Successful read-all
with a saved read snapshot and line reads reaching EOF clear the guard. A failed
first decode does not install a buffer or block reconfiguration. Buffering flags
can change independently. Text output is cleared before its binary write,
matching CPython when an exception leaves the partial write count unknown.

Stateful codecs, codec registration, pickle state methods, and comprehensive
CPython I/O introspection are outside this implemented subset.

### Filesystem denial boundaries

`_io.open` and builtins.open are the same immutable builtin function. It and
FileIO validate Python modes, text/binary options, buffering, paths, descriptor
indices, and closefd, then raise PermissionError. Python __fspath__/__index__
conversion may run; a custom opener is never invoked. open_code requires text
and also denies access. A FileIO subclass that skips initialization stays closed.
No request uses the source loader as a filesystem or acquires a process file
descriptor. Filesystem providers and Python-opened owned handles remain future
work.

text_encoding returns UTF-8 for None, preserves explicitly supplied objects,
and validates stacklevel through the index protocol. UnsupportedOperation is
the shared exception matching both OSError and ValueError.
