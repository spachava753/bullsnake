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
| Source loading | Not implemented; lexer APIs require decoded UTF-8 |
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

No source loader exists yet. `New` and `NewFile` accept one immutable, decoded
UTF-8 string. They remove an initial UTF-8 BOM from the token stream, reject
malformed UTF-8 and null bytes, and otherwise preserve the input bytes.

PEP 263 coding-cookie detection, source-encoding conversion, and file or reader
I/O are not implemented. They belong before the lexer because it operates only
on decoded source. Keeping the complete decoded source allows `Token.Text` to
be a zero-copy slice and keeps eventual I/O errors separate from lexical
errors.

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

Not implemented. This section will record grammar, lookahead, syntax-tree,
name-resolution, scope, and closure decisions as those stages are built.

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

Future baseline changes must update the conformance tables, pinned revision,
case counts, and affected focused tests in the same review.
