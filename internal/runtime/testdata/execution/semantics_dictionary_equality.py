# case: recursive and excessively deep dictionary comparisons raise Python errors
left, right = {}, {}
left['self'] = left
right['self'] = right
assert left == left
try:
    left == right
    assert False
except RecursionError as error:
    assert isinstance(error, RuntimeError)
    assert str(error) == 'maximum recursion depth exceeded in comparison'
left, right = {}, {}
for index in range(1100):
    left, right = {'child': left}, {'child': right}
try:
    left == right
    assert False
except RecursionError:
    pass

# ---
# case: dictionary equality ignores insertion order and compares values
assert {'a': 1, 'b': 2} == {'b': 2, 'a': 1}
assert {} == {}
assert {'a': 1} != {'a': 2}
assert {'a': 1} != {'b': 1}
assert {'a': 1} != {'a': 1, 'b': 2}
assert {'a': 1} != [('a', 1)]
assert {'a': {'b': 2}} == {'a': {'b': 2}}

# ---
# case: dictionary value equality invokes Python and truth-tests its result
calls = []
class Truth:
    def __bool__(self):
        calls.append('truth')
        return True
class Equal:
    def __eq__(self, other):
        calls.append('equal')
        return Truth()
left, right = Equal(), Equal()
assert {'a': left} == {'a': right}
assert calls == ['equal', 'truth']
calls = []
assert {'a': left} == {'a': left}
assert calls == []
class Broken:
    def __eq__(self, other):
        raise ValueError('value comparison')
try:
    {'a': Broken()} == {'a': 1}
    assert False
except ValueError as error:
    assert str(error) == 'value comparison'

# ---
# case: dictionary inequality inverts value equality rather than using value ne
class Value:
    def __eq__(self, other):
        return True
    def __ne__(self, other):
        raise AssertionError('must use equality')
assert not {'a': Value()} != {'a': Value()}
