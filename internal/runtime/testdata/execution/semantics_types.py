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
# ---
# case: isinstance native values and classes
list_type = type([])
assert isinstance(None, type(None))
assert isinstance(True, bool)
assert isinstance(True, int)
assert not isinstance(1, bool)
assert isinstance(1, int)
assert isinstance('text', str)
assert isinstance([], list_type)
assert not isinstance((), list_type)
assert isinstance(type, type)
assert isinstance(bool, type)
assert isinstance(ValueError, type)
assert not isinstance(1, type)
# ---
# case: isinstance user and exception ancestry
class InstanceBase:
    pass

class InstanceChild(InstanceBase):
    pass

class InstanceOther:
    pass

class ParentError(ValueError):
    pass

class ChildError(ParentError):
    pass

base = InstanceBase()
child = InstanceChild()
assert isinstance(base, InstanceBase)
assert isinstance(child, InstanceChild)
assert isinstance(child, InstanceBase)
assert not isinstance(base, InstanceChild)
assert not isinstance(child, InstanceOther)
assert isinstance(InstanceBase, type)
assert isinstance(ValueError('bad'), Exception)
assert isinstance(ParentError('bad'), ValueError)
assert isinstance(ChildError('bad'), ParentError)
assert isinstance(ChildError('bad'), ValueError)
assert not isinstance(ValueError('bad'), ParentError)
# ---
# case: isinstance tuple candidates
class TupleBase:
    pass

class TupleChild(TupleBase):
    pass

value = TupleChild()
assert isinstance(value, (str, TupleBase))
assert isinstance(value, (str, (int, TupleBase)))
assert not isinstance(value, (str, int))
assert isinstance(1, (int, None))
# ---
# case: issubclass native and user classes
class SubclassBase:
    pass

class SubclassMiddle(SubclassBase):
    pass

class SubclassLeaf(SubclassMiddle):
    pass

class SubclassOther:
    pass

assert issubclass(bool, bool)
assert issubclass(bool, int)
assert not issubclass(int, bool)
assert issubclass(int, int)
assert issubclass(type, type)
assert issubclass(SubclassLeaf, SubclassLeaf)
assert issubclass(SubclassLeaf, SubclassMiddle)
assert issubclass(SubclassLeaf, SubclassBase)
assert not issubclass(SubclassBase, SubclassLeaf)
assert not issubclass(SubclassLeaf, SubclassOther)
# ---
# case: issubclass exception ancestry
class ParentSubclassError(ValueError):
    pass

class ChildSubclassError(ParentSubclassError):
    pass

assert issubclass(ValueError, Exception)
assert issubclass(ParentSubclassError, ValueError)
assert issubclass(ChildSubclassError, ParentSubclassError)
assert issubclass(ChildSubclassError, Exception)
assert not issubclass(ValueError, ParentSubclassError)
# ---
# case: issubclass tuple candidates
class CandidateBase:
    pass

class CandidateChild(CandidateBase):
    pass

assert issubclass(CandidateChild, (str, CandidateBase))
assert issubclass(CandidateChild, (str, (int, CandidateBase)))
assert not issubclass(CandidateChild, (str, int))
assert issubclass(bool, (int, None))
# ---
# case: three argument type construction
marker = []

def read_value(self):
    return self.value

Dynamic = type('Dynamic', (), {'value': marker, 'read_value': read_value})
instance = Dynamic()
assert Dynamic.__name__ == 'Dynamic'
assert Dynamic.__qualname__ == 'Dynamic'
assert Dynamic.__module__ == __name__
assert type(Dynamic) is type
assert type(instance) is Dynamic
assert instance.value is marker
assert instance.read_value() is marker
# ---
# case: dynamic type inheritance and metadata
class DynamicBase:
    def inherited(self):
        return 'base'

DynamicChild = type(
    'DynamicChild',
    (DynamicBase,),
    {'own': lambda self: 'child', '__module__': 'remote', '__qualname__': 'Outer.DynamicChild'},
)
child = DynamicChild()
assert issubclass(DynamicChild, DynamicBase)
assert isinstance(child, DynamicBase)
assert child.inherited() == 'base'
assert child.own() == 'child'
assert DynamicChild.__module__ == 'remote'
assert DynamicChild.__qualname__ == 'Outer.DynamicChild'
# ---
# case: dynamic exception type
DynamicError = type('DynamicError', (ValueError,), {})
try:
    raise DynamicError('dynamic failure')
except ValueError as caught:
    assert type(caught) is DynamicError
    assert str(caught) == 'dynamic failure'
# ---
# case: list and tuple constructors
empty_tuple = tuple()
empty_list = list()
original_tuple = (1, 2)
original_list = [1, 2]
assert empty_tuple == ()
assert type(empty_list) is list
assert len(empty_list) == 0
assert tuple(original_tuple) is original_tuple
assert tuple(original_list) == (1, 2)
list_from_tuple = list(original_tuple)
assert len(list_from_tuple) == 2
assert list_from_tuple[0] == 1
assert list_from_tuple[1] == 2
list_copy = list(original_list)
assert len(list_copy) == 2
assert list_copy[0] == 1
assert list_copy[1] == 2
assert list_copy is not original_list
text_list = list('ab')
assert len(text_list) == 2
assert text_list[0] == 'a'
assert text_list[1] == 'b'
assert tuple(b'ab') == (97, 98)
assert type(empty_tuple) is tuple
assert type(empty_list) is list
# ---
# case: sequence constructors use iterator protocol
class CountingIterator:
    def __init__(self, stop):
        self.current = 0
        self.stop = stop

    def __iter__(self):
        return self

    def __next__(self):
        if self.current == self.stop:
            raise StopIteration
        value = self.current
        self.current += 1
        return value

counting_list = list(CountingIterator(3))
assert len(counting_list) == 3
assert counting_list[0] == 0
assert counting_list[1] == 1
assert counting_list[2] == 2
assert tuple(CountingIterator(3)) == (0, 1, 2)
# ---
# case: sequence constructors consume generators
def generated():
    yield 4
    yield 5
    yield 6

generated_list = list(generated())
assert len(generated_list) == 3
assert generated_list[0] == 4
assert generated_list[1] == 5
assert generated_list[2] == 6
assert tuple(value * 2 for value in generated()) == (8, 10, 12)
