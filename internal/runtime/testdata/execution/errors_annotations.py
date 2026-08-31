# case: class annotation format is limited
# error: NotImplementedError
# message: ""
class Model:
    value: int

Model.__annotate__(3)
