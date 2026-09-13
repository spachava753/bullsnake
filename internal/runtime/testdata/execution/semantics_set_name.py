# case: descriptor naming runs during type construction in namespace order
calls = []
class Named:
    def __set_name__(self, owner, name):
        calls.append((owner, name))
        self.owner = owner
        self.name = name
        return 99
first = Named()
second = Named()
class Owner:
    a = first
    b = second
assert calls == [(Owner, 'a'), (Owner, 'b')]
assert first.owner is Owner and second.name == 'b'
class Child(Owner):
    pass
assert calls == [(Owner, 'a'), (Owner, 'b')]
Owner.late = Named()
assert len(calls) == 2
Dynamic = type('Dynamic', (), {'x': Named()})
assert calls[-1] == (Dynamic, 'x')

# ---
# case: naming hooks use a snapshot while retaining real class mutations
calls = []
class Named:
    def __init__(self, label):
        self.label = label
    def __set_name__(self, owner, name):
        calls.append((self.label, name))
        if name == 'a':
            del owner.b
            owner.c = Named('new')
            owner.a = 'replaced'
class Owner:
    a = Named('first')
    b = Named('second')
assert calls == [('first', 'a'), ('second', 'b')]
assert Owner.a == 'replaced'
assert not hasattr(Owner, 'b')
assert Owner.c.label == 'new'

# ---
# case: naming completes before a metaclass new returns and ignores instance hooks
calls = []
class Named:
    def __set_name__(self, owner, name):
        calls.append('name')
        assert owner.check() is owner
value = Named()
value.__set_name__ = lambda *args: calls.append('instance')
class Meta(type):
    def __new__(meta, name, bases, namespace):
        calls.append('before')
        result = super().__new__(meta, name, bases, namespace)
        calls.append('after')
        return result
class Owner(metaclass=Meta):
    item = value
    @staticmethod
    def check():
        return __class__
assert calls == ['before', 'name', 'after']

# ---
# case: naming callback exceptions preserve identity and receive the CPython note
failure = ValueError('naming failed')
seen = []
class Named:
    def __set_name__(self, owner, name):
        seen.append(owner)
        raise failure
try:
    class Owner:
        item = Named()
except ValueError as error:
    assert error is failure
    assert str(error) == 'naming failed'
    assert error.__notes__ == ["Error calling __set_name__ on 'Named' instance 'item' in 'Owner'"]
else:
    assert False
assert seen[0].__name__ == 'Owner'
assert 'Owner' not in globals()

# ---
# case: naming resolves descriptor-valued class hooks before calling them
calls = []
class Hook:
    def __get__(self, receiver, cls):
        calls.append(('bind', receiver, cls))
        def name(owner, key):
            calls.append(('name', owner, key))
        return name
class Named:
    __set_name__ = Hook()
value = Named()
class Owner:
    item = value
assert calls == [('bind', value, Named), ('name', Owner, 'item')]
failure = ValueError('binding failed')
class Broken:
    def __get__(self, receiver, cls):
        raise failure
class BadName:
    __set_name__ = Broken()
try:
    type('Failed', (), {'x': BadName()})
except ValueError as error:
    assert error is failure
    assert not hasattr(error, '__notes__')
else:
    assert False

# ---
# case: property naming has real native behavior and subclass hooks
calls = []
class NamedProperty(property):
    def __set_name__(self, owner, name):
        calls.append(name)
        super().__set_name__(owner, name)
class Owner:
    @NamedProperty
    def value(self):
        return 42
assert Owner().value == 42
assert calls == ['value']
assert Owner.value.__name__ == 'value'
p = property(lambda self: 1)
property.__set_name__(p, None, 42)
assert p.__name__ == 42
property.__set_name__(p, None, 'renamed')
assert p.__name__ == 'renamed'
assert type(property.__set_name__).__name__ == 'method_descriptor'
