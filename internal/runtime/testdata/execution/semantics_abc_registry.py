# case: virtual registration shares checks but does not change inheritance
from _abc import _abc_init, _abc_register, _abc_subclasscheck, _abc_instancecheck, get_cache_token
class Meta(type):
    def __new__(mcls, name, bases, ns):
        cls = super().__new__(mcls, name, bases, ns)
        _abc_init(cls)
        return cls
    def register(cls, other):
        return _abc_register(cls, other)
    def __subclasscheck__(cls, other):
        return _abc_subclasscheck(cls, other)
    def __instancecheck__(cls, other):
        return _abc_instancecheck(cls, other)
class Base(metaclass=Meta):
    def method(self):
        return 42
class Virtual:
    pass
class Child(Virtual):
    pass
before = get_cache_token()
assert not issubclass(Virtual, Base)
assert not isinstance(Child(), Base)
assert Base.register(Virtual) is Virtual
assert get_cache_token() != before
assert issubclass(Virtual, Base)
assert issubclass(Child, Base)
assert isinstance(Virtual(), Base)
assert isinstance(Child(), Base)
assert Virtual.__bases__ == (object,)
assert not hasattr(Virtual(), 'method')
after = get_cache_token()
assert Base.register(Virtual) is Virtual
assert Base.register(Base) is Base
assert get_cache_token() == after
# ---
# case: native and exception classes can be registered
from _abc import _abc_init, _abc_register, _abc_subclasscheck, _abc_instancecheck
class Base:
    @classmethod
    def __subclasscheck__(cls, other):
        return _abc_subclasscheck(cls, other)
_abc_init(Base)
for cls, instance in ((int, 1), (ValueError, ValueError())):
    assert _abc_register(Base, cls) is cls
    assert _abc_subclasscheck(Base, cls)
    assert _abc_instancecheck(Base, instance)
assert _abc_subclasscheck(Base, bool)
assert not _abc_instancecheck(Base, 'text')
# ---
# case: hooks precede inheritance and cache exact boolean decisions
from _abc import _abc_init, _abc_subclasscheck, _reset_caches, get_cache_token
seen = []
class Base:
    @classmethod
    def __subclasshook__(cls, candidate):
        seen.append(candidate)
        return candidate is int
_abc_init(Base)
assert _abc_subclasscheck(Base, int)
assert _abc_subclasscheck(Base, int)
assert not _abc_subclasscheck(Base, Base)
assert not _abc_subclasscheck(Base, Base)
assert seen == [int, Base]
old = get_cache_token()
assert _reset_caches(Base) is None
assert _abc_subclasscheck(Base, int)
assert seen == [int, Base, int]
assert get_cache_token() == old
# ---
# case: invalid hooks and hook errors are not cached
from _abc import _abc_init, _abc_subclasscheck
class Base:
    @classmethod
    def __subclasshook__(cls, candidate):
        return 1
_abc_init(Base)
try:
    _abc_subclasscheck(Base, int)
    assert False
except AssertionError as error:
    assert str(error) == '__subclasshook__ must return either False, True, or NotImplemented'
class Failure:
    @classmethod
    def __subclasshook__(cls, candidate):
        raise ValueError('hook failed')
_abc_init(Failure)
for _ in range(2):
    try:
        _abc_subclasscheck(Failure, int)
        assert False
    except ValueError as error:
        assert str(error) == 'hook failed'
# ---
# case: registry and cache resets preserve independent state
from _abc import _abc_init, _abc_register, _abc_subclasscheck, _reset_registry, _reset_caches, get_cache_token
class Base:
    pass
_abc_init(Base)
_abc_register(Base, int)
assert _abc_subclasscheck(Base, bool)
old = get_cache_token()
assert _reset_registry(Base) is None
assert _abc_subclasscheck(Base, bool)
assert _reset_caches(Base) is None
assert not _abc_subclasscheck(Base, bool)
assert get_cache_token() == old
# ---
# case: invalid registration and helper calls fail without changing token
from _abc import _abc_init, _abc_register, _abc_subclasscheck, _abc_instancecheck, _reset_registry, _reset_caches, get_cache_token
class Base:
    pass
_abc_init(Base)
old = get_cache_token()
try:
    _abc_register(Base, 1)
    assert False
except TypeError as error:
    assert str(error) == 'Can only register classes'
try:
    _abc_subclasscheck(Base, 1)
    assert False
except TypeError as error:
    assert str(error) == 'issubclass() arg 1 must be a class'
