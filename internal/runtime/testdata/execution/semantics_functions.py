# Runtime execution cases for functions.
# case: lexical closures
def make(value):
    offset = 2
    def apply(item):
        return value + offset + item
    return apply
captured = make(40)(0)
def counter(start):
    value = start
    def set_value(new):
        nonlocal value
        value = new
    def read():
        return value
    return set_value, read
set_value, read = counter(3)
before = read()
set_result = set_value(9)
after = read()
def outer():
    a = 1
    z = 2
    def middle():
        def inner():
            return z, a
        return inner
    return middle
transitive = outer()()()
def make_lambda(offset, default):
    return lambda value=default: offset + value
lambda_result = make_lambda(10, 32)()
assert f'{captured!r}' == "42", "captured"
assert f'{before!r}' == "3", "before"
assert f'{set_result!r}' == "None", "set_result"
assert f'{after!r}' == "9", "after"
assert f'{transitive!r}' == "(2, 1)", "transitive"
assert f'{lambda_result!r}' == "42", "lambda_result"
# ---
# case: decorated definitions
order = 0
def record(value):
    global order
    order = order * 10 + value
    return value
def decorate(label):
    record(label)
    def apply(function):
        record(label + 2)
        def wrapped(value):
            return function(value) + label
        return wrapped
    return apply
@decorate(1)
@decorate(2)
def target(value=record(5)):
    return value
observed_order = order
decorated = target(10)
def replace(function):
    def replacement():
        return 42
    return replacement
@replace
def ignored():
    return 0
replaced = ignored()
assert f'{observed_order!r}' == "12543", "observed_order"
assert f'{decorated!r}' == "13", "decorated"
assert f'{replaced!r}' == "42", "replaced"
# ---
# case: deferred function annotations
events = 0
def mark():
    global events
    events = events + 1
    return 99
def identity(value: mark()) -> mark():
    return value
before = events
result = identity(42)
after = events
def outer(annotation):
    def nested(value: annotation) -> annotation:
        return value
    return nested
nested_result = outer('kind')(7)
assert f'{before!r}' == "0", "before"
assert f'{result!r}' == "42", "result"
assert f'{after!r}' == "0", "after"
assert f'{nested_result!r}' == "7", "nested_result"
