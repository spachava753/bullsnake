# Runtime exception cases for basic generators.
# case: uncaught generator failure after suspension
# error: ValueError
# message: "generator failed"
def fail_after_yield():
    yield 1
    raise ValueError('generator failed')

for value in fail_after_yield():
    pass
# ---
# case: generator reentry
# error: ValueError
# message: "generator already executing"
current = None

def recursive():
    for value in current:
        yield value

current = recursive()
for value in current:
    pass
# ---
# case: generator initializer result
# error: TypeError
# message: "__init__() should return None, not 'generator'"
class Broken:
    def __init__(self):
        yield 1

Broken()
# ---
# case: next rejects non-iterator
# error: TypeError
# message: "'int' object is not an iterator"
next(1)
# ---
# case: next requires an iterator
# error: TypeError
# message: "next expected at least 1 argument, got 0"
next()
# ---
# case: next limits positional arguments
# error: TypeError
# message: "next expected at most 2 arguments, got 3"
def values():
    yield 1

next(values(), None, None)
# ---
# case: next rejects keyword arguments
# error: TypeError
# message: "next() takes no keyword arguments"
def values():
    yield 1

next(values(), default=None)
# ---
# case: next rejects generator reentry
# error: ValueError
# message: "generator already executing"
current = None

def recursive_next():
    yield next(current)

current = recursive_next()
next(current)
# ---
# case: send rejects initial value
# error: TypeError
# message: "can't send non-None value to a just-started generator"
def values():
    yield 1

values().send(1)
# ---
# case: send requires one argument
# error: TypeError
# message: "generator.send() takes exactly one argument (0 given)"
def values():
    yield 1

values().send()
# ---
# case: send limits arguments
# error: TypeError
# message: "generator.send() takes exactly one argument (2 given)"
def values():
    yield 1

values().send(None, None)
# ---
# case: send rejects keywords
# error: TypeError
# message: "generator.send() takes no keyword arguments"
def values():
    yield 1

values().send(value=None)
# ---
# case: send rejects generator reentry
# error: ValueError
# message: "generator already executing"
current = None

def recursive_send():
    yield 1
    current.send(None)

current = recursive_send()
current.send(None)
current.send(None)