for call in (lambda: get_cache_token(1), lambda: get_cache_token(x=1),
             lambda: _abc_register(Base), lambda: _abc_register(Base, int, x=1),
             lambda: _abc_subclasscheck(Base), lambda: _abc_instancecheck(Base),
             lambda: _reset_registry(), lambda: _reset_caches(Base, x=1)):
    try:
        call()
        assert False
    except TypeError:
        pass
assert get_cache_token() == old
Base._abc_impl = None
try:
    _reset_caches(Base)
    assert False
except TypeError as error:
    assert str(error) == '_abc_impl is set to a wrong type'
del Base._abc_impl
try:
    _reset_registry(Base)
    assert False
except AttributeError:
    pass
# ---
# case: transitive registration invalidates older negative answers across ABCs
from _abc import _abc_init, _abc_register, _abc_subclasscheck, get_cache_token
class Meta(type):
    def __new__(mcls, name, bases, ns):
        cls = super().__new__(mcls, name, bases, ns)
        _abc_init(cls)
        return cls
    def register(cls, other):
        return _abc_register(cls, other)
    def __subclasscheck__(cls, other):
        return _abc_subclasscheck(cls, other)
class Root(metaclass=Meta):
    pass
class Derived(Root):
    pass
class Middle(metaclass=Meta):
    pass
class Concrete:
    pass
assert not issubclass(Concrete, Root)
Derived.register(Middle)
assert not issubclass(Concrete, Root)
Middle.register(Concrete)
assert issubclass(Concrete, Derived)
assert issubclass(Concrete, Root)
old = get_cache_token()
try:
    ConcreteChild = type('ConcreteChild', (Concrete,), {})
    Middle.register(Root)
    assert False
except RuntimeError as error:
    assert str(error) == 'Refusing to create an inheritance cycle'
assert get_cache_token() == old
assert issubclass(ConcreteChild, Root)
# ---
# case: class descriptors and subclass overrides run through the VM
from _abc import _abc_init, _abc_instancecheck
seen = []
class Truth:
    def __bool__(self):
        seen.append('truth')
        return False
class Reported:
    pass
class Actual:
    @property
    def __class__(self):
        seen.append('class')
        return Reported
class Base:
    @classmethod
    def __subclasscheck__(cls, other):
        seen.append(other)
        if other is Reported:
            return Truth()
        return 'actual result'
_abc_init(Base)
assert _abc_instancecheck(Base, Actual()) == 'actual result'
assert seen == ['class', Reported, 'truth', Actual]
class Failure:
    @property
    def __class__(self):
        raise LookupError('class lookup')
try:
    _abc_instancecheck(Base, Failure())
    assert False
except LookupError as error:
    assert str(error) == 'class lookup'
# ---
# case: subclass enumeration checks return type and observes list mutation
from _abc import _abc_init, _abc_subclasscheck, _reset_caches
class Base:
    @classmethod
    def __subclasses__(cls):
        return ()
_abc_init(Base)
try:
    _abc_subclasscheck(Base, int)
    assert False
except TypeError as error:
    assert str(error) == '__subclasses__() must return a list'
children = []
class Meta(type):
    def __subclasscheck__(cls, other):
        children.pop()
        children.pop()
        return False
class First(metaclass=Meta):
    pass
children.append(First)
children.append(str)
class Mutable:
    @classmethod
    def __subclasses__(cls):
        return children
_abc_init(Mutable)
assert not _abc_subclasscheck(Mutable, int)
assert children == []
# ---
# case: reinitializing an ABC replaces only its own registry and caches
from _abc import _abc_init, _abc_register, _abc_subclasscheck
class Parent:
    pass
_abc_init(Parent)
_abc_register(Parent, int)
class Child(Parent):
    pass
assert Child._abc_impl is Parent._abc_impl
_abc_init(Child)
assert Child._abc_impl is not Parent._abc_impl
assert not _abc_subclasscheck(Child, int)
assert _abc_subclasscheck(Parent, int)
old = Parent._abc_impl
_abc_init(Parent)
assert Parent._abc_impl is not old
assert not _abc_subclasscheck(Parent, int)
# ---
# case: default subclass hooks decline and support classmethod super
from _abc import _abc_init, _abc_subclasscheck
class Base:
    @classmethod
    def __subclasshook__(cls, candidate):
        return super().__subclasshook__(candidate)
_abc_init(Base)
assert Base.__subclasshook__(int) is NotImplemented
assert object.__subclasshook__(int) is NotImplemented
assert _abc_subclasscheck(Base, Base)
assert not _abc_subclasscheck(Base, int)
# ---
# case: class identity reads expose actual classes
class C:
    pass
instance = C()
assert instance.__class__ is C
assert C.__class__ is type
assert int.__class__ is type
assert (1).__class__ is int
assert 'text'.__class__ is str
