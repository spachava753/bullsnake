# case: sequence equality invokes element equality and truth in order
calls = []
class Truth:
    def __bool__(self):
        calls.append('truth')
        return True
class Item:
    def __eq__(self, other):
        calls.append('eq')
        return Truth()
first, second = Item(), Item()
assert [first] == [second]
assert (first,) == (second,)
assert calls == ['eq', 'truth', 'eq', 'truth']
calls = []
assert [first] == [first]
assert [first] != [second, second]
assert [first] != (first,)
assert calls == []
assert {'a': [(first,)]} == {'a': [(second,)]}
calls = []
assert (first,) != (second, second)
assert calls == ['eq', 'truth']

# ---
# case: list equality rereads lengths after element callbacks
left, right = [], []
class Shrink:
    def __eq__(self, other):
        left.pop()
        return True
left.extend([Shrink(), 1])
right.extend([0, 1])
assert left != right
assert len(left) == 1

# ---
# case: sequence inequality uses equality and preserves callback failures
class Equal:
    def __eq__(self, other):
        return True
    def __ne__(self, other):
        raise AssertionError('must use eq')
assert not [Equal()] != [Equal()]
assert not (Equal(),) != (Equal(),)
class Broken:
    def __eq__(self, other):
        raise ValueError('element failed')
try:
    [Broken()] == [1]
    assert False
except ValueError as error:
    assert str(error) == 'element failed'

# ---
# case: recursive sequences fail as Python exceptions across dictionary nesting
first, second = [], []
first.append({'cycle': first})
second.append({'cycle': second})
assert first == first
try:
    first == second
    assert False
except RecursionError:
    pass
