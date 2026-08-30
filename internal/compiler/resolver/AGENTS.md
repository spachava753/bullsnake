# Resolver notes

Before changing the resolver, read the `Parser and resolver` and `Testing`
sections in `docs/impl.md` and the front-end section in `docs/architecture.md`.

## Package contract

`Resolve` accepts a parsed module and returns a separate scope and symbol table.
It does not modify the AST.

The resolver owns name binding, local and global meaning, closure requests,
private-name rewriting, future-import options, scope flags, and checks that need
surrounding source context. It creates the extra scopes required by functions,
classes, comprehensions, annotations, and generic type syntax.

The resolver does not assign array positions, choose instructions, convert
literal values, or decide whether the runtime supports a feature. Those jobs
belong to the compiler and runtime.

Resolving a syntax form promises only that its names and surrounding-scope rules
are understood. Keep resolver support even when the compiler or runtime still
rejects that form.

Symbol and scope order follows source order and appears in stable dumps. Preserve
that order unless Python's rules require another order.

## Testing

Add source cases to `testdata/resolver_cases.json`. Successful cases compare the
complete scope tree. Error cases must parse first and then fail for a rule owned
by the resolver.

Use the exact CPython 3.14.7 revision recorded in the case file. Use focused
tests for table lookup, private-name rewriting, errors, dumps, and fuzz safety.

When a later stage needs new resolver information, add a resolver behavior case
and make the resolver change as its own commit. Do not add fixture-count or
category-coverage tests.
