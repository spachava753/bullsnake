# Lexer notes

Before changing the lexer, read the `Lexer` and `Testing` sections in
`docs/impl.md` and the front-end section in `docs/architecture.md`.

## Package contract

The lexer accepts one complete decoded UTF-8 source string. It performs no file
input and no source-encoding detection. The source loader owns those jobs.

`Next` is a pull interface. Tokens keep their exact source text and half-open
byte spans. Lines start at one. Columns and offsets count UTF-8 bytes in the
decoded source.

The lexer owns indentation, physical and logical newlines, delimiters, number
and string boundaries, formatted-string modes, identifier validity, and lexical
incompleteness. Preserve comments and non-significant newlines in the token
stream.

Keywords remain name tokens. The parser owns keyword meaning and soft keywords.
The compiler owns number conversion and string escape evaluation. The lexer
must not decide whether a valid token sequence is valid in its surrounding
scope or executable by the runtime.

A lexical `Incomplete` result reports only unfinished lexical input. It does not
answer whether a complete token stream forms a complete statement.

## Testing

Use the checked-in CPython 3.14.7 cases and the pinned reference named in
`docs/impl.md`. Add focused tests for spans, limits, malformed tokens, and lexer
state. Add fuzz seeds for failures that could expose cursor or mode errors.

Keep lexer tests independent of parser, compiler, and runtime support. A token
is valid when the lexer contract accepts it, even when a later stage rejects the
program.
