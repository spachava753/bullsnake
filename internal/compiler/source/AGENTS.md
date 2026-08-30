# Source loader notes

Before changing the source loader, read the `Source loading` and `Testing`
sections in `docs/impl.md`.

## Package contract

This package turns one complete source byte stream into one decoded source unit.
It owns file and reader input, the UTF-8 byte-order mark, Python coding comments,
codec lookup, strict decoding, and source-loading errors.

The returned text is UTF-8. Preserve the original coding comment and physical
line endings after decoding. Offsets used by later stages count bytes in this
decoded text.

The source loader does not tokenize Python and does not decide whether decoded
characters form valid Python syntax. The lexer owns malformed UTF-8 text passed
directly to it, null bytes, tokens, indentation, and lexical errors.

Keep file input out of the lexer. Keep byte-level encoding work out of the
parser and compiler. Do not add a codec merely because Go can decode it; source
codecs must follow the compatibility limits documented in `docs/impl.md`.

## Testing

Use direct byte cases for coding comments, aliases, byte-order marks, malformed
input, and conflicting declarations. Test `Read` and `ReadFile` separately for
input failures.

Use a loader-to-lexer test only when decoded byte positions or byte-order-mark
removal are part of the contract between the two packages. Lexer grammar does
not belong in source-loader tests.

Check Python behavior against the CPython 3.14.7 files named in `docs/impl.md`.
Run the full repository checks after changing supported codecs or source spans.
