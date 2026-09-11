# case: generic alias parameters preserve first occurrence and cached identity
def parameters[T, U, *Ts, **P]():
    alias = dict[T, tuple[U, T, Ts, P]]
    assert alias.__parameters__ == (T, U, Ts, P)
    assert alias.__parameters__ is alias.__parameters__
    assert list[int].__parameters__ == ()
    nested = [T, (U, T)]
    alias = list[nested]
    assert alias.__parameters__ == (T, U)
    nested.append(P)
    assert alias.__parameters__ == (T, U)
parameters()

# ---
# case: parameter discovery resumes descriptors ignores bare classes and retries
calls = []
class Parameter:
    @property
    def __typing_subst__(self):
        calls.append('subst')
        return None
class Nested:
    @property
    def __parameters__(self):
        calls.append('parameters')
        return (p, p)
p = Parameter()
assert list[(p, Nested())].__parameters__ == (p,)
assert calls == ['subst', 'parameters']
class Bare:
    __typing_subst__ = None
    __parameters__ = (p,)
assert list[Bare].__parameters__ == ()
class Broken:
    @property
    def __parameters__(self):
        calls.append('broken')
        raise ValueError('retry discovery')
alias = list[Broken()]
for i in range(2):
    try:
        alias.__parameters__
        assert False
    except ValueError as error:
        assert str(error) == 'retry discovery'
assert calls == ['subst', 'parameters', 'broken', 'broken']

# ---
# case: parameter discovery rejects recursive lists without native recursion
items = []
items.append(items)
try:
    list[items].__parameters__
    assert False
except RecursionError:
    pass
