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
# ---
# case: template iteration alternates nonempty parts
first = 1
second = 2
template = t'left {first} middle {second} right'
parts = ()
for part in template:
    parts = (*parts, part)

assert parts[0] == 'left '
assert parts[1] is template.interpolations[0]
assert parts[2] == ' middle '
assert parts[3] is template.interpolations[1]
assert parts[4] == ' right'
# ---
# case: template iteration skips empty strings
first = 1
second = 2
only_interpolations = t'{first}{second}'
parts = ()
for part in only_interpolations:
    parts = (*parts, part)

empty_count = 0
for absent in t'':
    empty_count = empty_count + 1
else:
    empty_completed = True

second_pass = ()
for part in only_interpolations:
    second_pass = (*second_pass, part)

assert parts == only_interpolations.interpolations
assert empty_count == 0
assert empty_completed is True
assert second_pass == parts
# ---
# case: template concatenation merges boundary strings
left_value = 'left'
right_value = 'right'
left = t'A{left_value}B'
right = t'C{right_value}D'
combined = left + right

assert combined.strings == ('A', 'BC', 'D')
assert combined.interpolations[0] is left.interpolations[0]
assert combined.interpolations[1] is right.interpolations[0]
assert combined.values == ('left', 'right')
assert left.strings == ('A', 'B')
assert right.strings == ('C', 'D')
# ---
# case: template concatenation handles empty boundaries
first = 1
second = 2
plain = t'' + t'plain'
adjacent = t'{first}' + t'{second}'
in_place = t'prefix '
in_place += t'{first}'

assert plain.strings == ('plain',)
assert plain.interpolations == ()
assert adjacent.strings == ('', '', '')
assert adjacent.interpolations[0].value == 1
assert adjacent.interpolations[1].value == 2
assert in_place.strings == ('prefix ', '')
assert in_place.values == (1,)
