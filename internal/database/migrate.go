package database

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"
)

// Embed SQL schema files into the binary
//
//go:embed schema/*.sql
var schemaFiles embed.FS

// runMigrations executes SQL migration files in order
// Migration files are executed in alphabetical order (001_, 002_, 003_...)
func (db *DB) runMigrations(ctx context.Context) error {
	entries, err := schemaFiles.ReadDir("schema")
	if err != nil {
		return fmt.Errorf("failed to read schema directory: %w", err)
	}

	// Collect and sort SQL files
	var fileNames []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			fileNames = append(fileNames, entry.Name())
		}
	}
	sort.Strings(fileNames) // Execute in order: 001_, 002_, 003_

	// Execute each migration file
	for _, fileName := range fileNames {
		content, err := schemaFiles.ReadFile("schema/" + fileName)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", fileName, err)
		}

		if _, err := db.db.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("failed to execute %s: %w", fileName, err)
		}

		db.logger.System().WithField("file", fileName).Info("Migration executed")
	}

	return nil
}
