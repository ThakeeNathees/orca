"""Orca runtime support for inline block expressions.

Generated code calls these functions when a BlockExpression is used
as a value, e.g. model { provider = "openai" } becomes orca.model(provider="openai").
"""

from __future__ import annotations

from types import SimpleNamespace
from typing import Any


def _block(kind: str, **kwargs: Any) -> SimpleNamespace:
    """Create a block instance with the given kind and keyword fields."""
    return SimpleNamespace(_kind=kind, **kwargs)


def _fields(params: dict[str, Any]) -> dict[str, Any]:
    """Filter out None values from a parameter dict to omit unset optional fields."""
    return {k: v for k, v in params.items() if v is not None}


def meta(name: str, *args: Any) -> SimpleNamespace:
    """Single Orca decorator as data (@name or @name(args...))."""
    return SimpleNamespace(_kind="meta", name=name, args=args)


def with_meta(value: Any, metas: list[Any]) -> SimpleNamespace:
    """Attach a non-empty list of meta() values to a block or field value."""
    return SimpleNamespace(_kind="with_meta", value=value, metas=metas)


def model(
    provider: str,
    model_name: str | SimpleNamespace,
    api_key: str | None = None,
    base_url: str | None = None,
    temperature: float | None = None,
) -> SimpleNamespace:
    return _block("model", **_fields(locals()))


def agent(
    model: str | SimpleNamespace,
    persona: str,
    tools: list[SimpleNamespace] | None = None,
    output_schema: SimpleNamespace | None = None,
) -> SimpleNamespace:
    return _block("agent", **_fields(locals()))


def tool(
    invoke: str | callable,
    desc: str | None = None,
    input_schema: SimpleNamespace | None = None,
    output_schema: SimpleNamespace | None = None,
) -> SimpleNamespace:
    return _block("tool", **_fields(locals()))


def knowledge(
    desc: str | None = None,
) -> SimpleNamespace:
    return _block("knowledge", **_fields(locals()))


def workflow(
    name: str | None = None,
    desc: str | None = None,
) -> SimpleNamespace:
    return _block("workflow", **_fields(locals()))


def cron(
    schedule: str,
    timezone: str | None = None,
) -> SimpleNamespace:
    return _block("cron", **_fields(locals()))


def webhook(
    path: str,
    method: str | None = None,
) -> SimpleNamespace:
    return _block("webhook", **_fields(locals()))


def input(
    type: SimpleNamespace,
    desc: str | None = None,
    default: Any | None = None,
    sensitive: bool | None = None,
) -> SimpleNamespace:
    return _block("input", **_fields(locals()))


def schema(**kwargs: Any) -> SimpleNamespace:
    return _block("schema", **kwargs)


def let(**kwargs: Any) -> SimpleNamespace:
    return _block("let", **kwargs)
