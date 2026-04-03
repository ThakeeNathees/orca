"""Orca runtime support for inline block expressions.

Generated code calls these functions when a BlockExpression is used
as a value, e.g. model { provider = "openai" } becomes orca.model(provider="openai").
"""

from types import SimpleNamespace


def _block(kind: str, **kwargs) -> SimpleNamespace:
    """Create a block instance with the given kind and keyword fields."""
    return SimpleNamespace(_kind=kind, **kwargs)


def model(**kwargs) -> SimpleNamespace:
    return _block("model", **kwargs)


def agent(**kwargs) -> SimpleNamespace:
    return _block("agent", **kwargs)


def tool(**kwargs) -> SimpleNamespace:
    return _block("tool", **kwargs)


def knowledge(**kwargs) -> SimpleNamespace:
    return _block("knowledge", **kwargs)


def workflow(**kwargs) -> SimpleNamespace:
    return _block("workflow", **kwargs)


def input(**kwargs) -> SimpleNamespace:
    return _block("input", **kwargs)


def schema(**kwargs) -> SimpleNamespace:
    return _block("schema", **kwargs)


def let(**kwargs) -> SimpleNamespace:
    return _block("let", **kwargs)
