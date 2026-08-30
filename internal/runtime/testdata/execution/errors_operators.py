# Runtime exception cases for operators.
# case: augmented zero division
# error: ZeroDivisionError
# message: "integer division or modulo by zero"
value = 1
value //= 0
# ---
# case: unsupported addition
# error: TypeError
# message: "unsupported operand type(s) for +: 'NoneType' and 'int'"
answer = None + 1
# ---
# case: floor division by zero
# error: ZeroDivisionError
# message: "integer division or modulo by zero"
answer = 1 // 0
# ---
# case: modulo by zero
# error: ZeroDivisionError
# message: "integer division or modulo by zero"
answer = 1 % 0
# ---
# case: negative left shift
# error: ValueError
# message: "negative shift count"
answer = 1 << -1
# ---
# case: negative right shift
# error: ValueError
# message: "negative shift count"
answer = 1 >> -1
# ---
# case: oversized left shift
# error: OverflowError
# message: "too many digits in integer"
answer = 1 << 10000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000
# ---
# case: unsupported shift count
# error: TypeError
# message: "unsupported operand type(s) for <<: 'int' and 'float'"
answer = 1 << 1.0
# ---
# case: unsupported subtraction
# error: TypeError
# message: "unsupported operand type(s) for -: 'int' and 'NoneType'"
answer = 1 - None
# ---
# case: unsupported bitwise and
# error: TypeError
# message: "unsupported operand type(s) for &: 'int' and 'float'"
answer = 1 & 1.0
# ---
# case: unsupported ordering
# error: TypeError
# message: "'<' not supported between instances of 'int' and 'str'"
answer = 1 < 'text'
# ---
# case: unsupported unary positive
# error: TypeError
# message: "bad operand type for unary +: 'str'"
answer = +'text'
# ---
# case: unsupported unary invert
# error: TypeError
# message: "bad operand type for unary ~: 'float'"
answer = ~1.5
# ---
# case: float division by zero
# error: ZeroDivisionError
# message: "division by zero"
answer = 1.0 / 0.0
# ---
# case: float modulo by zero
# error: ZeroDivisionError
# message: "division by zero"
answer = 1.0 % 0.0
# ---
# case: zero to negative integer power
# error: ZeroDivisionError
# message: "zero to a negative power"
answer = 0 ** -1
# ---
# case: oversized integer power
# error: OverflowError
# message: "integer power result exceeds 1048576-bit limit"
answer = 2 ** 1048576
# ---
# case: unsupported float power
# error: TypeError
# message: "unsupported operand type(s) for **: 'float' and 'int'"
answer = 2.0 ** 2
# ---
# case: unsupported power exponent
# error: TypeError
# message: "unsupported operand type(s) for **: 'int' and 'float'"
answer = 2 ** 2.0
