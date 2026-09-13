# case: object formatting uses real string conversion only for an empty spec
calls = []
class Sample:
    def __str__(self):
        calls.append('str')
        return 'sample text'
    def __format__(self, spec):
        calls.append('format')
        return 'custom'
value = Sample()
assert object.__format__(value, '') == 'sample text'
assert calls == ['str']
assert type(object.__format__).__name__ == 'method_descriptor'
assert object.__format__(42, '') == '42'
try:
    object.__format__(value, 'nonempty')
    assert False
except TypeError as error:
    assert str(error) == 'unsupported format string passed to Sample.__format__'
assert calls == ['str']

# ---
# case: inherited and super object format calls preserve receiver dispatch
class Base:
    def __repr__(self):
        return 'base repr'
assert Base().__format__('') == 'base repr'
class Child(Base):
    def __format__(self, spec):
        return super().__format__(spec)
    def root_hash(self):
        return super().__hash__()
child = Child()
assert child.root_hash() == object.__hash__(child)
formatted = child.__format__('')
assert formatted == 'base repr', formatted
assert [1, 2].__format__('') == '[1, 2]'
try:
    object.__format__(object(), 1)
    assert False
except TypeError as error:
    assert str(error) == '__format__() argument must be str, not int'
class Bad:
    def __str__(self):
        return 1
try:
    object.__format__(Bad(), '')
    assert False
except TypeError as error:
    assert str(error) == '__str__ returned non-string (type int)'
