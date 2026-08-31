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
