from _abc import _abc_init, _abc_register, _abc_subclasscheck, get_cache_token
before = get_cache_token()
class Base:
    pass
_abc_init(Base)
class Registered:
    pass
class Positive(Base):
    pass
class Negative:
    pass
assert _abc_register(Base, Registered) is Registered
assert _abc_subclasscheck(Base, Registered)
assert _abc_subclasscheck(Base, Positive)
assert not _abc_subclasscheck(Base, Negative)
assert get_cache_token() != before
from _abc import _get_dump
registry, positive, negative, version = _get_dump(Base)
saved_references = [next(iter(registry)), next(iter(positive)), next(iter(negative))]
saved_hashes = [hash(reference) for reference in saved_references]
class Other:
    pass
_abc_init(Other)
_abc_register(Other, Registered)
other_reference = next(iter(_get_dump(Other)[0]))
assert saved_references[0] == other_reference
