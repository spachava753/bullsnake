# Runtime execution cases for synchronous generators.
# case: generator suspension and resumption
events = 0
def generate():
    global events
    events = 1
    received = yield 10
    events = 2
    yield received
    events = 3
iterator = generate()
assert events == 0
values = [value for value in iterator]
assert f'{values!r}' == "[10, None]", "values"
assert events == 3
# ---
# case: generator expressions and delegation
def delegated(values):
    yield -1
    yield from values
    yield 9
expression_values = [value for value in (item * 2 for item in [1, 2, 3] if item > 1)]
delegated_values = [value for value in delegated([4, 5])]
assert f'{expression_values!r}' == "[4, 6]", "expression_values"
assert f'{delegated_values!r}' == "[-1, 4, 5, 9]", "delegated_values"
