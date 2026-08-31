# Python 3.14 template string values.
# case: basic template metadata
name = 'Python'
template = t'Hello, {name}'
interpolation = template.interpolations[0]

assert template.strings == ('Hello, ', '')
assert template.values == ('Python',)
assert interpolation.value is name
assert interpolation.expression == 'name'
assert interpolation.conversion is None
assert interpolation.format_spec == ''
assert f'{template!r}' == "Template(strings=('Hello, ', ''), interpolations=(Interpolation('Python', 'name', None, ''),))"
assert f'{interpolation!r}' == "Interpolation('Python', 'name', None, '')"
# ---
# case: template conversions formats and debug fields
value = 12
width = 4
formatted = t'{value!r:0>{width}}'
debugged = t'{ value = }'
raw = rt'\n{value}\t'
braced = t'{{{value}}}'

formatted_interpolation = formatted.interpolations[0]
debug_interpolation = debugged.interpolations[0]

assert formatted.strings == ('', '')
assert formatted_interpolation.value == 12
assert formatted_interpolation.expression == 'value'
assert formatted_interpolation.conversion == 'r'
assert formatted_interpolation.format_spec == '0>4'
assert debugged.strings == (' value = ', '')
assert debug_interpolation.value == 12
assert debug_interpolation.expression == 'value'
assert debug_interpolation.conversion == 'r'
assert debug_interpolation.format_spec == ''
assert raw.strings == ('\\n', '\\t')
assert raw.values == (12,)
assert braced.strings == ('{', '}')
# ---
# case: adjacent and nested template strings
name = 'Ada'
adjacent = t'left ' t'{name}' t' right'
inner = t'{name}'
outer = t'language: {inner}'

assert adjacent.strings == ('left ', ' right')
assert adjacent.values == ('Ada',)
assert adjacent.interpolations[0].expression == 'name'
assert outer.strings == ('language: ', '')
assert outer.values[0] is inner
assert outer.interpolations[0].expression == 'inner'
# ---
# case: template expressions run once in source order
calls = 0
def capture(value):
    global calls
    calls = calls + 1
    return value

ordered = t'{capture(1)}:{capture(2)}'

assert calls == 2
assert ordered.strings == ('', ':', '')
assert ordered.values == (1, 2)
assert ordered.interpolations[0].expression == 'capture(1)'
assert ordered.interpolations[1].expression == 'capture(2)'
