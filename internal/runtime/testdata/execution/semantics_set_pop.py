# case: native set pop removes one existing item and preserves its identity
value = ('a', 1)
items = {value}
assert items.pop() is value
assert len(items) == 0
items = {1, 2, 3}
removed = set()
while items:
    value = set.pop(items)
    assert value not in items
    assert value not in removed
    removed.add(value)
assert removed == {1, 2, 3}
assert type(set.pop).__name__ == 'method_descriptor'
assert set.pop.__objclass__ is set
assert set.pop.__get__({7}, set)() == 7
assert not hasattr(frozenset(), 'pop')

# ---
# case: set pop validates calls and invalidates active iterators
items = {1, 2}
iterator = iter(items)
items.pop()
try:
    next(iterator)
except RuntimeError as error:
    assert str(error) == 'Set changed size during iteration'
else:
    assert False
try:
    set().pop()
except KeyError as error:
    assert error.args == ('pop from an empty set',)
else:
    assert False
for call in (
    lambda: set.pop(),
    lambda: set.pop(frozenset({1})),
    lambda: set.pop({1}, 2),
    lambda: set.pop({1}, default=2),
):
    try:
        call()
    except TypeError:
        pass
    else:
        assert False
