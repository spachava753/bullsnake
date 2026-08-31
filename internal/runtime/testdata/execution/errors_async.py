# case: coroutine body exceptions leave the suspended frame
# error: ValueError
# message: "broken coroutine"
async def failing_coroutine():
    raise ValueError('broken coroutine')

failing_coroutine().send(None)

# ---
# case: just-started coroutines reject non-None sends
# error: TypeError
# message: "can't send non-None value to a just-started coroutine"
async def unstarted_coroutine():
    return 1

unstarted_coroutine().send(1)

# ---
# case: completed coroutines cannot be reused
# error: RuntimeError
# message: "cannot reuse already awaited coroutine"
async def one_shot_coroutine():
    return 1

one_shot = one_shot_coroutine()
try:
    one_shot.send(None)
except StopIteration:
    pass
one_shot.send(None)

# ---
# case: coroutines are not iterators
# error: TypeError
# message: "'coroutine' object is not an iterator"
async def next_coroutine():
    return 1

next(next_coroutine())

# ---
# case: coroutines are not iterable
# error: TypeError
# message: "'coroutine' object is not iterable"
async def iterated_coroutine():
    return 1

for value in iterated_coroutine():
    pass
