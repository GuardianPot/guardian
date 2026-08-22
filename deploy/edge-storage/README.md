# Edge durable storage fixture

The Edge agent stores its local delivery queue in SQLite with WAL journaling
and `synchronous=FULL`. The queue uses stable event IDs, durable lease state,
and a delivered tombstone so a replayed or retried event cannot be inserted a
second time.

Run the repeatable crash/restart evidence fixture from the repository root:

```powershell
task edge:wal
```

The fixture deliberately exits the first process after a committed claim. A
second process waits for the lease to expire, reclaims the event, acknowledges
it, and verifies that the final state has exactly one delivered row and no
pending or in-flight rows. It creates only a temporary database and removes it
after the run.
