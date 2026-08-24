package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/audit"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/storage/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	_ audit.Appender = (*Store)(nil)
	_ audit.Reader   = (*Store)(nil)
	_ audit.Appender = auditQueryAppender{}
)

// auditedMutation is implemented by storage-owned domain adapters. The
// callback performs a mutation with generated queries and returns the event
// that must be appended before the same PostgreSQL transaction can commit.
type auditedMutation func(context.Context, *dbgen.Queries) (audit.Event, error)

// withAuditedMutation commits a mutation only when its audit event is valid and
// appended successfully in the same pgx transaction.
func (s *Store) withAuditedMutation(
	ctx context.Context,
	options pgx.TxOptions,
	mutation auditedMutation,
) (audit.Record, error) {
	if mutation == nil {
		return audit.Record{}, errors.New("audited mutation callback is required")
	}

	var record audit.Record
	err := s.WithTx(ctx, options, func(queries *dbgen.Queries) error {
		event, err := mutation(ctx, queries)
		if err != nil {
			return err
		}
		record, err = (auditQueryAppender{queries: queries}).Append(ctx, event)
		return err
	})
	if err != nil {
		return audit.Record{}, fmt.Errorf("apply audited mutation: %w", err)
	}
	return record, nil
}

// Append inserts one validated audit event using the Store's connection pool.
// The explicit transaction holds the query's shared ("GUAR", protocol v1)
// advisory lock from before identity allocation through commit. Storage-owned
// mutation adapters use withAuditedMutation when atomicity with a domain
// mutation is required.
func (s *Store) Append(ctx context.Context, event audit.Event) (audit.Record, error) {
	var record audit.Record
	err := s.WithTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(queries *dbgen.Queries) error {
		var err error
		record, err = (auditQueryAppender{queries: queries}).Append(ctx, event)
		return err
	})
	if err != nil {
		return audit.Record{}, err
	}
	return record, nil
}

type auditQueryAppender struct {
	queries *dbgen.Queries
}

func (a auditQueryAppender) Append(ctx context.Context, event audit.Event) (audit.Record, error) {
	if a.queries == nil {
		return audit.Record{}, errors.New("audit queries are required")
	}
	if err := event.Validate(); err != nil {
		return audit.Record{}, fmt.Errorf("validate audit event: %w", err)
	}

	params := dbgen.AppendAuditRecordParams{
		OccurredAt:     pgtype.Timestamptz{Time: event.OccurredAt, Valid: true},
		ActorType:      string(event.Actor.Type),
		ActorID:        event.Actor.ID,
		Action:         string(event.Action),
		ObjectType:     string(event.Object.Type),
		ObjectID:       event.Object.ID,
		CorrelationID:  event.CorrelationID,
		RequestID:      optionalText(event.RequestID),
		BeforeSnapshot: snapshotBytes(event.Before),
		AfterSnapshot:  snapshotBytes(event.After),
	}
	row, err := a.queries.AppendAuditRecord(ctx, params)
	if err != nil {
		return audit.Record{}, fmt.Errorf("append audit record: %w", err)
	}
	record, err := auditRecordFromRow(row)
	if err != nil {
		return audit.Record{}, fmt.Errorf("decode appended audit record: %w", err)
	}
	return record, nil
}

// List returns an anchor-stable newest-first keyset page.
func (s *Store) List(ctx context.Context, query audit.ListQuery) (audit.Page, error) {
	var err error
	query, err = query.Normalized()
	if err != nil {
		return audit.Page{}, fmt.Errorf("validate audit list query: %w", err)
	}

	params := dbgen.ListAuditRecordsParams{
		AnchorSequence:      query.Cursor.AnchorSequence,
		PositionSequence:    query.Cursor.PositionSequence,
		FilterAction:        string(query.Action),
		FilterCorrelationID: query.CorrelationID,
		FilterObjectType:    string(query.ObjectType),
		FilterObjectID:      query.ObjectID,
		PageSize:            query.Limit + 1,
	}
	var rows []dbgen.ListAuditRecordsRow
	if query.Cursor.IsZero() {
		params.AnchorSequence, rows, err = s.firstPageAuditRows(ctx, params)
	} else {
		rows, err = s.queries.ListAuditRecords(ctx, params)
	}
	if err != nil {
		return audit.Page{}, fmt.Errorf("list audit records: %w", err)
	}
	anchor := params.AnchorSequence

	hasMore := len(rows) > int(query.Limit)
	if hasMore {
		rows = rows[:query.Limit]
	}
	records := make([]audit.Record, 0, len(rows))
	for _, row := range rows {
		record, err := auditRecordFromListRow(row)
		if err != nil {
			return audit.Page{}, fmt.Errorf("decode listed audit record: %w", err)
		}
		records = append(records, record)
	}

	page := audit.Page{Records: records}
	if hasMore {
		cursor, err := audit.NewCursor(anchor, records[len(records)-1].Sequence)
		if err != nil {
			return audit.Page{}, fmt.Errorf("create next audit cursor: %w", err)
		}
		page.NextCursor, err = cursor.Encode()
		if err != nil {
			return audit.Page{}, fmt.Errorf("encode next audit cursor: %w", err)
		}
	}
	if err := page.Validate(); err != nil {
		return audit.Page{}, fmt.Errorf("validate audit page: %w", err)
	}
	return page, nil
}

