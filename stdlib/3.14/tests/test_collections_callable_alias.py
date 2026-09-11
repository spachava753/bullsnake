# Project-owned behavior tests against unchanged CPython 3.14.7 _collections_abc.
from _collections_abc import Callable, _CallableGenericAlias

for arguments in [[int, str], (int, str)]:
    alias = Callable[arguments, int]
    assert type(alias) is _CallableGenericAlias
    assert alias.__origin__ is Callable
    assert alias.__args__ == (int, str, int)
    constructor, args = alias.__reduce__()
    restored = constructor(*args)
    assert restored.__origin__ is Callable
    assert restored.__args__ == alias.__args__
assert Callable[[], int].__args__ == (int,)
assert Callable[..., str].__args__ == (..., str)
assert repr(Callable[..., str]) == 'collections.abc.Callable[..., str]'
for arguments in [(), (int,), (int, str, int), (int, str)]:
    try:
        Callable[arguments]
        assert False
    except TypeError:
        pass
