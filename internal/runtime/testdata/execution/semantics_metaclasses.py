# case: type subclass creates classes with stable metaclass identity
class Meta(type):
    pass
class Base(metaclass=Meta):
    value = 7
class Child(Base):
    pass
assert type(Base) is Meta
assert type(Child) is Meta
assert isinstance(Base, Meta)
assert issubclass(Meta, type)
assert Base().value == 7
assert type(Meta('Dynamic', (), {'answer': 42})) is Meta
# ---
# case: metaclass new and init order and class cell
calls = []
class Meta(type):
    def __new__(mcls, name, bases, namespace):
        calls.append(('new', name))
        namespace['injected'] = 17
        return super().__new__(mcls, name, bases, namespace)
    def __init__(cls, name, bases, namespace):
        calls.append(('init', cls.__name__))
        super().__init__(name, bases, namespace)
class Base(metaclass=Meta):
    def owner(self):
        return __class__
assert calls == [('new', 'Base'), ('init', 'Base')]
assert Base.injected == 17
assert Base().owner() is Base
assert type(Base) is Meta
# ---
# case: metaclass conflict occurs before class body
class Left(type):
    pass
class Right(type):
    pass
class A(metaclass=Left):
    pass
class B(metaclass=Right):
    pass
events = []
try:
    class C(A, B):
        events.append('body')
except TypeError as error:
    assert str(error) == 'metaclass conflict: the metaclass of a derived class must be a (non-strict) subclass of the metaclasses of all its bases'
else:
    assert False
assert events == []
# ---
# case: most derived metaclass wins
class Meta(type):
    pass
class Derived(Meta):
    pass
class A(metaclass=Meta):
    pass
class B(metaclass=Derived):
    pass
class C(A, B, metaclass=type):
    pass
assert type(C) is Derived
# ---
# case: metaclass exceptions are catchable at class statement
class Meta(type):
    def __new__(mcls, name, bases, namespace):
        raise ValueError('construction failed')
try:
    class C(metaclass=Meta):
        pass
except ValueError as error:
    assert str(error) == 'construction failed'
else:
    assert False
assert 'C' not in dir()
class Good:
    pass
assert isinstance(Good(), Good)
# ---
# case: metaclass new may return an unrelated value
calls = []
class Meta(type):
    def __new__(mcls, name, bases, namespace):
        return 42
    def __init__(cls, name, bases, namespace):
        calls.append('init')
class Value(metaclass=Meta):
    pass
assert Value == 42
assert calls == []
# ---
# case: function metaclass receives body namespace
seen = []
def factory(name, bases, namespace, flag):
    seen.append((name, bases, namespace['value'], flag))
    return 99
class Value(metaclass=factory, flag=7):
    value = 8
assert Value == 99
assert seen == [('Value', (), 8, 7)]
# ---
# case: prepare runs before body and preserves namespace identity
seen = []
prepared = None
class Meta(type):
    @classmethod
    def __prepare__(mcls, name, bases, flag):
        global prepared
        seen.append(('prepare', mcls, flag))
        prepared = {'seed': 10}
        return prepared
    def __new__(mcls, name, bases, namespace, flag):
        assert namespace is prepared
        assert namespace['value'] == 11
        seen.append(('new', flag))
        return type.__new__(mcls, name, bases, namespace)
class C(metaclass=Meta, flag=3):
    seen.append('body')
    value = seed + 1
assert C.value == 11
assert seen == [('prepare', Meta, 3), 'body', ('new', 3)]
assert prepared['value'] == 11
# ---
# case: prepare errors skip body and leave interpreter usable
class Meta(type):
    @classmethod
    def __prepare__(mcls, name, bases):
        raise RuntimeError('prepare failed')
seen = []
try:
    class C(metaclass=Meta):
        seen.append('body')
except RuntimeError as error:
    assert str(error) == 'prepare failed'
else:
    assert False
assert seen == []
class Good:
    pass
assert isinstance(Good(), Good)
# ---
# case: bad initializer result and dropped class cell
class Meta(type):
    def __init__(cls, name, bases, namespace):
        return 7
try:
    class Bad(metaclass=Meta):
        pass
except TypeError as error:
    assert str(error) == "__init__() should return None, not 'int'"
else:
    assert False
class Dropping(type):
    def __new__(mcls, name, bases, namespace):
        namespace.pop('__classcell__')
        return type.__new__(mcls, name, bases, namespace)
try:
    class Bad(metaclass=Dropping):
        def owner(self):
            return __class__
except RuntimeError:
    pass
else:
    assert False
# ---
# case: dynamic type respects inherited metaclass
calls = []
class Meta(type):
    def __new__(mcls, name, bases, namespace):
        calls.append(name)
        return type.__new__(mcls, name, bases, namespace)
class Base(metaclass=Meta):
    pass
Child = type('Child', (Base,), {})
assert type(Child) is Meta
assert calls == ['Base', 'Child']
# ---
# case: prepared dictionary mutations are visible during class body
prepared = None
class Meta(type):
    @classmethod
    def __prepare__(mcls, name, bases):
        global prepared
        prepared = {'seed': 1}
        return prepared
class C(metaclass=Meta):
    prepared['seed'] = 5
    answer = seed
    del seed
assert C.answer == 5
assert 'seed' not in prepared
# ---
# case: class cell is consumed by type new
class C:
    def owner(self):
        return __class__
assert C().owner() is C
assert not hasattr(C, '__classcell__')
