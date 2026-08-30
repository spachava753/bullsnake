# Runtime exception cases for structural pattern matching.
# case: mapping pattern rejects duplicate dynamic keys
# error: ValueError
# message: "mapping pattern checks duplicate key ('same')"
class DuplicateKeys:
    pass

DuplicateKeys.first = 'same'
DuplicateKeys.second = 'same'

match {'same': 1}:
    case {DuplicateKeys.first: first, DuplicateKeys.second: second}:
        result = first + second
# ---
# case: class pattern requires a class
# error: TypeError
# message: "called match pattern must be a class"
pattern_target = 1
match 1:
    case pattern_target():
        pass
# ---
# case: class pattern requires tuple match args
# error: TypeError
# message: "Bad.__match_args__ must be a tuple (got list)"
class Bad:
    __match_args__ = []

match Bad():
    case Bad(value):
        pass
# ---
# case: class pattern requires string match args
# error: TypeError
# message: "__match_args__ elements must be strings (got int)"
class BadElement:
    __match_args__ = (1,)

match BadElement():
    case BadElement(value):
        pass
# ---
# case: class pattern limits positional patterns
# error: TypeError
# message: "Empty() accepts 0 positional sub-patterns (1 given)"
class Empty:
    pass

match Empty():
    case Empty(value):
        pass
# ---
# case: class pattern rejects duplicate attributes
# error: TypeError
# message: "Point() got multiple sub-patterns for attribute 'x'"
class Point:
    __match_args__ = ('x',)

point = Point()
point.x = 1
match point:
    case Point(value, x=other):
        pass
