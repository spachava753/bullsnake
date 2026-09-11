# case: ordinary type variables substitute recursively and preserve origins
def check[T, U]():
    alias = dict[T, list[U]]
    result = alias[int, str]
    assert result == dict[int, list[str]]
    assert result.__parameters__ == ()
    partial = alias[int, U]
    assert partial.__parameters__ == (U,)
    assert partial[str] == result
    assert list[T][None] == list[type(None)]
    assert list[[T, (U, T)]][int, str].__args__ == ([int, (str, int)],)
    assert list[T][23].__args__ == (23,)
    for key in [(), (int, str)]:
        try:
            list[T][key]
            assert False
        except TypeError:
            pass
    try:
        list[int][str]
        assert False
    except TypeError:
        pass
check()

# ---
# case: type variable defaults are lazy and retry failed evaluation
calls = []
def default():
    calls.append('default')
    if len(calls) == 1:
        raise ValueError('retry default')
    return str
def check[T, U = default()]():
    alias = dict[T, U]
    assert alias[int, bytes] == dict[int, bytes]
    assert calls == []
    try:
        alias[int]
        assert False
    except ValueError as error:
        assert str(error) == 'retry default'
    assert alias[int] == dict[int, str]
    assert alias[int] == dict[int, str]
    assert calls == ['default', 'default']
check()

# ---
# case: generic substitution invokes custom typing preparation and replacement
calls = []
class Parameter:
    def __typing_prepare_subst__(self, alias, args):
        calls.append(('prepare', args))
        if not args:
            return (int,)
        return args
    def __typing_subst__(self, arg):
        calls.append(('subst', arg))
        return list[arg]
p = Parameter()
assert tuple[p][()].__args__ == (list[int],)
assert calls == [('prepare', ()), ('subst', int)]
class Nested:
    __parameters__ = (p,)
    def __getitem__(self, args):
        calls.append(('nested', args))
        return args
assert list[Nested()][str].__args__ == ((str,),)
assert calls[-2:] == [('prepare', (str,)), ('nested', (str,))]
class Broken(Parameter):
    def __typing_subst__(self, arg):
        raise ValueError('broken substitution')
try:
    list[Broken()][int]
    assert False
except ValueError as error:
    assert str(error) == 'broken substitution'
