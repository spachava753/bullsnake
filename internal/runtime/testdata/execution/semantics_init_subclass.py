# case: subclass hooks run after naming and before metaclass initialization
calls = []
class Named:
    def __set_name__(self, owner, name):
        calls.append(('name', owner, name))
class Meta(type):
    def __new__(meta, name, bases, namespace, **kwargs):
        calls.append(('new before', name))
        result = super().__new__(meta, name, bases, namespace, **kwargs)
        calls.append(('new after', name))
        return result
    def __init__(cls, name, bases, namespace, **kwargs):
        calls.append(('init', name))
        super().__init__(name, bases, namespace, **kwargs)
class Base(metaclass=Meta):
    def __init_subclass__(cls, *, label, **kwargs):
        calls.append(('subclass', cls, label))
        assert cls.check() is cls
        cls.label = label
        super().__init_subclass__(**kwargs)
calls = []
class Child(Base, label='named'):
    item = Named()
    @staticmethod
    def check():
        return __class__
assert calls == [('new before', 'Child'), ('name', Child, 'item'), ('subclass', Child, 'named'), ('new after', 'Child'), ('init', 'Child')]
assert Child.label == 'named'
assert type(Base.__dict__['__init_subclass__']) is classmethod

# ---
# case: dynamic type forwards class keywords to the selected metaclass and base
calls = []
class Meta(type):
    def __new__(meta, name, bases, namespace, *, selected=False, **kwargs):
        calls.append(('selected', selected))
        return super().__new__(meta, name, bases, namespace, **kwargs)
class Base(metaclass=Meta):
    def __init_subclass__(cls, *, label, **kwargs):
        calls.append(('base', label))
        super().__init_subclass__(**kwargs)
calls = []
Child = type('Child', (Base,), {}, selected=True, label='dynamic')
assert type(Child) is Meta
assert calls == [('selected', True), ('base', 'dynamic')]

# ---
# case: cooperative subclass hooks use the new class C3 order
calls = []
class Left:
    def __init_subclass__(cls, *, left, **kwargs):
        calls.append(('left', cls, left))
        super().__init_subclass__(**kwargs)
class Right:
    def __init_subclass__(cls, *, right, **kwargs):
        calls.append(('right', cls, right))
        super().__init_subclass__(**kwargs)
class Combined(Left, Right, left=1, right=2):
    pass
assert calls == [('left', Combined, 1), ('right', Combined, 2)]
class Static:
    @staticmethod
    def __init_subclass__(*, label):
        calls.append(('static', label))
class Child(Static, label=3):
    pass
assert calls[-1] == ('static', 3)

# ---
# case: root subclass hooks reject leftover keywords and failures are catchable
assert object.__init_subclass__() is None
class Plain:
    pass
assert Plain.__init_subclass__() is None
assert Plain().__init_subclass__() is None
assert Plain.__init_subclass__.__self__ is Plain
assert type(object.__dict__['__init_subclass__']).__name__ == 'classmethod_descriptor'
for call in (
    lambda: type('Bad', (), {}, unknown=1),
    lambda: object.__init_subclass__(unknown=1),
    lambda: object.__init_subclass__(1),
):
    try:
        call()
    except TypeError:
        pass
    else:
        assert False
failure = ValueError('subclass failed')
class Base:
    def __init_subclass__(cls):
        raise failure
try:
    class Child(Base):
        pass
except ValueError as error:
    assert error is failure
else:
    assert False
assert 'Child' not in globals()
