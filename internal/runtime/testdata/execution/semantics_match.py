# Runtime execution cases for structural pattern matching.
# case: value capture wildcard and guard patterns
subject = 2
match subject:
    case 1:
        result = 'one'
    case captured if captured > 1:
        result = captured
    case _:
        result = 'other'
assert result == 2
match 9:
    case 1 | 2:
        alternate = False
    case _:
        alternate = True
assert alternate
# ---
# case: sequence and mapping patterns
match [1, 2, 3, 4]:
    case [first, *middle, last]:
        sequence = (first, middle, last)
match {'name': 'bullsnake', 'version': 1, 'extra': 2}:
    case {'name': name, **rest}:
        mapping = (name, rest)
assert f'{sequence!r}' == "(1, [2, 3], 4)", "sequence"
assert f'{mapping!r}' == "('bullsnake', {'version': 1, 'extra': 2})", "mapping"
# ---
# case: class and as patterns
class Point:
    __match_args__ = ('x', 'y')
    def __init__(self, x, y):
        self.x = x
        self.y = y
point = Point(3, 4)
match point:
    case Point(x, y=4) as whole:
        class_result = (x, whole is point)
    case _:
        class_result = None
match 'text':
    case str(value):
        text_result = value
assert f'{class_result!r}' == "(3, True)", "class_result"
assert text_result == 'text'
