# case: integer list assignment preserves aliases and element identity
items = [1, 2, 3]
alias = items
replacement = ['value']
items[0] = replacement
items[-1] = 4
items[True] = 5
assert alias is items
assert items[0] is replacement
assert items == [replacement, 5, 4]
items[1] += 2
assert items[1] == 7
iterator = iter(items)
assert next(iterator) is replacement
items[1] = 'changed'
assert next(iterator) == 'changed'

# ---
# case: invalid list indexes leave the list unchanged
items = [1, 2]
for index in [-3, 2, 10 ** 100, -(10 ** 100)]:
    try:
        items[index] = 9
        assert False
    except IndexError:
        pass
    assert items == [1, 2]
for index in ['bad', None, 1.5]:
    try:
        items[index] = 9
        assert False
    except TypeError:
        pass
    assert items == [1, 2]
