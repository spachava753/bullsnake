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
