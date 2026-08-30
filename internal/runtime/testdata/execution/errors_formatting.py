# Runtime exception cases for formatting.
# case: unknown float format code
# error: ValueError
# message: "Unknown format code 'q' for object of type 'float'"
value = f'{1.5:q}'
# ---
# case: integer precision
# error: ValueError
# message: "Precision not allowed in integer format specifier"
value = f'{1:.2d}'
# ---
# case: unknown integer format code
# error: ValueError
# message: "Unknown format code 'q' for object of type 'int'"
value = f'{1:q}'
# ---
# case: integer character range
# error: OverflowError
# message: "%c arg not in range(0x110000)"
value = f'{-1:c}'
# ---
# case: string sign option
# error: ValueError
# message: "Sign not allowed in string format specifier"
value = f"{'x':+5}"
# ---
# case: unknown string format code
# error: ValueError
# message: "Unknown format code 'q' for object of type 'str'"
value = f"{'x':q}"
