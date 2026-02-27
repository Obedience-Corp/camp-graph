package graph

import (
	"context"
	"encoding/json"
	"fmt"
)

// SaveGraph writes the entire graph to the store, replacing existing data.
// The operation is wrapped in a transaction for atomicity.
func SaveGraph(ctx context.Context, store *Store, g *Graph) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM edges"); err != nil {
		return fmt.Errorf("delete edges: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM nodes"); err != nil {
		return fmt.Errorf("delete nodes: %w", err)
	}

	for _, n := range g.Nodes() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		metaJSON, _ := json.Marshal(n.Metadata)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO nodes (id, type, name, path, status, metadata, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			n.ID, string(n.Type), n.Name, n.Path, n.Status, string(metaJSON), n.CreatedAt, n.UpdatedAt,
		); err != nil {
			return fmt.Errorf("insert node %s: %w", n.ID, err)
		}
	}

	for _, e := range g.Edges() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO edges (from_id, to_id, type, confidence, source, subtype, note, created_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			e.FromID, e.ToID, string(e.Type), e.Confidence, string(e.Source), e.Subtype, e.Note, e.CreatedAt,
		); err != nil {
			return fmt.Errorf("insert edge %s->%s: %w", e.FromID, e.ToID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// LoadGraph reads all nodes and edges from the store into a new in-memory Graph.
func LoadGraph(ctx context.Context, store *Store) (*Graph, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	g := New()

	nodes, err := store.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("load nodes: %w", err)
	}
	for _, n := range nodes {
		g.AddNode(n)
	}

	edges, err := store.GetAllEdges(ctx)
	if err != nil {
		return nil, fmt.Errorf("load edges: %w", err)
	}
	for _, e := range edges {
		g.AddEdge(e)
	}
	return g, nil
}
