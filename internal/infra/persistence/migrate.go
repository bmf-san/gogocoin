package persistence

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed schema/*.sql
var schemaFiles embed.FS

// runMigrations executes schema SQL files in alphabetical order.
// Files must use CREATE TABLE/INDEX IF NOT EXISTS for idempotency.
func (d *DB) runMigrations(ctx context.Context) error {
	entries, err := schemaFiles.ReadDir("schema")
	if err != nil {
		return fmt.Errorf("failed to read schema directory: %w", err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		content, err := schemaFiles.ReadFile("schema/" + name)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", name, err)
		}
		if _, err := d.db.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("failed to execute %s: %w", name, err)
		}
		d.logger.System().WithField("file", name).Info("Migration executed")
	}
	return nil
}
