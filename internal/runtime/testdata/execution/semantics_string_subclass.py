# case: string subclasses retain native text and separate Python attributes
class Text(str):
    pass
value = Text('aéz')
assert type(value) is Text and isinstance(value, str)
assert Text.__mro__ == (Text, str, object)
assert str(value) == 'aéz' and type(str(value)) is str
assert repr(value) == "'aéz'"
assert hash(value) == hash('aéz')
assert len(value) == 3 and list(value) == ['a', 'é', 'z']
assert bool(Text('')) is False
assert value[1] == 'é' and value[:2] == 'aé'
assert type(value[:]) is str
assert 'é' in value and Text('é') in value
assert value == 'aéz' and 'aéz' == value
assert value < 'z' and 'a' < value
value.label = 'text'
assert value.label == 'text'
assert value.lower() == 'aéz'
assert Text(' AÉZ ').strip().lower() == 'aéz'
assert str.__format__(value, '>5') == '  aéz'
assert str.__str__(value) == 'aéz' and type(str.__str__(value)) is str

# ---
# case: string methods accept native subtype text without invoking conversions
class Text(str):
    def __str__(self):
        return 'wrong'
    def __repr__(self):
        return 'repr override'
    def __hash__(self):
        return 17
    def __len__(self):
        return 99
    def lower(self):
        return 'lower override'
value = Text('ABC')
assert str(value) == 'wrong' and repr(value) == 'repr override'
assert hash(value) == 17 and len(value) == 99
assert str.__hash__(value) == hash('ABC')
assert str.__len__(value) == 3
assert str.lower(value) == 'abc' and value.lower() == 'lower override'
assert str.__format__(value, '') == 'wrong'
assert str.__format__(value, '>5') == '  ABC'
assert str.__repr__(value) == "'ABC'"
assert '|'.join((value, Text('D'))) == 'ABC|D'
assert Text('|').join(('a', 'b')) == 'a|b'
assert str.count('ABCABC', value) == 2
assert 'ABC'.startswith(Text('A'))
assert 'ABC'.endswith((Text('Z'), Text('C')))
assert 'ABC'.replace(Text('B'), Text('!')) == 'A!C'
assert 'ABC'.removeprefix(Text('A')) == 'BC'
assert 'A,B'.split(Text(',')) == ['A', 'B']
assert 'ABC'.strip(Text('AC')) == 'B'
assert Text('[%s]') % 'item' == '[item]'
class Reflected(str):
    def __rmod__(self, format):
        return 'reflected'
assert '%s' % Reflected('item') == 'reflected'
assert str.__mod__('%s', Reflected('item')) == 'item'
class Protocols(str):
    def __iter__(self):
        return iter(('override',))
    def __contains__(self, value):
        return False
    def __getitem__(self, key):
        return 'item override'
p = Protocols('abc')
assert list(p) == ['override']
assert list(str.__iter__(p)) == ['a', 'b', 'c']
assert 'a' not in p and str.__contains__(p, 'a') is True
assert p[0] == 'item override' and str.__getitem__(p, 0) == 'a'

# ---
# case: string subclass construction and comparison honor native and Python order
calls = []
class Text(str):
    def __new__(cls, value):
        calls.append(('new', value))
        return str.__new__(cls, value)
    def __init__(self, value):
        calls.append(('init', value))
    def __eq__(self, other):
        calls.append('eq')
        return NotImplemented
value = Text('text')
assert calls == [('new', 'text'), ('init', 'text')]
assert value == 'text' and 'text' == value
assert calls[-2:] == ['eq', 'eq']
assert str.__eq__(value, 'text') is True
class Foreign(str):
    def __new__(cls):
        return 42
    def __init__(self):
        assert False
assert Foreign() == 42
try:
    object.__new__(Text)
except TypeError:
    pass
else:
    assert False

# ---
# case: mixed string bases preserve actual native descriptor positions
class Mixin:
    def __repr__(self):
        return 'mixin repr'
class NativeFirst(str, Mixin):
    pass
class PythonFirst(Mixin, str):
    pass
assert NativeFirst.__mro__ == (NativeFirst, str, Mixin, object)
assert PythonFirst.__mro__ == (PythonFirst, Mixin, str, object)
assert NativeFirst.__base__ is str and PythonFirst.__base__ is str
assert repr(NativeFirst('text')) == "'text'"
assert repr(PythonFirst('text')) == 'mixin repr'
class Child(NativeFirst):
    def __new__(cls, value):
        return super().__new__(cls, value)
assert Child('text') == 'text'
try:
    type('Incompatible', (str, int), {})
except TypeError:
    pass
else:
    assert False
