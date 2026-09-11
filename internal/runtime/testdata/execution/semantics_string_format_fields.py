# case: numbered string fields select and repeat positional arguments
assert '{1} {0} {1!r}'.format('first', 'second') == "second first 'second'"
assert '{{{0}}}'.format(4) == '{4}'
assert '{00} {01}'.format('a', 'b') == 'a b'
for text in ['{} {0}', '{0} {}']:
    try:
        text.format(1, 2)
        assert False
    except ValueError:
        pass

# ---
# case: positional field attributes run descriptors and representation callbacks
calls = []
class Display:
    def __repr__(self):
        calls.append('repr')
        return '<display>'
class Descriptor:
    def __get__(self, instance, owner):
        calls.append('get')
        return Display()
class C:
    value = Descriptor()
assert '{0.__class__.__name__}({0.value!r})'.format(C()) == 'C(<display>)'
assert calls == ['get', 'repr']
assert '{.__class__.__name__}'.format(C()) == 'C'
try:
    '{0.missing}'.format(C())
    assert False
except AttributeError:
    pass
class Broken:
    @property
    def value(self):
        raise ValueError('attribute failed')
try:
    '{0.value}'.format(Broken())
    assert False
except ValueError as error:
    assert str(error) == 'attribute failed'
