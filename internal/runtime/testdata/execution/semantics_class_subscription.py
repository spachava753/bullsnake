# case: class subscription binds inherited class getitem automatically
class Base:
    def __class_getitem__(cls, key):
        return (cls, key)
class Child(Base):
    pass
assert Base[1] == (Base, 1)
assert Child[int, str] == (Child, (int, str))
assert Child.__class_getitem__(None) == (Child, None)
assert Base.__class_getitem__.__self__ is Base

# ---
# case: metaclass item access takes precedence over class getitem
class Meta(type):
    def __getitem__(cls, key):
        return ('meta', cls, key)
class C(metaclass=Meta):
    def __class_getitem__(cls, key):
        assert False
assert C['key'] == ('meta', C, 'key')
C.__getitem__ = lambda key: 'ignored'
assert C[0] == ('meta', C, 0)

# ---
# case: class subscription honors descriptor binding and runtime assignment
class Descriptor:
    def __get__(self, instance, owner):
        assert instance is None
        return lambda key: (owner, key)
class C:
    __class_getitem__ = Descriptor()
assert C[2] == (C, 2)
C.__class_getitem__ = lambda key: ('assigned', key)
assert C[3] == ('assigned', 3)
class Static:
    @staticmethod
    def __class_getitem__(key):
        return key
assert Static['x'] == 'x'

# ---
# case: missing and disabled class subscription raise TypeError
class Missing:
    pass
class Disabled:
    __class_getitem__ = None
for cls in [Missing, Disabled, int]:
    try:
        cls[1]
        assert False
    except TypeError:
        pass
class Meta(type):
    __getitem__ = None
class C(metaclass=Meta):
    def __class_getitem__(cls, key):
        assert False
try:
    C[1]
    assert False
except TypeError:
    pass

# ---
# case: class getitem exceptions propagate at the subscription
class C:
    def __class_getitem__(cls, key):
        raise LookupError('subscription failed')
try:
    C[1]
    assert False
except LookupError as error:
    assert str(error) == 'subscription failed'

# ---
# case: dynamic type construction also binds class getitem
Dynamic = type('Dynamic', (), {'__class_getitem__': lambda cls, key: (cls, key)})
assert Dynamic[4] == (Dynamic, 4)

# ---
# case: metaclass item descriptors receive the subscribed class
class Descriptor:
    def __get__(self, instance, owner):
        return lambda key: (instance, owner, key)
class Meta(type):
    __getitem__ = Descriptor()
class C(metaclass=Meta):
    pass
assert C[5] == (C, Meta, 5)
