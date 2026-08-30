# Parser notes

Before changing the parser, read:

- The front-end section in `docs/architecture.md` for the parser's place in the
  compiler.
- The `Parser and resolver` and `Testing` sections in `docs/impl.md` for current
  behavior and test coverage.
- `internal/compiler/ast/ast.go` for the current AST.
- `testdata/parser_cases.json` and `corpus_test.go` for the pinned Python
  reference and checked-in cases.

## Package contract

The parser consumes lexer tokens and builds the internal AST with source spans.
It owns Python grammar, keyword meaning, assignment target shape, and whether
missing input is syntactically incomplete.

The parser accepts every syntax form represented by the current AST, including
forms that the compiler or runtime does not support yet. Parsing success promises
only a valid AST. Do not reject syntax merely because a later stage cannot
execute it.

The parser does not classify local, global, cell, or free names. It does not
check whether `return`, `yield`, `await`, loop control, or declarations are valid
in a surrounding scope. The resolver owns those checks.

Preserve source facts that later stages cannot recover, such as raw string mode,
formatted-string debug text, explicit empty format specifications, and exact
spans. Do not make the compiler reconstruct text that the parser discarded.

## Testing

When adding a case, use the exact CPython commit recorded in
`parser_cases.json`. Compare the complete normalized AST or the owned parser
error. Add focused tests for token cursor behavior, spans, error formatting, and
source incompleteness.

Update the parser case count and supported syntax in `docs/impl.md` with the
finished change. Do not add tests that only count cases or inspect fixture
categories.
