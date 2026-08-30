# case: module arithmetic
left = 40
right = 2
answer = left + right
assert answer == 42
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
