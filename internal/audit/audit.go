// Package audit writes the append-only security audit trail (docs/security.md §8).
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"

	"reefcare/internal/db/dbq"
)

// Origin describes where a request came from. Handlers build it from the HTTP request;
// the IP must already be truncated (httpserver.TruncatedIP).
type Origin struct {
	RequestID string
	IPPrefix  *netip.Prefix
	UserAgent string
}

// Entry is one audit event. Metadata must never contain secrets or tokens.
type Entry struct {
	ActorID    *uuid.UUID
	Action     string // e.g. "auth.login"
	TargetType string
	TargetID   *uuid.UUID
	Metadata   map[string]any
}

// Record inserts e using q, normally inside the transaction of the audited change so
// the change and its audit record commit together.
func Record(ctx context.Context, q *dbq.Queries, now time.Time, o Origin, e Entry) error {
	var meta []byte
	if len(e.Metadata) > 0 {
		var err error
		if meta, err = json.Marshal(e.Metadata); err != nil {
			return fmt.Errorf("encode audit metadata: %w", err)
		}
	}
	var targetType, requestID *string
	if e.TargetType != "" {
		targetType = &e.TargetType
	}
	if o.RequestID != "" {
		requestID = &o.RequestID
	}
	if err := q.InsertAuditLog(ctx, dbq.InsertAuditLogParams{
		ActorID: e.ActorID, Action: e.Action, TargetType: targetType, TargetID: e.TargetID,
		Metadata: meta, RequestID: requestID, IpPrefix: o.IPPrefix, Now: now,
	}); err != nil {
		return fmt.Errorf("record audit %s: %w", e.Action, err)
	}
	return nil
}
