# case: native type identity and metadata
assert type(None) is type(None)
assert type(None).__name__ == 'NoneType'
assert type(True) is bool
assert type(1) is int
assert type('text') is str
assert type([]) is type([])
assert type([]).__name__ == 'list'
assert type(type([])) is type
assert repr(type([])) == "<class 'list'>"
assert type(()) is type(())
assert type({}) is type({})
assert type({1}) is type({1})
assert type(lambda: None).__name__ == 'function'
assert type(len).__name__ == 'builtin_function_or_method'
assert bool.__name__ == 'bool'
assert bool.__qualname__ == 'bool'
assert bool.__module__ == 'builtins'
assert type.__name__ == 'type'
assert type.__module__ == 'builtins'
assert type(type) is type
assert type(bool) is type
assert type(int) is type
assert type(str) is type
# ---
# case: user and exception type identity
class Base:
    pass

class Child(Base):
    pass

class CustomError(ValueError):
    pass

base = Base()
child = Child()
assert type(base) is Base
assert type(base).__name__ == 'Base'
assert Base.__name__ == 'Base'
assert Base.__qualname__ == 'Base'
assert type(child) is Child
assert Child.__name__ == 'Child'
assert type(Base) is type
assert type(Child) is type
assert type(ValueError) is type
assert ValueError.__name__ == 'ValueError'
assert ValueError.__qualname__ == 'ValueError'
assert ValueError.__module__ == 'builtins'
assert type(ValueError('bad')) is ValueError
assert type(ValueError('bad')).__name__ == 'ValueError'
assert type(CustomError) is type
assert type(CustomError('bad')) is CustomError
# ---
# case: existing scalar type constructors
assert type(bool()) is bool
assert type(bool(1)) is bool
assert type(int(3.5)) is int
assert type(str(42)) is str
