# Runtime exception cases for synchronous context managers.
# case: context manager missing exit
# error: TypeError
# message: "'MissingExit' object does not support the context manager protocol (missed __exit__ method)"
class MissingExit:
    def __enter__(self):
        return self

with MissingExit():
    pass
# ---
# case: context manager missing enter
# error: TypeError
# message: "'MissingEnter' object does not support the context manager protocol (missed __enter__ method)"
class MissingEnter:
    def __exit__(self, kind, value, traceback):
        return False

with MissingEnter():
    pass
# ---
# case: context manager does not suppress exception
# error: ValueError
# message: "visible"
class DoesNotSuppress:
    def __enter__(self):
        return self

    def __exit__(self, kind, value, traceback):
        return False

with DoesNotSuppress():
    raise ValueError('visible')
# ---
# case: context exit replaces normal completion
# error: RuntimeError
# message: "exit failed"
class ExitFailure:
    def __enter__(self):
        return self

    def __exit__(self, kind, value, traceback):
        raise RuntimeError('exit failed')

with ExitFailure():
    pass
