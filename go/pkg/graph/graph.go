// Package graph provides an entity-relation graph interface and a KV-backed
// implementation. Entities are identified by unique string labels and carry
// arbitrary key-value attributes. Relations connect two entities with a typed
// edge and are stored with forward and reverse indexes for efficient traversal.
package graph

import (
	"context"
	"errors"
	"iter"
)

// Sentinel errors.
var (
	// ErrNotFound is returned when an entity does not exist.
	ErrNotFound = errors.New("graph: not found")

	// ErrInvalidLabel is returned when a label or relation type contains
	// the KV separator character (default ':'), which would corrupt key
	// encoding. Labels and relation types are used as KV key segments
	// and must not contain the separator.
	ErrInvalidLabel = errors.New("graph: label contains separator")
)

// Entity is a node in the graph identified by a unique label.
type Entity struct {
	// Label is the unique identifier for this entity.
	Label string `json:"label"`

	// Attrs holds arbitrary key-value attributes associated with the entity.
	// Values must be JSON-serializable.
	Attrs map[string]any `json:"attrs,omitempty"`
}

// Relation is a typed directed edge between two entities.
type Relation struct {
	// From is the source entity label.
	From string `json:"from"`

	// To is the target entity label.
	To string `json:"to"`

	// RelType is the relation type (e.g., "knows", "works_at").
	RelType string `json:"rel_type"`
}

// Graph is the interface for an entity-relation graph.
type Graph interface {
	// --- Entity operations ---

	// GetEntity retrieves an entity by label. Returns ErrNotFound if not present.
	GetEntity(ctx context.Context, label string) (*Entity, error)

	// SetEntity creates or overwrites an entity. The Label field must be set.
	SetEntity(ctx context.Context, e Entity) error

	// DeleteEntity removes an entity and all its relations (both directions).
	DeleteEntity(ctx context.Context, label string) error

	// MergeAttrs merges the given attributes into an existing entity.
	// New keys are added; existing keys are overwritten. Keys not in attrs
	// are left unchanged. Returns ErrNotFound if the entity does not exist.
	MergeAttrs(ctx context.Context, label string, attrs map[string]any) error

	// ListEntities iterates over entities whose label starts with prefix.
	// Pass "" to list all entities.
	ListEntities(ctx context.Context, prefix string) iter.Seq2[Entity, error]

	// --- Relation operations ---

	// AddRelation creates a directed relation. If the same (from, to, relType)
	// already exists, this is a no-op.
	AddRelation(ctx context.Context, r Relation) error

	// RemoveRelation removes a specific relation. No error if it does not exist.
	RemoveRelation(ctx context.Context, from, to, relType string) error

	// Relations returns all relations where the given label is either the
	// source or target.
	Relations(ctx context.Context, label string) ([]Relation, error)

	// --- Traversal ---

	// Neighbors returns the labels of entities directly connected to the given
	// label. If relTypes is non-empty, only relations of those types are
	// considered. Returns labels from both directions (outgoing and incoming).
	Neighbors(ctx context.Context, label string, relTypes ...string) ([]string, error)

	// Expand performs a multi-hop breadth-first expansion from the given seed
	// labels, returning all discovered labels (including seeds). hops controls
	// the maximum traversal depth (0 returns only the seeds).
	Expand(ctx context.Context, labels []string, hops int) ([]string, error)
}
