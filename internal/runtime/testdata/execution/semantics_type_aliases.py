# case: non-generic type aliases are lazy and cached
events = 0

def build_alias_value():
    global events
    events = events + 1
    return []

type Product = build_alias_value()
before = events
assert Product.__name__ == 'Product'
assert Product.__module__ == 'fixture'
assert f'{Product!r}' == 'Product'
assert f'{Product.__type_params__!r}' == '()'
first_value = Product.__value__
after_first = events
second_value = Product.__value__
after_second = events
assert before == 0
assert after_first == 1
assert after_second == 1
assert first_value is second_value
# ---
# case: type aliases retain definition scopes
type Forward = later
later = 42
assert Forward.__value__ == 42

def make_alias(value):
    type Captured = value
    return Captured

captured_alias = make_alias(99)
assert captured_alias.__value__ == 99

class AliasOwner:
    Field = 7
    type FieldAlias = Field

assert AliasOwner.FieldAlias.__value__ == 7

type Recursive = Recursive
assert Recursive.__value__ is Recursive
# ---
# case: failed type alias evaluation is retried
alias_attempts = 0

def fail_alias():
    global alias_attempts
    alias_attempts = alias_attempts + 1
    raise ValueError('retry alias')

type RetryAlias = fail_alias()
try:
    RetryAlias.__value__
except ValueError:
    first_alias_failure = True
try:
    RetryAlias.__value__
except ValueError:
    second_alias_failure = True
assert first_alias_failure
assert second_alias_failure
assert alias_attempts == 2
# ---
# case: generic aliases expose inferred type variables
# CPython 3.14.7: Lib/test/test_type_aliases.py and Objects/typevarobject.c.
type Pair[T, U] = (T, U)
pair_parameters = Pair.__type_params__
T = pair_parameters[0]
U = pair_parameters[1]
assert Pair.__type_params__ is pair_parameters
assert f'{pair_parameters!r}' == '(T, U)'
assert T.__name__ == 'T'
assert T.__bound__ is None
assert T.__constraints__ == ()
assert T.__covariant__ is False
assert T.__contravariant__ is False
assert T.__infer_variance__ is True
assert f'{T!r}' == 'T'
assert U.__name__ == 'U'
pair_value = Pair.__value__
assert pair_value[0] is T
assert pair_value[1] is U
assert Pair.__value__ is pair_value
# ---
# case: generic aliases retain enclosing and class scopes
def make_generic_alias(marker):
    type Wrapped[T] = (T, marker)
    return Wrapped

Wrapped = make_generic_alias('enclosing')
wrapped_parameter = Wrapped.__type_params__[0]
assert Wrapped.__value__[0] is wrapped_parameter
assert Wrapped.__value__[1] == 'enclosing'

class GenericAliasOwner:
    marker = 'class'
    type Field[T] = (T, marker)

field_parameter = GenericAliasOwner.Field.__type_params__[0]
assert GenericAliasOwner.Field.__value__[0] is field_parameter
assert GenericAliasOwner.Field.__value__[1] == 'class'
