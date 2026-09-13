# case: fixed unpacking consumes native and Python iterables
x, y, z = 'aéz'
assert (x, y, z) == ('a', 'é', 'z')
x, y = {'first': 1, 'second': 2}
assert (x, y) == ('first', 'second')
x, y, z = range(3)
assert (x, y, z) == (0, 1, 2)
def values():
    yield 1
    yield 2
x, y = values()
assert (x, y) == (1, 2)
class Meta(type):
    def __iter__(cls):
        return values()
class Values(metaclass=Meta):
    pass
x, y = Values
assert (x, y) == (1, 2)

# ---
# case: fixed unpacking stops after the first excess item and keeps targets unchanged
calls = []
def values():
    for item in range(5):
        calls.append(item)
        yield item
iterator = values()
x = 'old x'
y = 'old y'
try:
    x, y = iterator
except ValueError as error:
    assert str(error) == 'too many values to unpack (expected 2)'
else:
    assert False
assert (x, y) == ('old x', 'old y')
assert calls == [0, 1, 2]
assert next(iterator) == 3
try:
    x, y = iter((1,))
except ValueError as error:
    assert str(error) == 'not enough values to unpack (expected 2, got 1)'
else:
    assert False

# ---
# case: starred unpacking splits arbitrary iterables and iterates the remainder
calls = []
class Iterator:
    def __init__(self):
        self.position = 0
    def __iter__(self):
        calls.append('iter')
        return self
    def __next__(self):
        calls.append(self.position)
        if self.position == 5:
            raise StopIteration
        result = self.position
        self.position += 1
        return result
first, *middle, last = Iterator()
assert (first, middle, last) == (0, [1, 2, 3], 4)
assert calls == ['iter', 0, 'iter', 1, 2, 3, 4, 5]
assert type(middle) is list
first, *middle, last = iter((1, 2))
assert (first, middle, last) == (1, [], 2)
*all_items, = (value for value in range(3))
assert all_items == [0, 1, 2]
first, *middle = 'aéz'
assert (first, middle) == ('a', ['é', 'z'])
try:
    first, *middle, last = iter((1,))
except ValueError as error:
    assert str(error) == 'not enough values to unpack (expected at least 2, got 1)'
else:
    assert False

# ---
# case: unpacking preserves iterator callback failures and assignment ordering
class Broken:
    def __iter__(self):
        raise TypeError('custom iteration error')
try:
    x, y = Broken()
except TypeError as error:
    assert str(error) == 'custom iteration error'
else:
    assert False
def broken():
    yield 1
    raise RuntimeError('next failed')
x, y = ('old x', 'old y')
try:
    x, y = broken()
except RuntimeError as error:
    assert str(error) == 'next failed'
else:
    assert False
assert (x, y) == ('old x', 'old y')
try:
    x, *y = broken()
except RuntimeError as error:
    assert str(error) == 'next failed'
else:
    assert False
assert (x, y) == ('old x', 'old y')
for expected in (0, 1):
    try:
        if expected:
            x, *y = 3
        else:
            x, y = 3
    except TypeError as error:
        assert str(error) == 'cannot unpack non-iterable int object'
    else:
        assert False
