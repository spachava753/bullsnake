# case: integer subclasses retain immutable value storage and mutable attributes
class Number(int):
    pass
value = Number(42)
assert type(value) is Number
assert isinstance(value, int)
assert issubclass(Number, int)
assert Number.__base__ is int
assert Number.__bases__ == (int,)
assert Number.__mro__ == (Number, int, object)
assert int(value) == 42
assert type(int(value)) is int
value.label = 'answer'
assert value.label == 'answer'
assert str(value) == '42' and repr(value) == '42'
assert repr([value]) == '[42]'
assert hash(value) == hash(42)
assert bool(Number(0)) is False
assert bool(value) is True
assert value + 1 == 43 and 1 + value == 43
assert value - 2 == 40 and 50 - value == 8
assert value * 2 == 84 and 2 * value == 84
assert value // 5 == 8 and value % 5 == 2
assert value / 2 == 21.0
assert value ** 2 == 1764
assert value << 1 == 84
assert value & 7 == 2
assert type(value + Number(1)) is int
assert type(-value) is int
assert int.__int__(value) == 42
assert int.__hash__(value) == 42
assert int.__format__(value, '04x') == '002a'
assert value == 42 and 42 == value
assert value != 43 and value < 43 and 43 > value
assert value == 42.0 and 42.0 == value
assert value + 0.5 == 42.5 and 0.5 + value == 42.5

# ---
# case: integer subtype constructors run allocation before initialization
calls = []
class Number(int):
    def __new__(cls, value, *, label):
        calls.append(('new', cls, value))
        return int.__new__(cls, value + 1)
    def __init__(self, value, *, label):
        calls.append(('init', value))
        self.label = label
value = Number(41, label='answer')
assert calls == [('new', Number, 41), ('init', 41)]
assert value == 42 and value.label == 'answer'
raw = int.__new__(Number, 7)
assert raw == 7 and not hasattr(raw, 'label')
class Foreign(int):
    def __new__(cls):
        return 'foreign'
    def __init__(self):
        assert False
assert Foreign() == 'foreign'
class Child(Number):
    pass
child = Child(8, label='child')
assert type(child) is Child and child == 9
try:
    object.__new__(Number)
except TypeError:
    pass
else:
    assert False

# ---
# case: native integer slots bypass overrides while operators preserve dispatch
calls = []
class Number(int):
    def __int__(self):
        calls.append('int')
        return 99
    def __repr__(self):
        return 'custom repr'
    def __str__(self):
        return 'custom str'
    def __hash__(self):
        return 17
    def __bool__(self):
        return False
    def __add__(self, other):
        calls.append('add')
        return NotImplemented
    def __radd__(self, other):
        calls.append('radd')
        return NotImplemented
    def __eq__(self, other):
        calls.append('eq')
        return NotImplemented
n = Number(3)
assert int(n) == 99
assert repr(n) == 'custom repr' and str(n) == 'custom str'
assert hash(n) == 17 and bool(n) is False
assert int.__int__(n) == 3 and int.__repr__(n) == '3'
assert int.__hash__(n) == 3 and int.__bool__(n) is True
assert int.__format__(n, '') == 'custom str'
assert int.__format__(n, 'd') == '3'
assert n + 4 == 7 and 4 + n == 7
assert n == 3 and 3 == n
assert calls == ['int', 'add', 'radd', 'eq', 'eq']
# A native float operation has priority over an unrelated int-subclass override.
calls = []
assert 3.0 == n
assert 1.0 + n == 4.0
assert calls == []

# ---
# case: native integer rounding does not suppress subtype hooks
class Number(int):
    def __round__(self, ndigits=None):
        return ('custom', ndigits)
value = Number(125)
assert round(value) == ('custom', None)
assert round(value, -1) == ('custom', -1)
assert int.__round__(value, -1) == 120
class Plain(int):
    pass
assert round(Plain(125), -1) == 120
assert type(round(Plain(125))) is int
assert [0, 1, 2][Plain(1)] == 1

# ---
# case: integer conversion hooks validate results and preserve errors
calls = []
class Indexed:
    def __index__(self):
        calls.append('index')
        return 42
assert int(Indexed()) == 42
class Number(int):
    pass
assert Number(Indexed()) == 42
assert calls == ['index', 'index']
class Bad:
    def __int__(self):
        return 'not an integer'
    def __index__(self):
        assert False
try:
    int(Bad())
except TypeError as error:
    assert str(error) == '__int__ returned non-int (type str)'
else:
    assert False
class Failure:
    def __int__(self):
        raise ValueError('conversion failed')
try:
    Number(Failure())
except ValueError as error:
    assert str(error) == 'conversion failed'
else:
    assert False
