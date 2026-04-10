package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func ApplyMigrations(databaseURL string, migrationsDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(files))
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(file.Name()), ".sql") {
			names = append(names, file.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		content, readErr := os.ReadFile(filepath.Join(migrationsDir, name))
		if readErr != nil {
			return readErr
		}
		sql := strings.TrimSpace(string(content))
		if sql == "" {
			continue
		}
		if _, execErr := pool.Exec(ctx, sql); execErr != nil {
			return fmt.Errorf("migration %s failed: %w", name, execErr)
		}
	}
	return nil
}
