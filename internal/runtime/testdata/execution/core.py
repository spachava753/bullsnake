# case: module arithmetic
left = 40
right = 2
answer = left + right
assert left == 40
assert right == 2
assert answer == 42
# ---
# case: arbitrary precision module arithmetic
left = 99999999999999999999999999999999999999
right = 1
answer = left + right
assert left == 99999999999999999999999999999999999999
assert right == 1
assert answer == 100000000000000000000000000000000000000
# ---
# case: pass and discarded expression
pass
40
answer = 2
assert answer == 2
# ---
# case: sibling closures share a cell
def outer():
    value = 1
    def read():
        return value
    def write():
        nonlocal value
        value = 2
    return read, write

read, write = outer()
assert read() == 1
write()
assert read() == 2
