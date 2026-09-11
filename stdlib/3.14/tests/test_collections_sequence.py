# Project-owned behavior tests against unchanged CPython 3.14.7 _collections_abc.
from _collections_abc import Sequence, MutableSequence, Reversible

class ReadOnly(Sequence):
    def __init__(self, source):
        self.data = list(source)
    def __getitem__(self, index):
        return self.data[index]
    def __len__(self):
        return len(self.data)

value = ReadOnly([1, 2, 1, 3])
assert list(value) == [1, 2, 1, 3]
assert 2 in value and 9 not in value
assert list(reversed(value)) == [3, 1, 2, 1]
assert value.index(1) == 0
assert value.index(1, 1) == 2
assert value.index(1, -2) == 2
assert value.index(2, 0, -1) == 1
for args in [(8,), (1, 1, 2)]:
    try:
        value.index(*args)
        assert False
    except ValueError:
        pass
assert isinstance(value, Reversible)
assert Reversible.__subclasshook__(ReadOnly) is True
assert Reversible.__subclasshook__(list) is True
assert Reversible.__subclasshook__(range) is True
assert list(list.__reversed__([1, 2])) == [2, 1]
assert list(range(3).__reversed__()) == [2, 1, 0]

class Mutable(ReadOnly, MutableSequence):
    def __setitem__(self, index, value):
        self.data[index] = value
    def __delitem__(self, index):
        self.data.pop(index)
    def insert(self, index, value):
        if index < 0:
            index = max(0, len(self.data) + index)
        index = min(index, len(self.data))
        result = self.data[:index]
        result.append(value)
        result.extend(self.data[index:])
        self.data = result

value = Mutable([1, 2])
value.append(3)
assert list(value) == [1, 2, 3]
value.extend([4, 5])
assert list(value) == [1, 2, 3, 4, 5]
assert value.pop() == 5
assert value.pop(0) == 1
value.remove(3)
assert list(value) == [2, 4]
value.reverse()
assert list(value) == [4, 2]
value.extend(value)
assert list(value) == [4, 2, 4, 2]
saved = value
value += [6]
assert value is saved
assert list(value) == [4, 2, 4, 2, 6]
value.clear()
assert list(value) == []
try:
    value.pop()
    assert False
except IndexError:
    pass

class Equal:
    def __eq__(self, other):
        return True
shared = Equal()
assert shared in ReadOnly([shared])
assert ReadOnly([Equal()]).index(Equal()) == 0
