# Compiler package notes

This file covers the Go files directly in `internal/compiler`. More specific
files in child packages add rules for their part of the compiler.

Before changing this package, read the `Compiler and bytecode` and `Testing`
sections in `docs/impl.md` and the compiler section in `docs/architecture.md`.

## Package contract

`Compile` accepts one parsed module and its resolver table. It returns an
immutable Bullsnake code object or a compiler error with a source span.

The compiler owns instruction choice, constants, name tables, local positions,
cell and free-variable positions, jump targets, and maximum stack depth. The
resolver owns name meaning and surrounding-scope rules. Do not repeat resolver
work in the compiler.

The compiler must not import `internal/runtime` or create runtime values. It
writes private bytecode descriptions that the runtime may turn into values later.

Compiler support and runtime support are separate. The compiler may translate a
syntax form or emit an instruction that the current runtime rejects. A successful
compile does not promise execution. Do not change compiler output merely to get
past a runtime guard for a feature the runtime has not implemented.

Unsupported AST forms must return a source-located compiler error. Do not emit a
plausible approximation for Python behavior that has not been chosen.

Every control-flow join must have one agreed stack depth. Every instruction must
keep its source span. Keep constant and name table order deterministic so code
dumps remain useful.

## Testing

Add source cases to `testdata/compiler_cases.json`. Each case passes through the
parser, resolver, and compiler, then compares the complete code dump. Use
focused Go tests for code positions, bad compiler inputs, and behavior that is
awkward to express in the case file.

Use the CPython revision recorded in the case file when checking Python compiler
behavior. Write tests for the compiler result, even when the runtime cannot yet
execute that result.

Add the behavior case before or with the implementation. Update `docs/impl.md`
when compiler behavior changes. Keep one finished compiler slice in one commit.
