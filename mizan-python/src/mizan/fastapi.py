"""FastAPI integration for Mizan webhook receivers.

Install with ``pip install 'mizan-billing[fastapi]'``.
"""

from __future__ import annotations

from typing import Any, Callable

from fastapi import Request, Response

from .webhooks import WebhookReceiver


def webhook_endpoint(receiver: WebhookReceiver) -> Callable[[Request], Any]:
    """Create an async FastAPI endpoint for both webhook streams."""

    async def endpoint(request: Request) -> Response:
        response = await receiver.receive(request.headers, await request.body())
        return Response(
            content=response.body,
            status_code=response.status_code,
            headers=dict(response.headers),
        )

    return endpoint


def mount_webhooks(app: Any, receiver: WebhookReceiver, path: str = "/mizan/webhooks") -> None:
    """Mount the receiver on a FastAPI application or APIRouter."""

    app.add_api_route(path, webhook_endpoint(receiver), methods=["POST"], include_in_schema=False)

