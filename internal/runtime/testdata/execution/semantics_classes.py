# Runtime execution cases for classes.
# case: multiple inheritance follows C3 resolution order
class Root:
    label = 'root'
class Left(Root):
    label = 'left'
class Right(Root):
    label = 'right'
class LeftFirst(Left, Right):
    pass
class RightFirst(Right, Left):
    pass
class ListSubclass(list):
    pass
assert LeftFirst.label == 'left'
assert RightFirst.label == 'right'
assert LeftFirst.__mro__ == (LeftFirst, Left, Right, Root)
# ---
# case: built-in generic aliases expose stable runtime types
list_alias = list[int]
assert type(list_alias).__name__ == 'GenericAlias'
assert list_alias.__origin__ is list
assert list_alias.__args__ == (int,)
assert type(lambda: None).__name__ == 'function'
assert type(...).__name__ == 'ellipsis'
# ---
# case: metaclass methods bind to constructed classes
class MarkerMeta(type):
    def identify(cls):
        return cls
class Marked(metaclass=MarkerMeta):
    pass
class InheritedMeta(Marked):
    pass
assert Marked.identify() is Marked
assert InheritedMeta.identify() is InheritedMeta
# ---
# case: built-in descriptors bind functions
class Described:
    factor = 3
    def __init__(self, value):
        self.value = value
    @classmethod
    def owner(cls):
        return cls
    @staticmethod
    def add(left, right):
        return left + right
    @property
    def scaled(self):
        return self.value * self.factor
assert Described.owner() is Described
assert Described(4).owner() is Described
assert Described.add(2, 5) == 7
assert Described(4).scaled == 12
class DescriptorSubclass(classmethod):
    pass
def marked():
    pass
marked.flag = 42
assert marked.flag == 42
# ---
# case: class private names are mangled consistently
class Vault:
    __kind = 'secret'
    def __init__(self, value):
        self.__value = value
    def read(self):
        return self.__value
    def __private(self):
        return self.__value + 1
    def call_private(self):
        return self.__private()
vault = Vault(7)
assert vault.read() == 7
assert vault.call_private() == 8
assert Vault._Vault__kind == 'secret'
assert vault._Vault__value == 7
# ---
# case: constructed class definitions
# module: classes
marker = 0
class Empty:
    global marker
    marker = 41
    value = 42
    def method(self):
        return 7
created = Empty
same = created is Empty
observed_marker = marker
def preserve(cls):
    return cls
@preserve
class Decorated:
    pass
decorated = Decorated
def make(value):
    class Inner:
        global marker
        marker = value
    return Inner
nested = make(43)
nested_marker = marker
assert f'{created!r}' == "<class 'classes.Empty'>", "created"
assert f'{same!r}' == "True", "same"
assert f'{observed_marker!r}' == "41", "observed_marker"
assert f'{decorated!r}' == "<class 'classes.Decorated'>", "decorated"
assert f'{nested!r}' == "<class 'classes.make.<locals>.Inner'>", "nested"
assert f'{nested_marker!r}' == "43", "nested_marker"
# ---
# case: type attribute reads
# module: attributes
class Config:
    value = 42
    def add(self, amount):
        return __class__.value + amount
    def owner(self):
        return __class__
loaded = Config.value
called = Config.add(None, 8)
owner = Config.owner(None)
def make(offset):
    class Inner:
        value = offset
    return Inner
nested_value = make(9).value
assert f'{loaded!r}' == "42", "loaded"
assert f'{called!r}' == "50", "called"
assert f'{owner!r}' == "<class 'attributes.Config'>", "owner"
assert f'{nested_value!r}' == "9", "nested_value"
# ---
# case: instance method binding
# module: instances
class Counter:
    value = 40
    def add(self, amount):
        return self.value + amount
    def owner(self):
        return __class__
first = Counter()
second = Counter()
class_value = first.value
called = first.add(2)
owner = first.owner()
distinct = first is not second
instance = first
assert f'{class_value!r}' == "40", "class_value"
assert f'{called!r}' == "42", "called"
assert f'{owner!r}' == "<class 'instances.Counter'>", "owner"
assert f'{distinct!r}' == "True", "distinct"
assert f'{instance!r}' == "<instances.Counter object>", "instance"
# ---
# case: constructor state mutation
class Box:
    kind = 'box'
    def __init__(self, value=1):
        self.value = value
    def set(self, value):
        self.value = value
    def clear(self):
        del self.value
first = Box(10)
second = Box()
initial = first.value
defaulted = second.value
set_result = first.set(20)
updated = first.value
isolated = second.value
first.clear()
def raw(value):
    return value
first.callable = raw
stored_raw = first.callable is raw
raw_result = first.callable(42)
Box.kind = 'updated'
class_update = Box.kind
del Box.kind
assert f'{initial!r}' == "10", "initial"
assert f'{defaulted!r}' == "1", "defaulted"
assert f'{set_result!r}' == "None", "set_result"
assert f'{updated!r}' == "20", "updated"
assert f'{isolated!r}' == "1", "isolated"
assert f'{stored_raw!r}' == "True", "stored_raw"
assert f'{raw_result!r}' == "42", "raw_result"
assert f'{class_update!r}' == "'updated'", "class_update"
# ---
# case: single inheritance
class Base:
    value = 40
    def __init__(self, start):
        self.start = start
    def total(self, extra):
        return self.start + self.value + extra
class Child(Base):
    value = 1
class Override(Base):
    def total(self, extra):
        return 99
child = Child(10)
inherited_value = Child.value
inherited_method = child.total(2)
class_method = Child.total(child, 3)
initialized = child.start
overridden = Override(5).total(8)
assert f'{inherited_value!r}' == "1", "inherited_value"
assert f'{inherited_method!r}' == "13", "inherited_method"
assert f'{class_method!r}' == "14", "class_method"
assert f'{initialized!r}' == "10", "initialized"
assert f'{overridden!r}' == "99", "overridden"
