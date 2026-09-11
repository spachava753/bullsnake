# case: sum folds integers and generators with positional or keyword start
assert sum([]) == 0
assert sum([1, 2, 3]) == 6
assert sum(range(10000)) == 49995000
assert sum(x * x for x in range(4)) == 14
assert sum([True, False, True]) == 2
assert sum([1 << 100, 2, -(1 << 100)]) == 2
assert sum([1, 2], 10) == 13
assert sum([1, 2], start=10) == 13
marker = object()
assert sum([], marker) is marker
assert sum([], True) is True

# ---
# case: sum calls addition rather than in-place addition
calls = []
class Total:
    def __init__(self, value):
        self.value = value
    def __add__(self, other):
        calls.append(other)
        return Total(self.value + other)
    def __iadd__(self, other):
        raise AssertionError('in-place addition')
start = Total(10)
result = sum([1, 2, 3], start)
assert result.value == 16
assert start.value == 10
assert result is not start
assert calls == [1, 2, 3]
class Reflected:
    def __radd__(self, other):
        return other + 5
assert sum([Reflected(), Reflected()]) == 10

# ---
# case: sum uses compensated floating point accumulation
assert sum([0.1 for _ in range(10)]) == 1.0
assert sum([1e16, 1.0, -1e16]) == 1.0
assert sum([1, 0.5, 2]) == 3.5
assert sum([], -0.0) == -0.0
assert sum([1e308, 1e308]) == 1e309

# ---
# case: sum preserves iterator and addition errors and checks start after iteration setup
calls = []
class Source:
    def __iter__(self):
        calls.append('iter')
        return iter([])
for start in ['', b'', bytearray()]:
    try:
        sum(Source(), start)
        assert False
    except TypeError:
        pass
assert calls == ['iter', 'iter', 'iter']
def source():
    yield 1
    raise ValueError('iterator failed')
try:
    sum(source())
    assert False
except ValueError as error:
    assert str(error) == 'iterator failed'
class Broken:
    def __radd__(self, other):
        raise ValueError('addition failed')
try:
    sum([Broken()])
    assert False
except ValueError as error:
    assert str(error) == 'addition failed'
for call in [lambda: sum(), lambda: sum([], 1, 2), lambda: sum(iterable=[]), lambda: sum([], 1, start=2), lambda: sum([], other=2), lambda: sum([], **{1: 2})]:
    try:
        call()
        assert False
    except TypeError:
        pass
