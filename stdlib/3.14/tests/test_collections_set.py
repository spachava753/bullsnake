# Project-owned behavior tests against unchanged CPython 3.14.7 _collections_abc.
from _collections_abc import Set, MutableSet

class Values(MutableSet):
    def __init__(self, source=()):
        self.data = set(source)
    def __contains__(self, value):
        return value in self.data
    def __iter__(self):
        return iter(self.data)
    def __len__(self):
        return len(self.data)
    def add(self, value):
        self.data.add(value)
    def discard(self, value):
        self.data.discard(value)

left = Values([1, 2, 3])
right = Values([3, 4])
assert left == {1, 2, 3}
assert left != {1}
assert left > Values([1, 2])
assert Values([1]) < left
assert left >= left and left <= left
assert not left < left
assert left.isdisjoint([4, 5])
assert not left.isdisjoint([3, 5])
assert left & right == {3}
assert left | [4, 5] == {1, 2, 3, 4, 5}
assert left - [2, 3] == {1}
assert [1, 4] - left == {4}
assert left ^ right == {1, 2, 4}
assert [2, 4] & left == {2}
assert [4] | left == {1, 2, 3, 4}
assert [3, 5] ^ left == {1, 2, 5}
assert Set.__eq__(left, []) is NotImplemented
assert Set.__and__(left, 1) is NotImplemented

saved = left
left |= [4, 5]
assert left is saved and left == {1, 2, 3, 4, 5}
left &= [2, 4, 6]
assert left is saved and left == {2, 4}
left ^= [4, 5]
assert left is saved and left == {2, 5}
left -= [5, 9]
assert left is saved and left == {2}
left.remove(2)
try:
    left.remove(2)
    assert False
except KeyError:
    pass
try:
    left.pop()
    assert False
except KeyError:
    pass
left |= [1, 2]
assert left.pop() in [1, 2]
left.clear()
assert not left
left |= [1, 2]
left ^= left
assert not left
left |= [1, 2]
left -= left
assert not left

class FrozenValues(Set):
    def __init__(self, source=()):
        self.data = frozenset(source)
    def __contains__(self, value):
        return value in self.data
    def __iter__(self):
        return iter(self.data)
    def __len__(self):
        return len(self.data)
    __hash__ = Set._hash

for values in [[], [1], [1, 2, 3], [-2, 0, 1 << 80], ['a', 'b'], [(1, 2), (3, 4)]]:
    value = FrozenValues(values)
    assert value == frozenset(values)
    assert hash(value) == hash(frozenset(values))
    assert hash(value) == hash(FrozenValues(reversed(values)))

class BadHash:
    def __hash__(self):
        raise ValueError('hash failed')
class CustomSet(Set):
    def __len__(self):
        return 1
    def __iter__(self):
        yield BadHash()
    def __contains__(self, value):
        return False
try:
    CustomSet()._hash()
    assert False
except ValueError as error:
    assert str(error) == 'hash failed'
