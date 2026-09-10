# case: print writes separate pieces and accepts all keyword controls
class Stream:
    def __init__(self):
        self.parts = []
        self.flushed = 0
    def write(self, text):
        self.parts.append(text)
        return 'ignored'
    def flush(self):
        self.flushed += 1
s = Stream()
assert print('é', 2, None, sep='|', end='!', file=s, flush=True) is None
assert s.parts == ['é', '|', '2', '|', 'None', '!']
assert s.flushed == 1
print(file=s)
print('a', 'b', sep=None, end=None, file=s)
assert s.parts == ['é', '|', '2', '|', 'None', '!', '\n', 'a', ' ', 'b', '\n']
# ---
# case: flush truth precedes stream resolution and each write lookup precedes str
import sys
events = []
class Stream:
    @property
    def write(self):
        events.append('lookup')
        return self.save
    def save(self, text):
        events.append(text)
    def flush(self):
        events.append('flush')
class Flag:
    def __bool__(self):
        events.append('truth')
        sys.stdout = Stream()
        return True
class Item:
    def __str__(self):
        events.append('str')
        return 'value'
print(Item(), flush=Flag())
assert events == ['truth', 'lookup', 'str', 'value', 'lookup', '\n', 'flush']
# ---
# case: current stdout is resolved per call and retained during that call
import sys
class Stream:
    def __init__(self):
        self.parts = []
    def write(self, text):
        self.parts.append(text)
first = Stream()
second = Stream()
sys.stdout = first
original = sys.__stdout__
class Item:
    def __str__(self):
        sys.stdout = second
        return 'first'
print(Item(), 'still first', sep=':', end='')
print('second', file=None)
assert first.parts == ['first', ':', 'still first', '']
assert second.parts == ['second', '\n']
assert sys.__stdout__ is original
# ---
# case: failed writes preserve earlier output and stop conversion and flushing
seen = []
class Stream:
    def write(self, text):
        seen.append(text)
        if text == ',':
            raise OSError('write failed')
    def flush(self):
        seen.append('flush')
class Later:
    def __str__(self):
        seen.append('later')
        return 'bad'
try:
    print('first', Later(), sep=',', file=Stream(), flush=True)
    assert False
except OSError as error:
    assert str(error) == 'write failed'
assert seen == ['first', ',']
# ---
# case: conversion and flush failures propagate with partial output intact
seen = []
class Stream:
    def write(self, text):
        seen.append(text)
    def flush(self):
        raise ValueError('flush failed')
class Bad:
    def __str__(self):
        raise LookupError('conversion failed')
try:
    print('one', Bad(), file=Stream())
    assert False
except LookupError:
    pass
assert seen == ['one', ' ']
try:
    print(file=Stream(), flush=True)
    assert False
except ValueError as error:
    assert str(error) == 'flush failed'
assert seen == ['one', ' ', '\n']
# ---
# case: invalid print arguments have no write side effects
class Stream:
    def write(self, text):
        assert False
s = Stream()
for call in (lambda: print(file=s, sep=1), lambda: print(file=s, end=[]),
             lambda: print(file=s, unknown=True)):
    try:
        call()
        assert False
    except TypeError:
        pass
class Bad:
    def __bool__(self):
        raise ValueError('truth failed')
try:
    print(file=s, flush=Bad())
    assert False
except ValueError:
    pass
# ---
# case: unavailable stdout is explicitly denied under the host policy
import sys
assert sys.stdout is None
try:
    print('denied')
    assert False
except PermissionError:
    pass
del sys.stdout
try:
    print('missing')
    assert False
except RuntimeError as error:
    assert str(error) == 'lost sys.stdout'
