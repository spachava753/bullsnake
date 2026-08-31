# case: class annotation format is limited
# error: NotImplementedError
# message: ""
class Model:
    value: int

Model.__annotate__(3)
# ---
# case: class annotations require dictionary result
# error: TypeError
# message: "__annotate__ returned non-dict of type 'int'"
class InvalidAnnotations:
    def __annotate__(format):
        return 42

InvalidAnnotations.__annotations__
# ---
# case: function annotation format is limited
# error: NotImplementedError
# message: ""
def described(value: int):
    return value

described.__annotate__(3)
