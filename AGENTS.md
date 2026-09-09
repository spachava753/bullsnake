# Bullsnake project notes

Bullsnake is a Python interpreter written in Go. It supports a useful subset of
Python and aims to run more pure Python packages over time.

Before making changes, use these files as the sources of truth:

- `README.md` gives the short project overview.
- `docs/architecture.md` records project goals, design choices, and compatibility
  boundaries.
- `docs/impl.md` describes what the code does now and what is still missing.
- `docs/unittest.md` tracks the first standard-library milestone and the proposed
  Go-backed module design using caller-supplied, composable capability interfaces.
  Its first host configuration covers arguments, standard streams, and a
  performance counter; Python filesystem access comes later.
- The code and tests define the current behavior. If they disagree with the
  documentation, resolve the difference as part of the change.

Read any more specific `AGENTS.md` file in the area being changed. Those files
contain the contract and test rules for that package.

## Development model

Bullsnake is under active development. It has no tagged compatibility checkpoint
and no public promise that every parsed or compiled program executes.

The source loader, lexer, parser, resolver, compiler, and runtime are separate
stages. Each stage may support more input than the stage after it. Success in an
earlier stage means only that the earlier stage handled its part.

Keep rejection checks in later stages until that stage implements the feature.
Do not allow an instruction, syntax form, or value merely because an earlier
stage can produce it. Code that cannot execute yet is an expected part of the
work in progress.

A vertical slice starts with source, passes through the stages involved in the
change, and ends in an observable result or error. Build one small slice at a
time. Add or select its behavior tests first, implement it, update the current
implementation notes, run all checks, and commit the finished slice.

When work in the active stage exposes a missing fact in an earlier stage, make
the earlier-stage change as a separate tested commit. Preserve that package's
branch rules when its local `AGENTS.md` names one.

Test program behavior and package contracts. Do not add tests whose only purpose
is counting fixture entries, checking fixture names, or requiring category
coverage.

## Verification

Run these checks before each finished commit:

```sh
go test ./...
go vet ./...
go tool laas -funcdoc.limit=5 -exclude-packages='(^|/)experiments($|/)' ./...
git diff --check
```
