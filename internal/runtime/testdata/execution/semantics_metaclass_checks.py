# case: metaclass checks run through Python methods
calls = []
class Meta(type):
    def __instancecheck__(cls, instance):
        calls.append(('instance', instance))
        return instance == 7
    def __subclasscheck__(cls, subclass):
        calls.append(('subclass', subclass))
        return subclass is str
class C(metaclass=Meta):
    pass
assert isinstance(7, C)
assert not isinstance(8, C)
assert issubclass(str, C)
assert not issubclass(int, C)
assert calls == [('instance', 7), ('instance', 8), ('subclass', str), ('subclass', int)]
# ---
# case: check truth callbacks and tuple short circuit
seen = []
class Truth:
    def __bool__(self):
        seen.append('truth')
        return True
class Meta(type):
    def __instancecheck__(cls, instance):
        return Truth()
    def __subclasscheck__(cls, subclass):
        return Truth()
class C(metaclass=Meta):
    pass
assert isinstance(1, (str, (C, 42)))
assert issubclass(int, (str, (C, 42)))
assert seen == ['truth', 'truth']
# ---
# case: instance identity bypass and subclass override
seen = []
class Meta(type):
    def __instancecheck__(cls, instance):
        seen.append('instance')
        return False
    def __subclasscheck__(cls, subclass):
        seen.append('subclass')
        return False
class C(metaclass=Meta):
    pass
assert isinstance(C(), C)
assert not issubclass(C, C)
assert seen == ['subclass']
# ---
# case: native checks through metaclass super
class Meta(type):
    def __instancecheck__(cls, instance):
        return super().__instancecheck__(instance)
    def __subclasscheck__(cls, subclass):
        return super().__subclasscheck__(subclass)
class Base(metaclass=Meta):
    pass
class Child(Base):
    pass
assert isinstance(Child(), Base)
assert issubclass(Child, Base)
assert not isinstance(1, Base)
assert not issubclass(int, Base)
assert type.__instancecheck__(Base, Child())
assert type.__subclasscheck__(Base, Child)
# ---
# case: subclass hook receives otherwise invalid first argument
class Meta(type):
    def __subclasscheck__(cls, candidate):
        return candidate == 7
class C(metaclass=Meta):
    pass
assert issubclass(7, C)
# ---
# case: hook and truth failures propagate normally
class Meta(type):
    def __instancecheck__(cls, candidate):
        raise ValueError('check failed')
class C(metaclass=Meta):
    pass
try:
    isinstance(1, C)
except ValueError as error:
    assert str(error) == 'check failed'
else:
    assert False
class Truth:
    def __bool__(self):
        raise LookupError('truth failed')
class Meta(type):
    def __subclasscheck__(cls, candidate):
        return Truth()
class C(metaclass=Meta):
    pass
try:
    issubclass(int, C)
except LookupError as error:
    assert str(error) == 'truth failed'
else:
    assert False
assert isinstance(1, int)
# ---
# case: empty tuple skips subclass first-argument validation
assert not issubclass(7, ())
assert not isinstance(7, ())
# ---
# case: candidate class attributes do not override metaclass checks
class Meta(type):
    def __instancecheck__(cls, instance):
        return True
    def __subclasscheck__(cls, candidate):
        return True
class C(metaclass=Meta):
    __instancecheck__ = None
    __subclasscheck__ = None
assert isinstance(7, C)
assert issubclass(int, C)
