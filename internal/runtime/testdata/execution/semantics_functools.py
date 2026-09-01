# case: cmp_to_key wrappers and ordering
from _functools import cmp_to_key

def compare(left, right):
    if left < right:
        return -1
    if left > right:
        return 1
    return 0

key = cmp_to_key(compare)
first = key(1)
second = key(obj=2)
equal = key(1)
assert callable(key)
assert callable(first)
assert first.obj == 1
assert second.obj == 2
assert type(first).__name__ == 'KeyWrapper'
assert first < second
assert first <= second
assert second > first
assert second >= first
assert first == equal
assert first != second
# ---
# case: cmp_to_key drives stable sorting
from _functools import cmp_to_key

calls = []
def descending(left, right):
    calls.append((left[0], right[0]))
    if left[1] > right[1]:
        return -1
    if left[1] < right[1]:
        return 1
    return 0

values = [('first', 1), ('second', 1), ('third', 2)]
key = cmp_to_key(mycmp=descending)
assert sorted(values, key=key) == [
    ('third', 2),
    ('first', 1),
    ('second', 1),
]
assert len(calls) > 0
assert values.sort(key=key) is None
assert values == [('third', 2), ('first', 1), ('second', 1)]
# ---
# case: retained cmp_to_key export
import _functools

converter = _functools.cmp_to_key
assert callable(converter)
def equal(left, right):
    return 0

wrapped = converter(equal)('value')
assert wrapped.obj == 'value'
