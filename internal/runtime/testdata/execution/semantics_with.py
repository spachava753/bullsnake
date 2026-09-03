# Runtime execution cases for synchronous context managers.
# case: normal and suppressing context exits
events = 0
seen_type = None
seen_exception = None
class Manager:
    def __init__(self, value, suppress=False):
        self.value = value
        self.suppress = suppress
    def __enter__(self):
        global events
        events = events * 10 + self.value
        return self.value
    def __exit__(self, exception_type, exception, traceback):
        global events, seen_type, seen_exception
        events = events * 10 + self.value + 1
        seen_type = exception_type
        seen_exception = exception
        return self.suppress
with Manager(1) as value:
    assert value == 1
with Manager(3, True):
    raise ValueError('hidden')
assert events == 1234
assert seen_type is ValueError
assert seen_exception is not None
# ---
# case: nested exit order and control transfer
events = 0
class Manager:
    def __init__(self, value):
        self.value = value
    def __enter__(self):
        global events
        events = events * 10 + self.value
        return self
    def __exit__(self, exception_type, exception, traceback):
        global events
        events = events * 10 + self.value + 5
def choose():
    with Manager(1), Manager(2):
        return 7
result = choose()
assert result == 7
assert events == 1276
# ---
# case: unsuppressed context exception
events = 0
class Manager:
    def __enter__(self):
        return self
    def __exit__(self, exception_type, exception, traceback):
        global events
        events = 1
        return False
try:
    with Manager():
        raise KeyError('visible')
except KeyError:
    caught = True
assert caught
assert events == 1
