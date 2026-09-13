# case: mixed integer bases preserve native positions in the C3 order
class Mixin:
    def __repr__(self):
        return 'mixin repr'
    def marker(self):
        return 'mixin'
class NativeFirst(int, Mixin):
    pass
class PythonFirst(Mixin, int):
    pass
assert NativeFirst.__bases__ == (int, Mixin)
assert NativeFirst.__base__ is int
assert NativeFirst.__mro__ == (NativeFirst, int, Mixin, object)
assert PythonFirst.__bases__ == (Mixin, int)
assert PythonFirst.__base__ is int
assert PythonFirst.__mro__ == (PythonFirst, Mixin, int, object)
assert NativeFirst(7) == 7 and PythonFirst(7) == 7
assert repr(NativeFirst(7)) == '7'
assert repr(PythonFirst(7)) == 'mixin repr'
assert NativeFirst(7).marker() == 'mixin'
assert NativeFirst.__repr__ is int.__repr__
assert PythonFirst.__repr__ is Mixin.__repr__
class Child(NativeFirst):
    pass
assert Child.__mro__ == (Child, NativeFirst, int, Mixin, object)
assert repr(Child(8)) == '8'

# ---
# case: mixed integer inheritance uses the most derived compatible layout base
class Number(int):
    def plus(self):
        return self + 1
class Mixin:
    pass
class Combined(Mixin, Number):
    pass
assert Combined.__bases__ == (Mixin, Number)
assert Combined.__base__ is Number
assert Combined.__mro__ == (Combined, Mixin, Number, int, object)
assert Combined(7).plus() == 8
class Other(int):
    pass
class Diamond(Number, Other):
    pass
assert Diamond.__mro__ == (Diamond, Number, Other, int, object)
assert Diamond.__base__ is Number
assert Diamond(7).plus() == 8
assert Diamond.__new__ is int.__new__

# ---
# case: super walks mixed native and Python positions without restarting native lookup
class Mixin:
    def __repr__(self):
        return 'mixin repr'
    def after(self):
        return super().__repr__()
class Number(int, Mixin):
    def __repr__(self):
        return super().__repr__()
value = Number(7)
assert repr(value) == '7'
assert value.after() == object.__repr__(value)
class PythonFirst(Mixin, int):
    def after(self):
        return super().after()
assert PythonFirst(9).after() == '9'

# ---
# case: mixed integer constructors obey MRO and metaclass selection
calls = []
class Meta(type):
    def __new__(meta, name, bases, namespace):
        calls.append((name, bases))
        return super().__new__(meta, name, bases, namespace)
class Mixin(metaclass=Meta):
    def __new__(cls, value):
        calls.append('mixin new')
        return super().__new__(cls, value + 1)
    def __init__(self, value):
        self.original = value
class Combined(Mixin, int):
    pass
assert type(Combined) is Meta
value = Combined(7)
assert value == 8 and value.original == 7
assert calls[-1] == 'mixin new'
Dynamic = type('Dynamic', (int, Mixin), {})
assert type(Dynamic) is Meta
assert Dynamic.__mro__ == (Dynamic, int, Mixin, object)
assert Dynamic(7) == 7

# ---
# case: inconsistent mixed MROs and unsupported native layouts remain rejected
class Number(int):
    pass
class Slotted:
    __slots__ = ('field',)
for bases in ((int, Number), (int, int), (int, dict), (int, Slotted)):
    try:
        type('Invalid', bases, {})
    except TypeError:
        pass
    else:
        assert False
