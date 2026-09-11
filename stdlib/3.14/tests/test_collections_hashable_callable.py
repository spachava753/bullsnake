# Project-owned behavior tests for remaining simple collection ABC protocols.
from _collections_abc import Hashable, Callable

for value in [None, True, 1, 1.5, 2j, 'a', b'a', (), frozenset(), object()]:
    assert isinstance(value, Hashable)
    assert callable(value.__hash__)
    assert value.__hash__() == hash(value)
for value in [[], {}, set(), bytearray(), {}.keys(), {}.items(), {}.values()]:
    assert not isinstance(value, Hashable)
    assert value.__hash__ is None
    try:
        hash(value)
        assert False
    except TypeError:
        pass

class Default:
    pass
class Equal:
    def __eq__(self, other):
        return True
class Retained(Equal):
    __hash__ = object.__hash__
assert isinstance(Default(), Hashable)
assert not isinstance(Equal(), Hashable)
assert isinstance(Retained(), Hashable)
retained = Retained()
assert hash(retained) == retained.__hash__()
assert Hashable.__hash__(None) == 0

class Called:
    def __call__(self, value=0):
        return value + 1
class Disabled(Called):
    __call__ = None
assert isinstance(Called(), Callable)
assert not isinstance(Disabled(), Callable)
for value in [lambda: None, len, [].append, Called, int]:
    assert isinstance(value, Callable)
assert not isinstance(1, Callable)
assert Callable.__call__(None, 1, ignored=True) is False

function = lambda value=0: value + 2
assert function.__call__(value=3) == 5
assert len.__call__([1, 2]) == 2
assert type(function).__dict__['__call__'](function, 4) == 6
assert type.__call__(Called)(4) == 5
