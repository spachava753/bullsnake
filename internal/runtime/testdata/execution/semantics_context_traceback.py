# case: nested context exits observe live traceback clearing in order
seen = []
class Manager:
    def __init__(self, clear):
        self.clear = clear
    def __enter__(self):
        return self
    def __exit__(self, kind, value, traceback):
        assert kind is ValueError
        assert traceback is value.__traceback__
        seen.append(traceback)
        if self.clear:
            value.__traceback__ = None
        return False
try:
    with Manager(False), Manager(True):
        raise ValueError('nested')
except ValueError as error:
    assert error.__traceback__ is None
assert seen[0] is not None
assert seen[1] is None
