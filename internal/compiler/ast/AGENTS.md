# AST package notes

Before changing the AST, read the `Parser and resolver` section in `docs/impl.md`
and the front-end section in `docs/architecture.md`.

## Package contract

The AST is Bullsnake's internal description of parsed Python source. It connects
the parser, resolver, and compiler. It is not a public extension API. Change its
shape when a compiler stage needs a clearer or more complete description; there
is no need to preserve an old internal shape.

Nodes describe source structure and carry lexer spans. Keep parser facts that
cannot be recovered later, but do not attach resolved names, bytecode positions,
or runtime values to nodes. The resolver returns a separate table, and the
compiler returns separate code objects.

Use named enums for Python operations and contexts. Their numeric values are not
bytecode operands. The compiler must map them explicitly.

When a later stage needs information the AST lacks, change the AST and parser in
a separate parser commit with a parser behavior test. Do not guess the missing
fact in the resolver or compiler.

## Testing

`Dump` is the stable test description of the current AST. Keep its output
complete, deterministic, and readable. Update parser cases whenever an AST
shape changes.

Use focused tests for dump options and enum formatting. Resolver and compiler
tests should consume the new AST through parsed source rather than constructing
large trees by hand.
