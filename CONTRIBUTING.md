# Contributing

The SDKs intentionally contain no billing calculations. The Worker is authoritative for prices,
eligibility, allocation, tax, and budgets. Preserve exact amounts as strings and retry mutations only with
the same idempotency key and identical body.

Run both release gates before submitting changes:

```bash
cd mizan-python
PYTHONPATH=src python -m unittest discover -s tests -v
python -m build

cd ../mizan-go
go test -race ./...
go vet ./...
```

