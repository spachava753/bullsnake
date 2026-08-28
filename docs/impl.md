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
| Parser and resolver | Not implemented |
| Compiler and bytecode | Not implemented |
| Virtual machine and frames | Not implemented |
| Object model and runtime | Not implemented |
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

The initial parser grammar is implemented. Name resolution remains a separate
future phase.

`internal/compiler/ast` defines the internal AST. The initial node set covers
the module root; expression, chained, annotated, and augmented assignments;
`pass`, `return`, `raise`, `del`, `assert`, loop-control, scope-declaration,
import, and conditional statements; names; number, plain string, boolean,
`None`, and ellipsis literals; unary, binary, boolean, comparison, conditional,
named-assignment, await, yield,
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
conditional and assignment expressions, `await`, `yield`, tuples,
parenthesized expressions, collection displays, comprehensions, generator
expressions, unpacking, and complete ordinary call arguments. A
primary-expression loop supports chained calls, attributes, subscripts, and
slices; attributes, lists, and subscripts can also be assignment targets.
Simple-statement lines support semicolon separators, all assignment forms,
`return`, `raise`, `del`, `assert`, `pass`, `break`, `continue`, `global`,
`nonlocal`, and imports. `if`/`elif`/`else` supports a same-line simple-statement
list or an indented statement list; `elif` clauses become nested `IfStmt`
alternatives. Loops, lambdas, adjacent string concatenation, and formatted
strings remain future grammar stages.

The resolver will handle bindings, scopes, and contextual placement rules.
Parser tests identify whether an invalid program belongs to the lexer or parser
so that later phases do not accidentally absorb grammar errors.

## Compiler and bytecode

Not implemented. This section will record instruction encoding, constants,
exception regions, cache compatibility, and compiler invariants.

## Virtual machine and frames

Not implemented. This section will record dispatch, operand stacks, calls,
suspension, frames, tracebacks, and introspection behavior.

## Object model and runtime

Not implemented. This section will record object representation, type slots,
attribute access, descriptors, exceptions, builtins, and runtime isolation.

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

The interpreter runtime is not implemented. Go GC behavior relevant to object
identity, finalizers, weak pointers, cleanup, and cycles is explored in
`experiments/gcprobe`; runtime choices remain documented in the architecture
until production code adopts them.

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
The initial corpus contains fifty-nine successful AST cases and twenty-six
failures.
Successful cases record source and a normalized module dump. Failures record
the owning compiler phase, exception family, message fragment, completeness,
and optional span. The corpus test requires at least one successful and one
failing case. Individual fixture fields are checked by the parser assertions
that consume them rather than a separate schema validator.

The single corpus test runs every successful and failing case. Focused tests
cover successful AST spans, source validation, error formatting, token-cursor
laziness and rewinds, terminal-error caching, and parser fuzz seeds.

Future baseline changes must update the conformance tables, pinned revision,
case counts, and affected focused tests in the same review.
