# case: dumps return independent sets of live callable weak references
from _abc import _abc_init, _abc_register, _abc_subclasscheck, _get_dump, _reset_registry, get_cache_token
class Base:
    pass
class Registered:
    pass
class Positive(Base):
    pass
class Negative:
    pass
_abc_init(Base)
_abc_register(Base, Registered)
assert _abc_subclasscheck(Base, Registered)
assert _abc_subclasscheck(Base, Positive)
assert not _abc_subclasscheck(Base, Negative)
registry, positive, negative, version = _get_dump(Base)
assert len(registry) == len(positive) == len(negative) == 1
assert version == get_cache_token()
reference = next(iter(registry))
assert callable(reference)
assert reference() is Registered
assert reference.__callback__ is None
assert hash(reference) == hash(Registered)
assert next(iter(positive))() is Positive
assert next(iter(negative))() is Negative
second = _get_dump(Base)[0]
assert second == registry
assert second is not registry
assert next(iter(second)) is reference
registry.discard(reference)
assert len(_get_dump(Base)[0]) == 1
_reset_registry(Base)
assert len(_get_dump(Base)[0]) == 0
assert reference() is Registered
assert len(second) == 1
# ---
# case: weak diagnostic references compare by live class identity
from _abc import _abc_init, _abc_register, _get_dump
class Left:
    pass
class Right:
    pass
class Registered:
    pass
for base in (Left, Right):
    _abc_init(base)
    _abc_register(base, Registered)
left = next(iter(_get_dump(Left)[0]))
right = next(iter(_get_dump(Right)[0]))
assert left == right
assert not (left != right)
assert hash(left) == hash(right)
assert left != Registered
for call in (lambda: left(1), lambda: left(x=1), lambda: _get_dump(), lambda: _get_dump(Left, x=1)):
    try:
        call()
        assert False
    except TypeError:
        pass
# ---
# case: dumping and resetting caches do not refresh stale negative versions
from _abc import _abc_init, _abc_register, _abc_subclasscheck, _get_dump, _reset_caches, get_cache_token
class First:
    pass
class Second:
    pass
_abc_init(First)
_abc_init(Second)
assert not _abc_subclasscheck(First, int)
old = _get_dump(First)[3]
_abc_register(Second, str)
assert get_cache_token() != old
assert _get_dump(First)[3] == old
assert len(_get_dump(First)[2]) == 1
_reset_caches(First)
assert _get_dump(First)[3] == old
assert not _abc_subclasscheck(First, int)
assert _get_dump(First)[3] == get_cache_token()
reference = next(iter(_get_dump(First)[2]))
assert type(reference).__name__ == 'ReferenceType'
assert type(reference).__module__ == 'weakref'
