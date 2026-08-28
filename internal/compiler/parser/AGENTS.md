# Parser notes

Before changing the parser, read:

- The `Front end` section in `docs/architecture.md` for the parser design and
  its place in the compiler.
- The `Parser and resolver` and `Testing` sections in `docs/impl.md` for current
  behavior, supported syntax, and test coverage.
- `internal/compiler/ast/ast.go` for the current AST.
- `internal/compiler/parser/testdata/parser_cases.json` for the pinned CPython
  version, reference commit, and parser cases.
- `internal/compiler/parser/corpus_test.go` for how those cases are checked.

Do all parser work on the long-lived `feat/parser` branch. Commit each finished
stage on that branch instead of creating a branch for each grammar stage.

When adding a corpus case, use the exact CPython commit named in
`parser_cases.json`. Check that commit's grammar or tests rather than the local
CPython `main` branch. Write the expected result using Bullsnake's AST and the
format already used by the corpus. Update the case count and supported syntax
in `docs/impl.md` in the same stage.
