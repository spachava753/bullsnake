# case: native list structural equality
assert [] == []
assert [1, 'two', None] == [1, 'two', None]
assert [1, [2, 3], (4, 5)] == [1, [2, 3], (4, 5)]
assert [1, 2] != [1, 3]
assert [1] != [1, 2]
assert [1, 2] != (1, 2)
assert not ([1, 2] == (1, 2))
