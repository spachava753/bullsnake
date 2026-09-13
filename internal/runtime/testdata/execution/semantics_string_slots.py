# case: native string allocation representation and formatting execute
assert str.__new__(str) == ''
value = 'native text'
assert str.__new__(str, value) is value
assert str.__str__(value) is value
assert str.__repr__('a\nb') == "'a\\nb'"
assert str.__format__('a', '>3') == '  a'
assert str.__format__(value, '') is value
assert value.__format__('') is value
assert str.__getitem__('aéz', 1) == 'é'
class Keys:
    def __getitem__(self, key):
        return key
assert str.__getitem__('aéz', Keys()[:2]) == 'aé'
assert str.__eq__('a', 'a') is True
assert str.__ne__('a', 'b') is True
assert str.__lt__('a', 'b') is True
assert str.__le__('a', 'a') is True
assert str.__gt__('b', 'a') is True
assert str.__ge__('a', 'a') is True
assert str.__eq__('1', 1) is NotImplemented
assert 'a'.__lt__('b') is True
class Text:
    def __str__(self):
        return 'converted'
assert str.__new__(str, Text()) == 'converted'

# ---
# case: string method descriptors share the existing native implementations
assert str.lower('ABC') == 'abc'
assert str.capitalize('hELLO') == 'Hello'
assert str.count('banana', 'an') == 2
assert str.startswith('hello', 'he') is True
assert str.endswith('hello', 'lo') is True
assert str.removeprefix('hello', 'he') == 'llo'
assert str.replace('hello', 'l', 'x', count=1) == 'hexlo'
assert str.split('a,b', ',') == ['a', 'b']
assert str.splitlines('a\nb') == ['a', 'b']
assert str.strip('  hello  ') == 'hello'
assert str.format('value={0}', 42) == 'value=42'
assert type(str.lower).__name__ == 'method_descriptor'
assert str.lower.__objclass__ is str
assert str.lower.__get__('ABC', str)() == 'abc'
assert str.lower is str.__dict__['lower']

# ---
# case: string native descriptors preserve failures and receiver checks
for call in (
    lambda: str.__new__(int, 'text'),
    lambda: str.__repr__(1),
    lambda: str.__str__(),
    lambda: str.__format__('a', 1),
    lambda: str.__getitem__('a'),
    lambda: str.lower(1),
    lambda: str.lower('A', 1),
    lambda: str.lower('A', extra=1),
):
    try:
        call()
    except TypeError:
        pass
    else:
        assert False
try:
    str.__getitem__('a', 5)
except IndexError:
    pass
else:
    assert False
class Bad:
    def __str__(self):
        raise ValueError('conversion failed')
try:
    str.__new__(str, Bad())
except ValueError as error:
    assert str(error) == 'conversion failed'
else:
    assert False
