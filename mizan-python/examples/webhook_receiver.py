"""FastAPI webhook receiver with a durable, idempotent SQLite inbox.

The validated event is committed before the SDK returns HTTP 204 and, for a
ledger event, ``X-Mizan-Ack-Sequence``. A downstream worker may project the
durable inbox asynchronously. Use a shared transactional database instead of
SQLite when the receiver runs in multiple replicas.
"""

from __future__ import annotations

import os
import sqlite3
from contextlib import closing
from pathlib import Path

from fastapi import FastAPI

from mizan import LedgerWebhook, NotificationWebhook, WebhookContext, WebhookReceiver
from mizan.fastapi import mount_webhooks


class SQLiteInbox:
    """Durably deduplicate deliveries by both outbox and event identity."""

    def __init__(self, path: str) -> None:
        self.path = path
        Path(path).parent.mkdir(parents=True, exist_ok=True)
        with closing(self._connect()) as connection:
            with connection:
                connection.execute(
                    """
                    CREATE TABLE IF NOT EXISTS mizan_webhook_inbox (
                        outbox_id TEXT PRIMARY KEY,
                        stream TEXT NOT NULL,
                        event_id TEXT NOT NULL,
                        business_id TEXT NOT NULL,
                        business_sequence INTEGER,
                        payload BLOB NOT NULL,
                        received_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
                        UNIQUE (stream, event_id)
                    )
                    """
                )

    def persist(
        self,
        *,
        outbox_id: str,
        stream: str,
        event_id: str,
        business_id: str,
        business_sequence: int | None,
        payload: bytes,
    ) -> None:
        """Commit the durable application effect or validate an exact retry."""
        with closing(self._connect()) as connection:
            with connection:
                try:
                    connection.execute(
                        """
                        INSERT INTO mizan_webhook_inbox
                            (outbox_id, stream, event_id, business_id, business_sequence, payload)
                        VALUES (?, ?, ?, ?, ?, ?)
                        """,
                        (
                            outbox_id,
                            stream,
                            event_id,
                            business_id,
                            business_sequence,
                            sqlite3.Binary(payload),
                        ),
                    )
                except sqlite3.IntegrityError:
                    existing_outbox = connection.execute(
                        """
                        SELECT stream, event_id, business_id, business_sequence, payload
                        FROM mizan_webhook_inbox
                        WHERE outbox_id = ?
                        """,
                        (outbox_id,),
                    ).fetchone()
                    identity = (stream, event_id, business_id, business_sequence, payload)
                    if existing_outbox is not None:
                        if existing_outbox == identity:
                            return
                        # Raising makes the receiver return 500 without a ledger
                        # acknowledgement, surfacing a dangerous identity collision.
                        raise RuntimeError("webhook identity was reused for a different event")
                    existing_event = connection.execute(
                        """
                        SELECT stream, event_id, business_id, business_sequence, payload
                        FROM mizan_webhook_inbox
                        WHERE stream = ? AND event_id = ?
                        """,
                        (stream, event_id),
                    ).fetchone()
                    if existing_event != identity:
                        raise RuntimeError("webhook identity was reused for a different event")
                    # A repeated event under another outbox ID is also already
                    # durably accepted. The unique event index is the second
                    # deduplication boundary.

    def _connect(self) -> sqlite3.Connection:
        connection = sqlite3.connect(self.path, timeout=10)
        connection.execute("PRAGMA journal_mode = WAL")
        connection.execute("PRAGMA synchronous = FULL")
        return connection


inbox = SQLiteInbox(os.getenv("MIZAN_WEBHOOK_INBOX", "./data/mizan-webhooks.sqlite3"))


def on_ledger(event: LedgerWebhook, delivery: WebhookContext) -> None:
    inbox.persist(
        outbox_id=delivery.outbox_id,
        stream="ledger",
        event_id=event["event_id"],
        business_id=event["business_id"],
        business_sequence=event["business_sequence"],
        payload=delivery.raw_body,
    )


def on_notification(event: NotificationWebhook, delivery: WebhookContext) -> None:
    # Notifications have no separate event_id; the stable outbox ID is their
    # durable delivery identity.
    inbox.persist(
        outbox_id=delivery.outbox_id,
        stream="notification",
        event_id=delivery.outbox_id,
        business_id=event["business_id"],
        business_sequence=None,
        payload=delivery.raw_body,
    )


receiver = WebhookReceiver(
    on_ledger=on_ledger,
    on_notification=on_notification,
    bearer_token=os.environ["MIZAN_WEBHOOK_TOKEN"],
)

app = FastAPI(title="Mizan webhook receiver")
mount_webhooks(app, receiver, path="/mizan/webhooks")
