# case: percent text conversions consume single values or positional tuples
assert '_%s__' % ('Enum',) == '_Enum__'
assert '%s:%r:%a:%%' % ('é', 'é', 'é') == "é:'é':'\\xe9':%"
assert '%s' % 42 == '42'
assert '%s' % None == 'None'
assert '%r' % ((1, 2),) == '(1, 2)'
assert 'literal %%' % () == 'literal %'
assert 'literal' % {} == 'literal'
assert '%s' % '\ud800' == '\ud800'
assert str.__mod__('%s', 'value') == 'value'
assert '%s'.__mod__('bound') == 'bound'
assert str.__rmod__('value', '%s') == 'value'
assert str.__rmod__('value', 1) is NotImplemented
text = '%s'
text %= 'replaced'
assert text == 'replaced'

# ---
# case: percent conversions resume Python callbacks in order
calls = []
class Value:
    def __str__(self):
        calls.append('str')
        return 'text'
    def __repr__(self):
        calls.append('repr')
        return 'é'
    def __rmod__(self, left):
        assert False
value = Value()
assert '%s/%r/%a' % (value, value, value) == 'text/é/\\xe9'
assert calls == ['str', 'repr', 'repr']
try:
    '%s %s' % (value,)
    assert False
except TypeError as error:
    assert str(error) == 'not enough arguments for format string'
assert calls == ['str', 'repr', 'repr', 'str']

# ---
# case: percent text formatting errors do not report success
try:
    '%s' % ()
    assert False
except TypeError as error:
    assert str(error) == 'not enough arguments for format string'
try:
    '%s' % ('one', 'extra')
    assert False
except TypeError as error:
    assert str(error) == 'not all arguments converted during string formatting'
try:
    'trailing %' % ()
    assert False
except ValueError as error:
    assert str(error) == 'incomplete format'
class Bad:
    def __str__(self):
        return 1
try:
    '%s' % Bad()
    assert False
except TypeError as error:
    assert str(error) == '__str__ returned non-string (type int)'

# ---
# case: large positional percent formatting does not grow native call depth
format = ''.join('%s' for item in range(10000))
values = tuple('x' for item in range(10000))
result = format % values
assert len(result) == 10000
assert result[0] == result[-1] == 'x'

# ---
# case: unsupported percent formatting families remain explicit
try:
    '%5s' % 'value'
    assert False
except NotImplementedError as error:
    assert str(error) == 'percent formatting supports only positional %s, %r, %a, and %%'
try:
    '%z' % 'value'
    assert False
except ValueError as error:
    assert str(error) == "unsupported format character 'z' (0x7a) at index 1"
