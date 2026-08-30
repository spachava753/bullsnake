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
