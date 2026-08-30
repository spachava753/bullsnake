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
