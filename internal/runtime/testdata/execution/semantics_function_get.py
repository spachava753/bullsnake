# case: function descriptor get creates real bound methods or returns the function
calls = []
def method(self, value=1, *, label='default'):
    calls.append((self, value, label))
    return value
class Owner:
    action = method
owner = Owner()
bound = method.__get__(owner, Owner)
assert bound.__self__ is owner and bound.__func__ is method
assert bound(42, label='named') == 42
assert calls == [(owner, 42, 'named')]
assert method.__get__(None, Owner) is method
assert type(method).__get__(method, owner)(2) == 2
assert method.__get__(42, None)(3) == 3
assert calls[-1] == (42, 3, 'default')
assert method.__get__(owner, 17)(4) == 4
assert type(type(method).__get__).__name__ == 'wrapper_descriptor'
assert type(method).__get__.__objclass__ is type(method)

# ---
# case: function get attributes can be replaced without changing special binding
calls = []
def method(self):
    calls.append(self)
    return 'bound'
class Owner:
    action = method
owner = Owner()
method.__get__ = lambda *args: 'attribute override'
assert method.__get__(owner) == 'attribute override'
assert owner.action() == 'bound'
assert calls == [owner]
del method.__get__
assert method.__get__(owner)() == 'bound'
assert calls == [owner, owner]

# ---
# case: function get validates wrapper arguments without restricting bound owners
def function(self):
    return self
for call in (
    lambda: function.__get__(),
    lambda: function.__get__(None),
    lambda: function.__get__(None, None),
    lambda: function.__get__(1, 2, 3),
    lambda: function.__get__(1, owner=object),
    lambda: type(function).__get__(42, 1),
):
    try:
        call()
    except TypeError:
        pass
    else:
        assert False
try:
    function.__get__(None, None)
except TypeError as error:
    assert str(error) == '__get__(None, None) is invalid'
else:
    assert False
assert function.__get__(1)() == 1
