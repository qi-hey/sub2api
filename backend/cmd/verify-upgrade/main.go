// Command verify-upgrade rehearses embedded migrations on a restored database.
// The database-name guard prevents accidental use against the live database.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/repository"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if len(os.Args) != 2 || !regexp.MustCompile(`^sub2api_upgrade_verify_[a-z0-9_]+$`).MatchString(os.Args[1]) {
		log.Fatal("expected a restored database named sub2api_upgrade_verify_<suffix>")
	}
	db, err := sql.Open("pgx", "host=/var/run/postgresql user=postgres sslmode=disable dbname="+os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	before, err := protectedDigest(ctx, db)
	if err != nil {
		log.Fatal(err)
	}
	var countBefore, countAfter int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&countBefore); err != nil {
		log.Fatal(err)
	}
	if err := repository.ApplyMigrations(ctx, db); err != nil {
		log.Fatal(err)
	}
	after, err := protectedDigest(ctx, db)
	if err != nil {
		log.Fatal(err)
	}
	if before != after {
		log.Fatal("migration changed protected account, group, key, user, or game data")
	}
	if err := repository.ApplyMigrations(ctx, db); err != nil {
		log.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&countAfter); err != nil {
		log.Fatal(err)
	}
	result, _ := json.Marshal(map[string]any{
		"migrations_before": countBefore, "migrations_after": countAfter,
		"protected_data_sha256": after, "idempotent": true,
	})
	fmt.Println(string(result))
}

func protectedDigest(ctx context.Context, db *sql.DB) (string, error) {
	tables := []string{
		"accounts", "account_groups", "api_keys", "api_key_groups", "users", "settings",
	}
	gameTables, err := db.QueryContext(ctx, "SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename LIKE 'game\\_%' ORDER BY tablename")
	if err != nil {
		return "", err
	}
	for gameTables.Next() {
		var table string
		if err := gameTables.Scan(&table); err != nil {
			gameTables.Close()
			return "", err
		}
		if !regexp.MustCompile(`^game_[a-z0-9_]+$`).MatchString(table) {
			gameTables.Close()
			return "", fmt.Errorf("unexpected game table name")
		}
		tables = append(tables, table)
	}
	err = gameTables.Err()
	gameTables.Close()
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	for _, table := range tables {
		var exists bool
		if err := db.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", "public."+table).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			continue
		}
		// Hash only; credentials and private settings never leave this process.
		rows, err := db.QueryContext(ctx, "SELECT to_jsonb(t)::text FROM "+table+" t ORDER BY to_jsonb(t)::text")
		if err != nil {
			return "", err
		}
		fmt.Fprintln(hash, table)
		for rows.Next() {
			var raw string
			if err := rows.Scan(&raw); err != nil {
				rows.Close()
				return "", err
			}
			fmt.Fprintln(hash, raw)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return "", err
		}
	}
	rows, err := db.QueryContext(ctx, "SELECT (to_jsonb(g)-'i18n')::text FROM groups g ORDER BY id")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	fmt.Fprintln(hash, "groups")
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return "", err
		}
		fmt.Fprintln(hash, raw)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
