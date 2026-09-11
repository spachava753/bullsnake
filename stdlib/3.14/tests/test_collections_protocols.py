# Project-owned behavior tests against unchanged CPython 3.14.7 _collections_abc.
from _collections_abc import Iterable, Iterator, Sized, Container, Collection

for value in [[], (), {}, set(), frozenset(), '', b'', bytearray(), range(0), {}.keys(), {}.values(), {}.items()]:
    assert Iterable.__subclasshook__(type(value)) is True
    assert Sized.__subclasshook__(type(value)) is True
    assert isinstance(value, Iterable)
    assert isinstance(value, Sized)
    assert value.__len__() == 0
    assert list(value.__iter__()) == []
for value, present, absent in [([1], 1, 2), ((1,), 1, 2), ({1: 2}, 1, 3), ({1}, 1, 2), (frozenset({1}), 1, 2), ('ab', 'a', 'c'), (b'ab', 97, 99)]:
    assert Container.__subclasshook__(type(value)) is True
    assert Collection.__subclasshook__(type(value)) is True
    assert value.__contains__(present)
    assert not value.__contains__(absent)
for value in [iter([1]), iter((1,)), iter({1: 2}), iter({1}), iter('a'), iter(b'a'), iter(bytearray(b'a')), iter(range(1)), iter({1: 2}.keys()), iter({1: 2}.values()), iter({1: 2}.items()), reversed([1]), enumerate([1]), map(lambda x: x, [1]), filter(None, [1]), zip([1]), (lambda: (yield 1))()]:
    assert Iterator.__subclasshook__(type(value)) is True
    assert value.__iter__() is value
    value.__next__()
    try:
        value.__next__()
        assert False
    except StopIteration:
        pass

class Structural:
    def __iter__(self):
        return iter([1])
    def __len__(self):
        return 1
    def __contains__(self, value):
        return value == 1
class Disabled(Structural):
    __iter__ = None
assert isinstance(Structural(), Collection)
assert not isinstance(Disabled(), Iterable)
assert not isinstance(1, Collection)

# Direct defaults are callable even though their defining classes are abstract.
assert list(Iterable.__iter__(None)) == []
assert Sized.__len__(None) == 0
assert Container.__contains__(None, 1) is False
