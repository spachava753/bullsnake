# Bullsnake in Go: implementation

This document is a work in progress. It records implementation choices in the
Go interpreter as they are made. The [runtime design](architecture.md) describes
goals, compatibility boundaries, and proposed architecture; this document
describes the code that exists.

- [Pipeline status](#pipeline-status)
- [Source loading](#source-loading)
- [Lexer](#lexer)
- [Parser and resolver](#parser-and-resolver)
- [Compiler and bytecode](#compiler-and-bytecode)
- [Virtual machine and frames](#virtual-machine-and-frames)
- [Object model and runtime](#object-model-and-runtime)
- [Import system](#import-system)
- [Go embedding](#go-embedding)
- [Async and scheduling](#async-and-scheduling)
- [Memory management](#memory-management)
- [REPL](#repl)
- [Testing](#testing)

## Pipeline status

| Stage | Status |
| --- | --- |
| Source loading | Initial PEP 263 behavior implemented for supported codecs |
| Lexer | Initial Python 3.14 behavior implemented |
| Parser and resolver | Initial Python 3.14 parser and name resolution implemented |
| Compiler and bytecode | Initial Python 3.14-derived bytecode subset implemented |
| Virtual machine and frames | Initial module execution slice implemented |
| Object model and runtime | Initial scalar values and module runtime implemented |
| Import system and standard library | Not implemented |
| Go embedding API | Not implemented |
| Async and scheduling | Not implemented |
| REPL | Not implemented |

## Source loading

`internal/compiler/source` loads one complete source unit before lexing.
`Decode` accepts bytes, `Read` consumes an `io.Reader`, and `ReadFile` reads a
path. They return a `Unit` containing the filename, detected encoding, and one
immutable UTF-8 string. Reading the complete unit keeps I/O failures separate
from lexical failures and lets `Token.Text` remain a zero-copy slice.

The loader implements PEP 263's first-two-line coding-cookie rules. Its CPython
3.14.7 references are `Parser/tokenizer/helpers.c`,
`Parser/tokenizer/string_tokenizer.c`, and `Lib/test/test_source_encoding.py`.
It defaults to strict UTF-8, removes an initial UTF-8 BOM, rejects a conflicting
cookie, and transcodes supported declared encodings to UTF-8. The coding
comment and physical CR, LF, or CRLF spellings remain in the decoded text.
Token offsets therefore count bytes in the decoded UTF-8 text; a source loaded
through a BOM starts at offset zero.

Bullsnake does not yet have Python's runtime-extensible codec registry. Source
encodings are limited to ASCII-compatible IANA names and aliases implemented by
`golang.org/x/text`, plus common Python aliases for those codecs. This includes
the supported ISO-8859 and Windows code pages, Shift-JIS, EUC-JP, EUC-KR,
ISO-2022-JP, GBK, GB18030, HZ-GB-2312, and Big5. Unknown, unavailable, and
non-ASCII-compatible encodings fail with a structured `source.Error`.
Malformed legacy input also fails when an `x/text` decoder would otherwise
insert a replacement character that does not round-trip to the original bytes.

The source loader owns byte-level BOM handling. `lexer.New` and
`lexer.NewFile` accept decoded UTF-8 text, reject malformed UTF-8 and null
bytes, and do not perform file I/O. A decoded U+FEFF is ordinary source input
and receives the same lexical validation as any other source character.

## Lexer

Status: initial implementation complete.

Reference: CPython 3.14.7, tag `v3.14.7`, commit
`823f0323ee6ec1402088b73bce1a38473cac36dc`.

Bullsnake follows Python 3.14 lexical behavior for supported syntax. The main
references are `Grammar/Tokens`, `Parser/lexer/lexer.c`,
`Parser/lexer/state.c`, and `Lib/test/test_tokenize.py` at the revision above.
CPython `main` was also reviewed because it has split number and string
scanning into separate files. Bullsnake copies the lexical semantics, not
CPython's C storage and buffer-management model.

### Go representation

`internal/compiler/lexer` holds a byte cursor, indentation stack, delimiter
stack, f/t-string mode stack, and queue for pending dedents. These are ordinary
Go values and slices. The Python-facing limits of a 100-entry indentation
stack, 200 open delimiters, and a 150-entry f/t-string mode stack remain part of
the lexical behavior. The regular root mode occupies one f/t-string stack
entry, matching CPython.

`Next` is a pull API so the parser can request lookahead without materializing
the token stream. The lexer precomputes physical line starts so it can convert
arbitrary byte offsets into line and column positions.

Complex scanners are split by phase. Indentation measurement is separate from
indent-stack updates, number scanning separates digit groups from fractions
and exponents, and formatted-string scanning separates text boundaries from
brace transitions. This keeps the state changes local while preserving
CPython's token order.

### Token contract

Tokens retain their exact source spelling and a half-open span. Lines are
one-based. Columns and absolute offsets count UTF-8 bytes, matching Python
compiler and AST column semantics and allowing ordinary spans to slice the Go
source directly.

`NEWLINE` and `NL` end coordinates follow Python's token convention and remain
on the preceding line after the line-ending bytes. The following token starts
on the next line. Python's public `tokenize` module reports Unicode code-point
columns instead; an adapter for that module must convert columns at its
boundary.

The lexer emits exact operator kinds. Keywords remain `NAME` tokens. Literal
values remain as source text, and the lexer does not decode string escapes.
Keyword classification belongs to the parser. Number conversion and string
literal evaluation belong to later compiler stages.

`COMMENT` and non-significant `NL` tokens remain in the stream. A parser can
skip them with `Token.IsTrivia`; retaining them supports diagnostics and future
source tooling without a second lexer.

F/t-string middle tokens follow CPython's parser-facing brace behavior. Doubled
braces leave a gap between spans rather than appearing twice in token text.

### Implemented behavior

The lexer implements:

- Python indentation, form-feed handling, the alternate tab-width stack, and
  ordered `INDENT` and `DEDENT` emission
- Physical `NL` versus logical `NEWLINE`, explicit continuations, implicit
  joining inside delimiters, and synthetic final line endings
- Ordered delimiter matching and Python's nesting limits
- Python 3.14 operators and maximal-munch tokenization
- Decimal, binary, octal, hexadecimal, float, exponent, underscore, imaginary,
  and leading-zero validation
- Ordinary raw, bytes, and Unicode string prefixes and triple-quoted strings
- Python 3.14 f-strings and template strings, including nested replacement
  fields, format specifications, doubled braces, and nested strings
- PEP 3131 identifiers with NFKC checks
- Structured syntax, indentation, tab, and encoding errors with source spans
- A lexical `Incomplete` hint for an open delimiter, triple-quoted string,
  triple-quoted f/t string, or final line continuation

Go 1.27 and `golang.org/x/text` use Unicode 17, while Python 3.14 uses Unicode
16. The lexer removes the small Unicode 17 XID addition delta so it does not
accept identifiers that the reference Python rejects.

### Current boundaries

The scanner uses file-input EOF behavior. A REPL must combine the lexical
`Incomplete` hint with parser completeness. A lexically complete `if` header is
still parser-incomplete, so the lexer cannot decide by itself when to request
another input line.

Type comments remain `COMMENT` tokens. Optional `TYPE_COMMENT` and
`TYPE_IGNORE` classification can be added when the AST adopts type comments.
`SOFT_KEYWORD` classification also belongs to the parser.

Exact CPython warning behavior and error wording are outside Bullsnake's
contract. CPython emits transitional `SyntaxWarning` messages for some
number-and-keyword adjacencies. Bullsnake preserves valid token boundaries and
reports malformed literals directly.

## Parser and resolver

The checked-in parser grammar and resolver are implemented for the Python 3.14
syntax forms emitted by the lexer. The parser constructs the AST; the resolver
classifies names and rejects rules that depend on surrounding scopes.

`internal/compiler/ast` defines the internal AST. The initial node set covers
the module root; expression, chained, annotated, and augmented assignments;
`pass`, `return`, `raise`, `del`, `assert`, loop-control, scope-declaration,
import, conditional, loop, function-definition, class-definition, type-alias,
context-manager, exception-handling, and structural pattern-matching statements;
names; number, plain string,
boolean, `None`, ellipsis, adjacent-string, formatted-string, and template-string
literals; unary, binary, boolean, comparison, conditional,
lambda, named-assignment, await, yield,
attribute, subscript, slice, starred, list, set, dictionary, comprehension, and
generator expressions; positional, starred, named, and dictionary-unpacked
calls; and tuples. Nodes carry lexer byte spans.
`ast.Dump` provides a deterministic structural representation with optional
spans for tests and diagnostics. The node set will grow with the supported
grammar; it is not a stable extension API.

`internal/compiler/parser.Parse` accepts decoded source and a filename. It
creates the lexer and returns an `ast.Module` or a structured compiler error.
All source uses the same statement grammar. A later eval API will require one
`ExprStmt` and preserve its value; a REPL will display expression-statement
values while executing other statements normally. Reaching the end marker
while required syntax is missing marks an error as incomplete for any caller,
so the REPL needs no parser mode.

The parser token cursor requests lexer tokens lazily, removes `COMMENT` and
non-significant `NL` tokens from parser lookahead, and caches every significant
token. Marks are token indexes, so local speculative parses can rewind without
rewinding the lexer. End markers and lexer errors are cached as terminal cursor
items, which makes repeated lookahead deterministic.

The hand-written recursive-descent grammar constructs AST nodes directly.
Ordinary binary operators use precedence climbing. Dedicated rules handle
comparison chains, boolean `and`/`or`/`not`, unary operators, power,
conditional, lambda, and assignment expressions, `await`, `yield`, tuples,
parenthesized expressions, collection displays, comprehensions, generator
expressions, unpacking, and complete ordinary call arguments. A
primary-expression loop supports chained calls, attributes, subscripts, and
slices; attributes, lists, and subscripts can also be assignment targets.
Simple-statement lines support semicolon separators, all assignment forms,
`return`, `raise`, `del`, `assert`, `pass`, `break`, `continue`, `global`,
`nonlocal`, and imports. Context-manager statements support multiple items,
parentheses, assignment targets, and `async with`. Exception handling supports
`except`, `except*`, `else`, and `finally`. Structural matching supports literal,
capture, wildcard, value, OR, AS, sequence, mapping, and class patterns with
optional guards. `if`/`elif`/`else` supports a same-line simple-statement
list or an indented statement list; `elif` clauses become nested `IfStmt`
alternatives. `while` and synchronous or asynchronous `for` loops support
optional `else` suites. Functions and lambdas support positional-only,
ordinary, variadic, keyword-only, and keyword-variadic parameters and defaults.
Functions also support parameter and return annotations. Functions, classes,
and type aliases support generic type parameters; functions and classes support
decorators. Adjacent plain and formatted strings, nested format specifications,
debug fields, and template strings are parsed directly from the lexer's string
tokens. Formatted-string nodes retain raw-prefix mode and the exact debug-field
spelling needed by bytecode generation.

`internal/compiler/resolver.Resolve` accepts a filename and parsed module. Its
first walk follows source order, creates the scope tree, records every use and
binding, applies private-name mangling, reads future imports, and performs
context-sensitive checks. Its second walk classifies each symbol as local,
cell, free, explicit global, or implicit global and propagates closure requests
back through enclosing scopes. The result is a separate `resolver.Table`; the
AST remains unchanged.

The scope tree represents modules, functions and lambdas, classes,
comprehensions, annotations, type-parameter lists, type-variable bounds and
defaults, and type-alias values. `Table.ScopeFor` distinguishes the several
scopes that one AST node can create. Symbols retain source-order flags and
locations, while stable resolver dumps expose the complete tree for tests and
diagnostics.

The resolver implements ordinary and declaration bindings, closure cells,
class-local skipping and `__class__` closure creation, private names,
comprehension iterator scopes and assignment-expression targets, deferred
annotations, PEP 695 generic scopes, pattern capture validation, generator and
coroutine flags, and placement checks for `return`, loop control, `yield`,
`await`, asynchronous statements, `except*`, wildcard imports, and
`__debug__`. Function-local annotation expressions are traversed for syntax
validation but do not record direct name facts or propagate closure requests.
Unknown names remain implicit globals for runtime lookup rather than becoming
compile-time errors.

The resolver does not assign local, cell, or free-variable array positions and
does not choose bytecode instructions. Those remain compiler responsibilities.
Parser tests identify whether invalid syntax belongs to the lexer or parser;
resolver tests start only from ASTs the parser accepts.

## Compiler and bytecode

`internal/compiler.Compile` accepts a parsed module and its resolver table and
produces an `internal/compiler/bytecode.Code`. Code objects copy their
instruction, position, constant, and name tables at construction and expose
copies through accessors. Each instruction has an explicit opcode and operand;
a parallel table retains its lexer span. The compiler tracks operand-stack
depth while emitting and records the maximum on the code object. Control-flow
instructions use absolute instruction indexes. Labels patch forward jumps and
require every incoming edge to have the same stack depth.

Bytecode version 1 implements the initial file-input module slices: empty
modules, `pass`, singleton, numeric, string, bytes, and formatted-string
constants; module name loads and stores; simple, chained, destructuring,
augmented, module-deferred, and function-local annotated assignments;
recursive deletion targets; expression statements;
collection displays; unary, binary, boolean, comparison, conditional, named,
lambda, attribute, subscription, slice, and call expressions; ordinary,
relative, aliased, and wildcard imports; assertions; bare and explicit raises;
synchronous function definitions with decorators, required and defaulted
parameters, lazy parameter and return annotations, closures, and returns;
class definitions with decorators, ordinary
and starred bases, class keywords, methods, zero-argument `super()`, and
enclosing closure reads;
`if`/`elif`/`else`
statements;
`while` loops; and synchronous `for` loops with name, tuple, or list targets,
including one starred target per sequence. Both loop forms support optional
`else`, `break`, and `continue`.
Reachable code-object fallthrough ends with a synthetic `None` return.
Integer literals are canonicalized at arbitrary precision; float and imaginary
literals are converted to binary64. The compiler decodes Python string and
bytes escapes, normalizes physical newlines in literal values, folds adjacent
plain literals, and retains lone Unicode surrogates as WTF-8-compatible bytes.
Formatted strings preserve raw mode and debug-field spelling, apply `str`,
`repr`, or `ascii` conversions, recursively build format specs, and join plain
and formatted components with explicit stack effects. Tuple, list, set, and
dictionary displays use count-based build instructions when they have no
unpacking. Starred displays use typed append, extend, and update instructions
against one accumulator while evaluating elements from left to right. Unary,
binary, and in-place operations use the same explicit operand IDs. Augmented
attribute and subscript assignments retain their evaluated address beneath the
current value, then rotate the result into the ordinary store order without
reevaluating the object or index. Boolean operators retain the selected operand
across short-circuit jumps. Comparison chains evaluate each operand once,
retain only the next left operand, and clean it up on a false edge. Conditional
expressions merge their two value-producing branches at one checked stack
depth. Attribute loads share the deterministic name table with ordinary names.
Subscriptions evaluate the container before the index; slices represent omitted
bounds with `None` and use one build instruction for two or three components.
Calls without unpacking or keywords use an inline argument count. Other calls
build a positional tuple and optional keyword map; keyword mappings merge in source
order and reject duplicate names rather than applying dictionary-update
semantics. Imports carry an explicit relative level and from-name tuple.
Dotted `as` imports traverse components while removing intermediate modules;
imported values use ordinary resolver-selected stores. Wildcard imports consume
the module after updating the current namespace.
Named assignment expressions copy their value before storing the
target, so the same value remains as the expression result. Attribute and
subscript stores evaluate their object and index after the right-hand value.
Function-local annotated assignments never execute their annotation expression.
Simple module annotations record their source-order index when execution reaches
them; a module-end `__annotate__` child checks those indexes before adding lazy
values to its result map. An optional value uses the ordinary store path. An
annotation-only attribute or subscript evaluates and discards its address
components without reading or writing the target. Fixed tuple and list targets
unpack once, then consume their targets from left to right.
Starred targets use CPython's packed `UNPACK_EX`. Its low byte records up to 255
targets before the star, and its upper 24 bits record targets after it.
Delete statements recursively visit grouped targets without building or
unpacking a runtime collection. Assertions evaluate their message only on the
failing edge, construct `AssertionError`, and terminate that edge
with the same zero-, one-, or two-argument raise instruction used by `raise`.
A fully terminating code object has no synthetic return. Synchronous function
definitions store immutable child code objects by index. Child metadata records
required parameter counts and variadic flags; resolver-local names use indexed
fast operations, while explicit and implicit globals use the name table.
Non-capturing nested functions receive Python-style qualified names. Lambdas
use `<lambda>` names, share the function creation path, and compile their body
expression directly to `RETURN_VALUE`.
Function scopes index cells before free variables in one dereference table,
following resolver order. Captured parameters remain in both the argument-local and cell
tables so frame setup can seed their cells. Each child requests cell objects in
its own free-variable order, which keeps transitive captures correct when parent
and child indexes differ. The parent evaluates positional defaults into one
tuple and sparse keyword-only defaults into one map before loading the closure
tuple and creating the function. Attribute instructions attach the closure,
keyword-only map, and positional tuple in reverse stack order while retaining
the function. Decorator expressions evaluate in source order before defaults.
Calls apply them in reverse order after function creation and attribute
attachment. Parameter and return annotations compile into a sibling
`__annotate__` code object with one positional-only `format` argument. Its
format guard matches CPython 3.14's VALUE and internal fake-globals boundary,
and the body returns an insertion-ordered map of annotation names to values.
The annotation callable may capture enclosing cells and attaches with function
attribute `0x10` before defaults and decorators are consumed. Class-visible
annotation loads check a captured `__classdict__` before their global or
free-variable fallback. Class definitions evaluate decorators before class
construction.
`LOAD_BUILD_CLASS` calls a namespace body function with the class name and
bases. Ordinary arguments use inline `CALL`; starred bases and keyword maps use
a seeded positional list, `MAP_MERGE`, and `CALL_EX`. The body initializes
`__module__`, `__qualname__`, and `__firstlineno__`, uses namespace name
operations, may capture an enclosing function cell, and gives methods
class-qualified names. When the resolver requests `__classdict__`, the class body
captures its live namespace, publishes the cell as `__classdictcell__`, and
passes it to annotation callables. When the resolver requests `__class__`, the
class body allocates that cell before free variables, passes it to methods,
stores a copy as `__classcell__`, and returns the cell to the class builder.
Conditional statements use the same checked labels as conditional expressions;
every true, false, and `elif` edge merges with an empty operand stack. The
compiler keeps a nearest-loop stack for `break` and `continue`. A `while`
condition's normal false edge enters `else`, while `break` targets the loop end
directly. A `for` loop keeps its iterator beneath the body stack; successful
iteration pushes one item, normal exhaustion removes the iterator and enters
`else`, and `break`
pops to the loop's recorded base depth before skipping `else`. This depth rule
preserves outer iterators in nested loops. A suite stops emitting after an
unconditional jump, and a join with no reachable input remains unreachable.
Constants and referenced names use deterministic indexed tables. Stable code
dumps support compiler tests and future diagnostics.

The instruction representation remains decoded rather than serialized.
Template strings, class annotated assignments, future annotations, generic and
async functions, class docstrings, static-attribute metadata, `async for`,
exception handling, and suspended execution are not yet compiled.
Unsupported AST nodes fail with a source-located compiler error.

## Virtual machine and frames

`internal/runtime` contains the first executable VM slice. `Runtime.ExecuteModule`
accepts an immutable code object, prepares it once per runtime, executes it in a
new module namespace, and caches the module by name only after normal return.
`Runtime.Module` and `Module.Get` expose successful executions to internal
callers and tests. There is not yet a public Go embedding API.

Preparation copies the instruction and name tables, materializes code constants
as runtime values, and validates the complete code object before execution.
Validation currently accepts `NOP`, `LOAD_CONST`, `LOAD_NAME`, `STORE_NAME`,
`POP_TOP`, `COPY`, `SWAP`, scalar `UNARY_OP`, selected integer `BINARY_OP` and
scalar `COMPARE_OP` variants, absolute `JUMP`, both pop-and-test jumps, both
short-circuit-or-pop jumps, and `RETURN_VALUE`. It checks constant and name
indexes, operation operands, jump targets, stack underflow, the declared
maximum stack size, return stack balance, and reachable termination. Any unsupported constant, instruction, or operand
fails with a source-located `BytecodeError` before a module can observe side
effects.

Stack validation uses a worklist over instruction indexes. Each reachable edge
carries its operand-stack depth. Conditional jumps propagate their distinct
fallthrough and taken-edge effects, loops terminate through already-seen
instruction depths, and a merge with different depths is invalid. The
validator allows well-formed unreachable instructions but still checks their
opcodes, operands, and table indexes before execution. These instructions now
execute boolean short-circuit expressions, conditional expressions, `if`
statements, and `while` loops. Loop `else`, `break`, and `continue` require no
separate runtime mechanism; their compiler-selected jump targets preserve the
same frame and operand stack.

A heap-allocated frame contains prepared code, the next instruction index, a
preallocated operand stack, local, global, and builtin namespaces, and its
logical predecessor. A thread state points at the active frame. The dispatch
loop handles explicit advance, return, and raise outcomes. Returning replaces
the active frame with its predecessor and will let a later Python call use the
same iterative loop without Go recursion. Calls, suspension, exception
handlers, traceback chains, cancellation, and execution budgets are not yet
implemented.

## Object model and runtime

The initial sealed `Value` interface keeps every Python reference in a typed Go
interface or pointer. Process-wide immutable singletons represent `None`,
`False`, `True`, and `Ellipsis`. Heap-backed objects represent arbitrary-
precision integers, binary64 floats, complex values, strings, bytes, and
exceptions. Code preparation materializes each constant once per runtime and
code object. String objects accept UTF-8 plus the compiler's deliberate WTF-8
encoding for lone surrogates; bytes objects retain arbitrary payloads. Stable
representations escape non-printable text and bytes without losing their
contents.

Scalar truth testing follows Python for the current fixed types: `None`, false
booleans, numeric zero, and empty strings or bytes are false; other scalar
values are true. Unary `not` returns a boolean singleton. Unary plus and minus
support integers, booleans, floats, and complex values; invert supports
integers and booleans. Booleans produce ordinary integer results for numeric
unary and binary operations. Binary `+`, `-`, `*`, `|`, `^`, `&`, `//`, `%`,
`<<`, and `>>` currently accept integers and booleans and produce arbitrary-
precision integer results. Floor division rounds toward negative infinity, and
modulo produces a remainder with the divisor's sign. Shifts reject negative
counts; huge right shifts collapse by the left operand's sign, while a huge
left shift of a nonzero value raises `OverflowError`. A zero divisor raises
`ZeroDivisionError`.

Equality covers all current scalar values, including boolean/integer,
integer/float, and real-valued complex numeric pairs. Ordering supports
integers, booleans, floats, strings, and bytes; incompatible pairs raise
`TypeError`. Identity compares runtime object identity. Chained comparisons use
validated `COPY`, `SWAP`, and short-circuit jumps so each middle operand is
evaluated once and later operands are skipped after a false result.

Unsupported operand pairings, missing names, and invalid unary types raise
Python `TypeError` or `NameError` values.
`UncaughtException` carries the exception across the current Go host boundary.

A runtime owns its prepared-code cache, builtin namespace, and successful
modules. A module owns one string-keyed namespace used as both locals and
globals during module execution. This namespace is intentionally narrower than
a Python dictionary. General hashing, equality, insertion ordering, ordinary
collections, user types, descriptors, attributes, and callable values remain
unimplemented.

## Import system

Not implemented. This section will record module state, finders, loaders,
filesystem and embedded imports, standard-library policy, and import locking.

## Go embedding

Not implemented. This section will record runtime construction, value
conversion, native callables, cancellation, resource limits, and host error
boundaries.

## Async and scheduling

Not implemented. This section will record generators, coroutines, awaitables,
asynchronous generators, and scheduler integration.

## Memory management

The initial runtime relies on Go's collector. Frames, operand stacks,
namespaces, integers, and exceptions retain Python references through typed
pointers and interfaces; the VM does not hide references in integers or unsafe
storage. Behavior relevant to object identity, finalizers, weak pointers,
cleanup, and cycles is explored in `experiments/gcprobe`. Those features remain
design constraints until a supported runtime feature requires production
machinery.

## REPL

Not implemented. The REPL will need a source accumulator plus lexer and parser
completeness checks. An `io.Reader` alone cannot represent the distinction
between a completed input chunk and a prompt for another line.

## Testing

The ordinary test suite runs with:

```sh
go test ./...
```

The lexer has checked-in Go conformance tables adapted from CPython 3.14.7,
focused CPython-derived tests, malformed-input tests, span checks,
Unicode-version checks, and fuzz seeds. The tables require no Python
installation, CPython checkout, external test data, network access, or
generation step. They port every direct `CTokenizeTest.check_tokenize` case;
CPython tests that require parsing or execution remain deferred until those
pipeline stages exist.

The parser follows the same offline model. Its checked-in corpus is pinned to
CPython 3.14.7 at commit `823f0323ee6ec1402088b73bce1a38473cac36dc`.
The initial corpus contains ninety-six successful AST cases and fifty-one
failures.
Successful cases record source and a normalized module dump. Failures record
the owning compiler phase, exception family, message fragment, completeness,
and optional span. The corpus test requires at least one successful and one
failing case. Individual fixture fields are checked by the parser assertions
that consume them rather than a separate schema validator.

The single parser corpus test runs every successful and failing case. Focused
tests cover successful AST spans, formatted-string format presence, source
validation, error formatting, token-cursor laziness and rewinds, terminal-error
caching, and parser fuzz seeds.

The resolver corpus is pinned to the same CPython revision. It contains
thirty-eight successful symbol-table cases and fifty resolver-owned failures.
Successful cases record complete stable scope dumps. Failures record the
exception family, message fragment, and selected exact spans. Focused tests
cover table lookup, private-name rewriting, dump and diagnostic formatting,
and resolver fuzz seeds.

The compiler corpus currently contains eighty-two successful
parse-resolve-compile cases for the initial module instruction set and
expression evaluation. Cases record stable Bullsnake code-object dumps;
focused tests cover instruction source positions, stack effects, code-object
copying, opcode formatting, literal decoding, formatted-string errors, and
compiler input errors.

Runtime tests compile source through the complete front end before executing
it. The initial cases cover module globals, discarded expressions, every
compiler scalar constant, singleton identity, scalar truth testing, numeric
unary operations, selected arbitrary-precision integer binary operations,
boolean short-circuiting, conditional expressions, conditional statements,
chained scalar comparisons, and `while` loops with normal exhaustion, `else`,
`break`, and `continue`.
Floor-division cases pin quotient rounding, remainder signs, and zero-divisor
errors. Shift cases cover signed values, booleans, negative counts, and huge
counts that cannot fit a machine word. Python `NameError`, `TypeError`,
`ValueError`, `OverflowError`, and `ZeroDivisionError` cases cover language
failures. Focused malformed-code cases cover unsupported instructions,
operands, and constant kinds; invalid integer and string descriptors; table and
jump bounds; stack underflow, overflow, and merge mismatches; unreachable
returns; and fallthrough.

Future baseline changes must update the conformance tables, pinned revision,
case counts, and affected focused tests in the same review.
