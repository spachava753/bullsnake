# Runtime execution cases for formatting.
# case: plain formatted strings
name = 'Ada'
number = 42
simple = f'hello {name}: {number}'
converted = f'{name!s}|{name!r}|{name!a}'
debug = f'{name = }'
unicode_ascii = f"{'café'!a}"
container = f'{[1, 2]}'
bytes_value = f"{b'xy'}"
assert f'{simple!r}' == "'hello Ada: 42'", "simple"
assert f'{converted!r}' == "\"Ada|'Ada'|'Ada'\"", "converted"
assert f'{debug!r}' == "\"name = 'Ada'\"", "debug"
assert f'{unicode_ascii!r}' == "\"'caf\\\\xe9'\"", "unicode_ascii"
assert f'{container!r}' == "'[1, 2]'", "container"
assert f'{bytes_value!r}' == "\"b'xy'\"", "bytes_value"
# ---
# case: specified string formatting
text = 'cat'
width = 8
right = f'{text:>6}'
left = f'{text:.<6}'
center = f'{text:*^7}'
typed = f'{text:6s}'
truncated = f'{"bullsnake":.4s}'
nested = f'{text:>{width}}'
unicode_fill = f'{"猫":🐍^5}'
identity = f'{text:s}' is text
empty_integer = f'{42:}'
assert f'{right!r}' == "'   cat'", "right"
assert f'{left!r}' == "'cat...'", "left"
assert f'{center!r}' == "'**cat**'", "center"
assert f'{typed!r}' == "'cat   '", "typed"
assert f'{truncated!r}' == "'bull'", "truncated"
assert f'{nested!r}' == "'     cat'", "nested"
assert f'{unicode_fill!r}' == "'🐍🐍猫🐍🐍'", "unicode_fill"
assert f'{identity!r}' == "True", "identity"
assert f'{empty_integer!r}' == "'42'", "empty_integer"
# ---
# case: formatted integers
value = 42
negative = -42
width = 10
decimal = f'{value:d}'
signed_zero = f'{value:+06d}'
negative_zero = f'{negative:06d}'
hexadecimal = f'{value:#06x}'
upper_hex = f'{value:#06X}'
binary = f'{value:#010b}'
octal = f'{value:#06o}'
left = f'{value:*<6d}'
center = f'{value:*^7d}'
equal = f'{negative:*=7d}'
grouped = f'{1000000:,}'
grouped_zero = f'{1000:010,}'
grouped_hex = f'{0x12345678:_x}'
unicode_pad = f'{value:🐍>5d}'
huge_hex = f'{0x123456789abcdef0123456789abcdef:X}'
nested = f'{value:#0{width}x}'
character = f'{65:c}'
boolean = f'{True:d}'
assert f'{decimal!r}' == "'42'", "decimal"
assert f'{signed_zero!r}' == "'+00042'", "signed_zero"
assert f'{negative_zero!r}' == "'-00042'", "negative_zero"
assert f'{hexadecimal!r}' == "'0x002a'", "hexadecimal"
assert f'{upper_hex!r}' == "'0X002A'", "upper_hex"
assert f'{binary!r}' == "'0b00101010'", "binary"
assert f'{octal!r}' == "'0o0052'", "octal"
assert f'{left!r}' == "'42****'", "left"
assert f'{center!r}' == "'**42***'", "center"
assert f'{equal!r}' == "'-****42'", "equal"
assert f'{grouped!r}' == "'1,000,000'", "grouped"
assert f'{grouped_zero!r}' == "'00,001,000'", "grouped_zero"
assert f'{grouped_hex!r}' == "'1234_5678'", "grouped_hex"
assert f'{unicode_pad!r}' == "'🐍🐍🐍42'", "unicode_pad"
assert f'{huge_hex!r}' == "'123456789ABCDEF0123456789ABCDEF'", "huge_hex"
assert f'{nested!r}' == "'0x0000002a'", "nested"
assert f'{character!r}' == "'A'", "character"
assert f'{boolean!r}' == "'1'", "boolean"
# ---
# case: binary64 formatting
value = 12.5
precision = 3
fixed = f'{value:.2f}'
default_precision = f'{value:f}'
signed_zero = f'{value:+08.2f}'
negative_zero = f'{-12.5:08.1f}'
scientific = f'{1234.0:.2e}'
upper_scientific = f'{1234.0:.2E}'
percent = f'{0.125:.1%}'
grouped = f'{12345.5:,.2f}'
unicode_pad = f'{value:🐍>8.1f}'
nested = f'{value:.{precision}f}'
coerced_zero = f'{-0.0:z.1f}'
assert f'{fixed!r}' == "'12.50'", "fixed"
assert f'{default_precision!r}' == "'12.500000'", "default_precision"
assert f'{signed_zero!r}' == "'+0012.50'", "signed_zero"
assert f'{negative_zero!r}' == "'-00012.5'", "negative_zero"
assert f'{scientific!r}' == "'1.23e+03'", "scientific"
assert f'{upper_scientific!r}' == "'1.23E+03'", "upper_scientific"
assert f'{percent!r}' == "'12.5%'", "percent"
assert f'{grouped!r}' == "'12,345.50'", "grouped"
assert f'{unicode_pad!r}' == "'🐍🐍🐍🐍12.5'", "unicode_pad"
assert f'{nested!r}' == "'12.500'", "nested"
assert f'{coerced_zero!r}' == "'0.0'", "coerced_zero"
# ---
# case: general float formatting
default_fixed = f'{1.0:.3}'
default_scientific = f'{1234.0:.3}'
general_integer = f'{1.0:.3g}'
general_scientific = f'{1234.0:.3g}'
upper_general = f'{1234.0:.3G}'
default_precision = f'{1.234567:g}'
alternate = f'{1.0:#.3g}'
grouped = f'{12345.0:,.6g}'
assert f'{default_fixed!r}' == "'1.0'", "default_fixed"
assert f'{default_scientific!r}' == "'1.23e+03'", "default_scientific"
assert f'{general_integer!r}' == "'1'", "general_integer"
assert f'{general_scientific!r}' == "'1.23e+03'", "general_scientific"
assert f'{upper_general!r}' == "'1.23E+03'", "upper_general"
assert f'{default_precision!r}' == "'1.23457'", "default_precision"
assert f'{alternate!r}' == "'1.00'", "alternate"
assert f'{grouped!r}' == "'12,345'", "grouped"
