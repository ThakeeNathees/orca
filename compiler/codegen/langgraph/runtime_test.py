"""Standalone Python tests for runtime.py helpers that don't need an LLM.

Executed by TestRuntimePython in langgraph_test.go. Kept self-contained —
only pydantic is required, and the test runner skips gracefully if the
embedded runtime.py or pydantic is unavailable.
"""

import importlib.util
import pathlib
import sys


def _load_runtime():
    """Load runtime.py without triggering its langchain import.

    runtime.py imports `from langchain.agents import create_agent` at module
    top. We only care about _orca__coerce_output_schema here, so we stub the
    langchain module before importing to avoid pulling the full dependency.
    """
    stub = type(sys)("langchain.agents")
    stub.create_agent = lambda *a, **kw: None
    sys.modules.setdefault("langchain", type(sys)("langchain"))
    sys.modules["langchain.agents"] = stub

    path = pathlib.Path(__file__).with_name("runtime.py")
    spec = importlib.util.spec_from_file_location("orca_runtime", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def main():
    from pydantic import BaseModel

    runtime = _load_runtime()
    coerce = runtime._orca__coerce_output_schema

    # Pydantic model passes through unchanged.
    class User(BaseModel):
        name: str
        age: int

    model, wrapped = coerce(User)
    assert model is User, "pydantic models should pass through"
    assert wrapped is False

    # Primitives get wrapped into a one-field model and validate correctly.
    for prim, value in [(str, "hi"), (int, 7), (float, 1.5), (bool, True)]:
        model, wrapped = coerce(prim)
        assert wrapped is True, f"{prim} should be wrapped"
        assert issubclass(model, BaseModel)
        instance = model(result=value)
        assert instance.result == value

    # Nested generic containers must round-trip through validation.
    cases = [
        (list[list[bool]], [[True, False], [False]]),
        (dict[str, int], {"a": 1, "b": 2}),
        (list[dict[str, int]], [{"x": 1}, {"y": 2}]),
        (dict[str, list[float]], {"a": [1.0, 2.0]}),
    ]
    for schema, value in cases:
        model, wrapped = coerce(schema)
        assert wrapped is True
        instance = model(result=value)
        assert instance.result == value

    # Pydantic model nested inside a container is still wrapped (the container
    # is not itself a BaseModel subclass).
    model, wrapped = coerce(list[User])
    assert wrapped is True
    instance = model(result=[User(name="a", age=1), User(name="b", age=2)])
    assert len(instance.result) == 2
    assert instance.result[0].name == "a"

    # Invalid value must raise pydantic ValidationError — proves the wrapper
    # enforces the declared type rather than silently passing anything through.
    from pydantic import ValidationError
    model, _ = coerce(list[int])
    try:
        model(result=["not", "ints"])
    except ValidationError:
        pass
    else:
        raise AssertionError("expected ValidationError for list[int] with strings")

    print("OK")


if __name__ == "__main__":
    main()
