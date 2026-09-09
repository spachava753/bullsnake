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
