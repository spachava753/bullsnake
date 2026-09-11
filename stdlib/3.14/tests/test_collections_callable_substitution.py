# Project-owned behavior tests against unchanged CPython 3.14.7 _collections_abc.
from _collections_abc import Callable, _CallableGenericAlias

def check_substitution[T, U]():
    alias = Callable[[T, list[U]], U]
    assert alias.__parameters__ == (T, U)
    specialized = alias[int, str]
    assert type(specialized) is _CallableGenericAlias
    assert specialized == Callable[[int, list[str]], str]
    assert specialized.__parameters__ == ()
    assert alias[T, str][int] == specialized
check_substitution()

def check_paramspec[**P, T, **Q]():
    alias = Callable[P, int]
    assert alias.__parameters__ == (P,)
    assert alias[[str, bytes]] == Callable[[str, bytes], int]
    assert alias[str, bytes] == Callable[[str, bytes], int]
    assert alias[[]] == Callable[[], int]
    assert alias[...] == Callable[..., int]
    assert alias[Q].__parameters__ == (Q,)
    assert alias[Q][[str]] == Callable[[str], int]
    both = Callable[P, T]
    assert both[[str, None], bytes] == Callable[[str, type(None)], bytes]
    assert both[[str], T][int] == Callable[[str], int]
    try:
        both[str, bytes]
        assert False
    except TypeError:
        pass
    try:
        alias[()]
        assert False
    except TypeError:
        pass
check_paramspec()

def check_default[**P = [str, bytes]]():
    alias = Callable[P, int]
    assert alias[()] == Callable[[str, bytes], int]
check_default()
