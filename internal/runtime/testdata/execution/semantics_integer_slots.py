# case: integer native arithmetic slots execute and decline nonintegers
assert int.__add__(7, 3) == 10
assert (7).__add__(3) == 10
assert int.__rsub__(3, 7) == 4
assert int.__mul__(7, 3) == 21
assert int.__truediv__(7, 2) == 3.5
assert int.__floordiv__(-7, 3) == -3
assert int.__mod__(-7, 3) == 2
assert int.__pow__(2, 10) == 1024
assert int.__rpow__(3, 2) == 8
assert int.__lshift__(3, 4) == 48
assert int.__rshift__(-8, 2) == -2
assert int.__and__(7, 3) == 3
assert int.__or__(4, 3) == 7
assert int.__xor__(7, 3) == 4
assert int.__add__(1, 2.0) is NotImplemented
assert int.__add__(1, object()) is NotImplemented
assert type(int.__and__(True, True)) is int
assert int.__neg__(7) == -7
assert int.__pos__(-7) == -7
assert int.__invert__(7) == -8
assert int.__abs__(-7) == 7
assert int.__bool__(0) is False
assert int.__bool__(-1) is True
assert int.__int__(True) == 1
assert type(int.__index__(True)) is int
assert int.__getnewargs__(7) == (7,)
large = 1 << 100
assert int.__int__(large) is large
assert int.__index__(large) is large
assert int.__pos__(large) is large
assert int.__abs__(large) is large
assert int.__getnewargs__(large)[0] is large
negative = -large
assert int.__abs__(negative) == large
assert negative == -large

# ---
# case: integer comparisons retain native slot NotImplemented behavior
assert int.__eq__(7, 7) is True
assert int.__ne__(7, 8) is True
assert int.__lt__(7, 8) is True
assert int.__le__(7, 7) is True
assert int.__gt__(7, 6) is True
assert int.__ge__(7, 7) is True
assert int.__eq__(7, 7.0) is NotImplemented
assert int.__lt__(7, '8') is NotImplemented
assert int.__eq__(1, True) is True

# ---
# case: integer representation and formatting share numeric value semantics
assert int.__repr__(42) == '42'
assert (42).__repr__() == '42'
assert int.__repr__(True) == '1'
assert True.__repr__() == 'True'
assert int.__str__ is object.__str__
assert (42).__str__() == '42'
assert int.__str__(True) == 'True'
assert int.__format__(42, '04x') == '002a'
assert (42).__format__('#x') == '0x2a'
assert int.__format__(True, '') == 'True'
assert int.__format__(True, '04d') == '0001'
assert type(int.__repr__).__name__ == 'wrapper_descriptor'
assert type(int.__format__).__name__ == 'method_descriptor'
assert int.__add__.__objclass__ is int
assert int.__add__.__get__(3, int)(4) == 7

# ---
# case: integer slot errors preserve receivers arity and arithmetic failures
for call in (
    lambda: int.__add__('3', 4),
    lambda: int.__add__(3),
    lambda: int.__add__(3, 4, 5),
    lambda: int.__add__(3, other=4),
    lambda: int.__format__(3, 1),
    lambda: int.__index__('3'),
):
    try:
        call()
    except TypeError:
        pass
    else:
        assert False
try:
    int.__floordiv__(1, 0)
except ZeroDivisionError:
    pass
else:
    assert False
try:
    int.__lshift__(1, -1)
except ValueError as error:
    assert str(error) == 'negative shift count'
else:
    assert False
