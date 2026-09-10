# case: native collections produce aliases with stable type and argument metadata
Alias = type(list[int])
assert Alias.__name__ == 'GenericAlias'
assert Alias.__module__ == 'types'
for origin in [list, tuple, dict, set, frozenset, type]:
    alias = origin[int, str]
    assert type(alias) is Alias
    assert alias.__origin__ is origin
    assert alias.__args__ == (int, str)
    assert alias.__args__ is alias.__args__
    assert origin.__class_getitem__(int).__args__ == (int,) if origin is not type else True
assert repr(list[int]) == 'list[int]'
assert repr(dict[str, list[int]]) == 'dict[str, list[int]]'
assert repr(tuple[()]) == 'tuple[()]'
assert repr(tuple[int, ...]) == 'tuple[int, ...]'

# ---
# case: direct generic alias construction preserves origin and arguments
Alias = type(list[int])
class Origin:
    pass
argument = [1]
alias = Alias(Origin, argument)
assert alias.__origin__ is Origin
assert alias.__args__[0] is argument
assert alias.__mro_entries__((alias,)) == (Origin,)
argument.append(2)
assert alias.__args__ == ([1, 2],)
for name in ['__origin__', '__args__']:
    try:
        setattr(alias, name, None)
        assert False
    except AttributeError:
        pass

# ---
# case: generic alias constructors reject invalid argument shapes
Alias = type(list[int])
for args in [(), (list,), (list, int, str)]:
    try:
        Alias(*args)
        assert False
    except TypeError:
        pass
try:
    Alias(origin=list, args=int)
    assert False
except TypeError:
    pass
for cls in [str, int, bytes]:
    try:
        cls[int]
        assert False
    except TypeError:
        pass
