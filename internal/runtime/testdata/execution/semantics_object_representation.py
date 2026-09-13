# case: root string and representation methods are real slot wrappers
value = object()
assert object.__str__.__objclass__ is object
assert type(object.__str__).__name__ == 'wrapper_descriptor'
assert type(value.__str__).__name__ == 'method-wrapper'
assert value.__str__.__self__ is value
assert value.__str__() == str(value) == repr(value)
assert object.__repr__(value) == repr(value)
assert object.__str__(1) == '1'
assert object.__str__('text') == "'text'"
assert object.__repr__(1) != '1'
assert 'int object' in object.__repr__(1)
assert object.__str__.__get__(value)() == str(value)
for method in [object.__str__, object.__repr__]:
    for args, kw in [((), {}), ((value, 1), {}), ((value,), {'extra': 1})]:
        try:
            method(*args, **kw)
            assert False
        except TypeError:
            pass

# ---
# case: object string dispatch uses class repr and ignores str or instance overrides
calls = []
class Custom:
    def __repr__(self):
        calls.append('repr')
        return 'custom repr'
    def __str__(self):
        calls.append('str')
        return 'custom str'
value = Custom()
value.__repr__ = lambda: 'instance repr'
assert object.__str__(value) == 'custom repr'
assert calls == ['repr']
assert str(value) == 'custom str'
assert calls == ['repr', 'str']
assert object.__repr__(value) != 'custom repr'
assert 'Custom object' in object.__repr__(value)
class Inherited:
    def __repr__(self):
        return 'inherited repr'
assert Inherited.__str__ is object.__str__
assert Inherited().__str__() == 'inherited repr'
assert str(Inherited()) == 'inherited repr'
class Super(Inherited):
    def __str__(self):
        return f'super: {super().__str__()}'
assert str(Super()) == 'super: inherited repr'
class Default:
    __repr__ = object.__repr__
    __str__ = object.__str__
assert 'Default object' in repr(Default())

# ---
# case: direct object string slot returns raw repr result while str validates
class Bad:
    def __repr__(self):
        return 42
assert object.__str__(Bad()) == 42
assert Bad().__str__() == 42
try:
    str(Bad())
    assert False
except TypeError as error:
    assert str(error) == '__str__ returned non-string (type int)'
class Broken:
    def __repr__(self):
        raise ValueError('repr failed')
try:
    object.__str__(Broken())
    assert False
except ValueError as error:
    assert str(error) == 'repr failed'
