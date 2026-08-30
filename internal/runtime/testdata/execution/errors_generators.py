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
# ---
# case: throw requires an exception
# error: TypeError
# message: "throw expected at least 1 argument, got 0"
def values():
    yield 1

values().throw()
# ---
# case: throw limits arguments
# error: TypeError
# message: "throw expected at most 3 arguments, got 4"
def values():
    yield 1

values().throw(ValueError, 'bad', None, None)
# ---
# case: throw rejects keywords
# error: TypeError
# message: "throw() takes no keyword arguments"
def values():
    yield 1

values().throw(value=ValueError())
# ---
# case: throw requires exception value
# error: TypeError
# message: "exceptions must be classes or instances deriving from BaseException, not int"
def values():
    yield 1

values().throw(1)
# ---
# case: throw rejects separate instance value
# error: TypeError
# message: "instance exception may not have a separate value"
def values():
    yield 1

values().throw(ValueError('first'), 'second')
# ---
# case: throw requires traceback object
# error: TypeError
# message: "throw() third argument must be a traceback object"
def values():
    yield 1

values().throw(ValueError, 'bad', 1)
# ---
# case: throw rejects generator reentry
# error: ValueError
# message: "generator already executing"
current = None

def recursive_throw():
    yield 1
    current.throw(ValueError('nested'))

current = recursive_throw()
next(current)
current.send(None)
# ---
# case: close rejects arguments
# error: TypeError
# message: "generator.close() takes no arguments (1 given)"
def values():
    yield 1

values().close(None)
# ---
# case: close rejects keywords
# error: TypeError
# message: "generator.close() takes no keyword arguments"
def values():
    yield 1

values().close(value=None)
# ---
# case: close rejects yielded value
# error: RuntimeError
# message: "generator ignored GeneratorExit"
def ignores_close():
    try:
        yield 1
    except GeneratorExit:
        yield 2

stream = ignores_close()
next(stream)
stream.close()
# ---
# case: close propagates replacement exception
# error: ValueError
# message: "close failed"
def close_failure():
    try:
        yield 1
    finally:
        raise ValueError('close failed')

stream = close_failure()
next(stream)
stream.close()
# ---
# case: close rejects generator reentry
# error: ValueError
# message: "generator already executing"
current = None

def recursive_close():
    yield 1
    current.close()

current = recursive_close()
next(current)
current.send(None)
# ---
# case: throw GeneratorExit is not close
# error: GeneratorExit
# message: ""
def values():
    yield 1

stream = values()
next(stream)
stream.throw(GeneratorExit())
# ---
# case: yield from rejects non-iterable
# error: TypeError
# message: "'int' object is not iterable"
def outer():
    yield from 1

next(outer())
# ---
# case: yield from native iterator rejects sent value
# error: AttributeError
# message: "'tuple_iterator' object has no attribute 'send'"
def outer():
    yield from (1, 2)

stream = outer()
next(stream)
stream.send(9)
# ---
# case: yield from propagates unhandled thrown exception
# error: ValueError
# message: "forwarded"
def inner():
    yield 1

def outer():
    yield from inner()

stream = outer()
next(stream)
stream.throw(ValueError('forwarded'))
# ---
# case: yield from close propagates delegate replacement exception
# error: ValueError
# message: "delegated close failed"
def failing_close_delegate():
    try:
        yield 1
    finally:
        raise ValueError('delegated close failed')

def failing_close_outer():
    yield from failing_close_delegate()

stream = failing_close_outer()
next(stream)
stream.close()
