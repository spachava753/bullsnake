# Runtime exception cases for collections.
# case: augmented operand types
# error: TypeError
# message: "unsupported operand type(s) for +=: 'str' and 'int'"
value = 'x'
value += 1
# ---
# case: non-string text membership
# error: TypeError
# message: "'in <string>' requires string as left operand, not int"
answer = 1 in 'abc'
# ---
# case: string in bytes membership
# error: TypeError
# message: "a bytes-like object is required, not 'str'"
answer = 'A' in b'ABC'
# ---
# case: out-of-range byte membership
# error: ValueError
# message: "byte must be in range(0, 256)"
answer = 256 in b'ABC'
# ---
# case: huge integer byte membership
# error: TypeError
# message: "a bytes-like object is required, not 'int'"
answer = 1000000000000000000000000000000 in b'ABC'
# ---
# case: unhashable set membership
# error: TypeError
# message: "cannot use 'list' as a set element (unhashable type: 'list')"
answer = [] in {1}
# ---
# case: unhashable dictionary membership
# error: TypeError
# message: "cannot use 'list' as a dict key (unhashable type: 'list')"
answer = [] in {}
# ---
# case: membership in non-container
# error: TypeError
# message: "argument of type 'int' is not a container or iterable"
answer = 1 in 2
# ---
# case: unhashable set element
# error: TypeError
# message: "cannot use 'list' as a set element (unhashable type: 'list')"
answer = {[1]}
# ---
# case: nested unhashable set element
# error: TypeError
# message: "cannot use 'tuple' as a set element (unhashable type: 'list')"
answer = {([1],)}
# ---
# case: non-iterable starred set
# error: TypeError
# message: "'int' object is not iterable"
answer = {*1}
# ---
# case: non-mapping dictionary unpack
# error: TypeError
# message: "'int' object is not a mapping"
answer = {**1}
# ---
# case: delete missing dictionary key
# error: KeyError
# message: "'missing'"
mapping = {}
del mapping['missing']
# ---
# case: unhashable dictionary assignment
# error: TypeError
# message: "cannot use 'list' as a dict key (unhashable type: 'list')"
mapping = {}
mapping[[1]] = 2
# ---
# case: unhashable dictionary deletion
# error: TypeError
# message: "cannot use 'list' as a dict key (unhashable type: 'list')"
mapping = {}
del mapping[[1]]
# ---
# case: missing dictionary key
# error: KeyError
# message: "'missing'"
answer = {'present': 1}['missing']
# ---
# case: missing tuple dictionary key
# error: KeyError
# message: "(1, 2)"
answer = {}[(1, 2)]
# ---
# case: unhashable dictionary subscription
# error: TypeError
# message: "cannot use 'list' as a dict key (unhashable type: 'list')"
answer = {}[[1]]
# ---
# case: unhashable dictionary key
# error: TypeError
# message: "cannot use 'list' as a dict key (unhashable type: 'list')"
answer = {[1]: 2}
# ---
# case: nested unhashable dictionary key
# error: TypeError
# message: "cannot use 'tuple' as a dict key (unhashable type: 'list')"
answer = {([1],): 2}
# ---
# case: non-iterable starred display
# error: TypeError
# message: "Value after * must be an iterable, not int"
answer = [*1]
# ---
# case: zero slice step
# error: ValueError
# message: "slice step cannot be zero"
answer = [1, 2][::0]
# ---
# case: non-integer slice bound
# error: TypeError
# message: "slice indices must be integers or None or have an __index__ method"
answer = (1, 2)[1.5:]
# ---
# case: non-subscriptable slice
# error: TypeError
# message: "'NoneType' object is not subscriptable"
answer = None[:]
# ---
# case: huge string index
# error: IndexError
# message: "cannot fit 'int' into an index-sized integer"
answer = 'x'[1000000000000000000000000000000]
# ---
# case: string index out of range
# error: IndexError
# message: "string index out of range"
answer = 'x'[1]
# ---
# case: bytes index out of range
# error: IndexError
# message: "index out of range"
answer = b'x'[-2]
# ---
# case: non-integer string index
# error: TypeError
# message: "string indices must be integers, not 'float'"
answer = 'x'[1.5]
# ---
# case: non-integer bytes index
# error: TypeError
# message: "byte indices must be integers or slices, not float"
answer = b'x'[1.5]
# ---
# case: tuple index out of range
# error: IndexError
# message: "tuple index out of range"
answer = (1, 2)[2]
# ---
# case: list index out of range
# error: IndexError
# message: "list index out of range"
answer = [1, 2][-3]
# ---
# case: huge sequence index
# error: IndexError
# message: "cannot fit 'int' into an index-sized integer"
answer = [1][1000000000000000000000000000000]
# ---
# case: non-integer sequence index
# error: TypeError
# message: "list indices must be integers or slices, not float"
answer = [1][1.5]
# ---
# case: non-subscriptable value
# error: TypeError
# message: "'NoneType' object is not subscriptable"
answer = None[0]
# ---
# case: dictionary insertion during iteration
# error: RuntimeError
# message: "dictionary changed size during iteration"
mapping = {'first': 1}
for key in mapping:
    mapping['second'] = 2
# ---
# case: dictionary deletion during iteration
# error: RuntimeError
# message: "dictionary changed size during iteration"
mapping = {'first': 1, 'second': 2}
for key in mapping:
    del mapping['second']
# ---
# case: dictionary keys replaced during iteration
# error: RuntimeError
# message: "dictionary keys changed during iteration"
mapping = {'first': 1, 'second': 2}
for key in mapping:
    del mapping['second']
    mapping['third'] = 3
# ---
# case: non-iterable for loop
# error: TypeError
# message: "'int' object is not iterable"
for value in 1:
    pass
# ---
# case: not enough values for starred unpack
# error: ValueError
# message: "not enough values to unpack (expected at least 2, got 1)"
first, *middle, last = [1]
# ---
# case: non-iterable starred unpack
# error: TypeError
# message: "cannot unpack non-iterable int object"
first, *middle = 1
# ---
# case: not enough values to unpack
# error: ValueError
# message: "not enough values to unpack (expected 2, got 1)"
first, second = (1,)
# ---
# case: too many values to unpack
# error: ValueError
# message: "too many values to unpack (expected 2)"
first, second = (1, 2, 3)
# ---
# case: non-iterable unpack
# error: TypeError
# message: "cannot unpack non-iterable int object"
first, second = 1
# ---
# case: non-iterable list comprehension
# error: TypeError
# message: "'int' object is not iterable"
values = [item for item in 1]
# ---
# case: list comprehension target does not leak
# error: NameError
# message: "name 'item' is not defined"
values = [item for item in ()]
item
# ---
# case: non-iterable set comprehension
# error: TypeError
# message: "'int' object is not iterable"
values = {item for item in 1}
# ---
# case: unhashable set comprehension value
# error: TypeError
# message: "cannot use 'list' as a set element (unhashable type: 'list')"
values = {item for item in ([1],)}