// firstPageAuditRows takes the exclusive side of the ("GUAR", protocol v1)
// append/anchor advisory lock before issuing MAX and the bounded first-page
// read as subsequent READ COMMITTED statements in the same transaction. A
// snapshot established before waiting for the lock would still miss an append
// that commits during the wait. Context cancellation aborts lock acquisition;
// the background rollback then releases all transaction-level state even when
// the request context is already canceled.
func (s *Store) firstPageAuditRows(
	ctx context.Context,
	params dbgen.ListAuditRecordsParams,
) (int64, []dbgen.ListAuditRecordsRow, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return 0, nil, fmt.Errorf("begin audit first-page transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	queries := dbgen.New(tx)
	if err := queries.AcquireAuditAnchorLock(ctx); err != nil {
		return 0, nil, fmt.Errorf("acquire audit anchor lock: %w", err)
	}
	anchor, err := queries.GetAuditAnchor(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("select audit anchor: %w", err)
	}
	params.AnchorSequence = anchor
	rows, err := queries.ListAuditRecords(ctx, params)
	if err != nil {
		return 0, nil, fmt.Errorf("select audit first page: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, nil, fmt.Errorf("commit audit first-page transaction: %w", err)
	}
	return anchor, rows, nil
}

func auditRecordFromRow(row dbgen.AppendAuditRecordRow) (audit.Record, error) {
	return newAuditRecord(
		row.EventID,
		row.Sequence,
		row.OccurredAt,
		row.RecordedAt,
		row.ActorType,
		row.ActorID,
		row.Action,
		row.ObjectType,
		row.ObjectID,
		row.CorrelationID,
		row.RequestID,
		row.BeforeSnapshot,
		row.AfterSnapshot,
	)
}

func auditRecordFromListRow(row dbgen.ListAuditRecordsRow) (audit.Record, error) {
	return newAuditRecord(
		row.EventID,
		row.Sequence,
		row.OccurredAt,
		row.RecordedAt,
		row.ActorType,
		row.ActorID,
		row.Action,
		row.ObjectType,
		row.ObjectID,
		row.CorrelationID,
		row.RequestID,
		row.BeforeSnapshot,
		row.AfterSnapshot,
	)
}

func newAuditRecord(
	eventID string,
	sequence int64,
	occurredAt pgtype.Timestamptz,
	recordedAt pgtype.Timestamptz,
	actorType string,
	actorID string,
	action string,
	objectType string,
	objectID string,
	correlationID string,
	requestID pgtype.Text,
	beforeBytes []byte,
	afterBytes []byte,
) (audit.Record, error) {
	before, err := optionalSnapshot(beforeBytes)
	if err != nil {
		return audit.Record{}, fmt.Errorf("decode before snapshot: %w", err)
	}
	after, err := optionalSnapshot(afterBytes)
	if err != nil {
		return audit.Record{}, fmt.Errorf("decode after snapshot: %w", err)
	}

	record := audit.Record{
		EventID:    eventID,
		Sequence:   sequence,
		RecordedAt: recordedAt.Time,
		Event: audit.Event{
			OccurredAt: occurredAt.Time,
			Actor: audit.Actor{
				Type: audit.ActorType(actorType),
				ID:   actorID,
			},
			Action: audit.Action(action),
			Object: audit.ObjectRef{
				Type: audit.ObjectType(objectType),
				ID:   objectID,
			},
			CorrelationID: correlationID,
			RequestID:     requestID.String,
			Before:        before,
			After:         after,
		},
	}
	if err := record.Validate(); err != nil {
		return audit.Record{}, fmt.Errorf("validate stored audit record: %w", err)
	}
	return record, nil
}

func optionalText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func snapshotBytes(snapshot *audit.Snapshot) []byte {
	if snapshot == nil {
		return nil
	}
	return snapshot.Bytes()
}

func optionalSnapshot(raw []byte) (*audit.Snapshot, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	snapshot, err := audit.ParseSnapshot(raw)
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}
