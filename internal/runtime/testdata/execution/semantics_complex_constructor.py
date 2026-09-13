# case: numeric complex construction preserves components and native identity
assert complex() == 0j
assert complex(2).real == 2.0
assert complex(2).imag == 0.0
value = complex(1, -2)
assert value.real == 1.0
assert value.imag == -2.0
assert repr(value) == '(1-2j)'
assert complex(value) is value
assert value.__complex__() is value
assert complex(real=3, imag=4).real == 3.0
assert complex(imag=2) == 2j
assert complex(True, False).real == 1.0
assert type(value) is complex
assert type(value.real) is type(1.0)
slot = complex.real
assert slot.__get__(value) == 1.0
try:
    value.real = 5
    assert False
except AttributeError:
    pass

# ---
# case: single-argument complex construction invokes class conversion hooks
calls = []
expected = complex(3, 4)
class Number:
    def __complex__(self):
        calls.append('complex')
        return expected
number = Number()
number.__complex__ = lambda: 0j
assert complex(number) is expected
assert calls == ['complex']
class Bad:
    def __complex__(self):
        return 1
try:
    complex(Bad())
    assert False
except TypeError as error:
    assert str(error) == '__complex__ returned non-complex (type int)'

# ---
# case: complex constructor validates shape and numeric overflow
try:
    complex(1, real=2)
    assert False
except TypeError as error:
    assert str(error) == "argument for complex() given by name ('real') and position (1)"
try:
    complex(1, 2, 3)
    assert False
except TypeError as error:
    assert str(error) == 'complex() takes at most 2 arguments (3 given)'
try:
    complex(10**1000)
    assert False
except OverflowError as error:
    assert str(error) == 'int too large to convert to float'
try:
    complex('1j')
    assert False
except NotImplementedError as error:
    assert str(error) == 'complex() string parsing is not supported'
try:
    complex(1j, 2)
    assert False
except NotImplementedError as error:
    assert str(error) == 'complex() deprecated complex-valued component arguments require warning support'
try:
    complex('1', 2)
    assert False
except TypeError as error:
    assert str(error) == "complex() can't take second arg if first is a string"
try:
    complex(1, '2')
    assert False
except TypeError as error:
    assert str(error) == "complex() second arg can't be a string"
