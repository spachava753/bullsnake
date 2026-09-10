# case: immediate subclasses preserve definition order and return snapshots
class Base:
    pass
class First(Base):
    pass
class Second(Base):
    pass
class Grandchild(First):
    pass
assert Base.__subclasses__() == [First, Second]
assert First.__subclasses__() == [Grandchild]
assert Grandchild.__subclasses__() == []
snapshot = Base.__subclasses__()
snapshot.append(int)
assert Base.__subclasses__() == [First, Second]
assert type.__subclasses__(Base) == [First, Second]
# ---
# case: multiple inheritance and dynamic class construction track direct edges
class Left:
    pass
class Right:
    pass
Both = type('Both', (Left, Right), {})
assert Left.__subclasses__() == [Both]
assert Right.__subclasses__() == [Both]
# ---
# case: user subclasses method overrides retain ordinary lookup
class Base:
    @classmethod
    def __subclasses__(cls):
        return [str]
class Child(Base):
    pass
assert Base.__subclasses__() == [str]
assert type.__subclasses__(Base) == [Child]
# ---
# case: subclasses arguments are checked
class Base:
    pass
for call in (lambda: Base.__subclasses__(1), lambda: Base.__subclasses__(x=1),
             lambda: type.__subclasses__(), lambda: type.__subclasses__(1)):
    try:
        call()
        assert False
    except TypeError:
        pass
