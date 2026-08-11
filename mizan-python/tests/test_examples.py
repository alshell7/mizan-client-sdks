import importlib.util
import os
import tempfile
import unittest
from pathlib import Path
from types import ModuleType
from unittest.mock import patch


def load_webhook_example(inbox_path: str) -> ModuleType:
    example = Path(__file__).parents[1] / "examples" / "webhook_receiver.py"
    spec = importlib.util.spec_from_file_location("mizan_webhook_receiver_example", example)
    if spec is None or spec.loader is None:
        raise RuntimeError("could not load webhook receiver example")
    module = importlib.util.module_from_spec(spec)
    with patch.dict(
        os.environ,
        {"MIZAN_WEBHOOK_TOKEN": "test-secret", "MIZAN_WEBHOOK_INBOX": inbox_path},
    ):
        spec.loader.exec_module(module)
    return module


class WebhookExampleTests(unittest.TestCase):
    def test_sqlite_inbox_deduplicates_both_identities(self):
        with tempfile.TemporaryDirectory() as directory:
            module = load_webhook_example(str(Path(directory) / "import.sqlite3"))
            inbox = module.SQLiteInbox(str(Path(directory) / "test.sqlite3"))
            record = {
                "outbox_id": "outbox-1",
                "stream": "ledger",
                "event_id": "ledger-1",
                "business_id": "business-1",
                "business_sequence": 7,
                "payload": b'{"event_id":"ledger-1"}',
            }
            inbox.persist(**record)
            inbox.persist(**record)

            event_retry = dict(record, outbox_id="outbox-2")
            inbox.persist(**event_retry)

            with self.assertRaises(RuntimeError):
                inbox.persist(**dict(record, event_id="ledger-2"))
            with self.assertRaises(RuntimeError):
                inbox.persist(**dict(event_retry, business_sequence=8))


if __name__ == "__main__":
    unittest.main()
