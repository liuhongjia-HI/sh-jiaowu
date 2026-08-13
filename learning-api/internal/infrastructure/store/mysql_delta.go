package store

import (
	"database/sql"
	"fmt"
	"reflect"
	"sort"
)

type persistenceRow struct {
	key        string
	deleteSQL  string
	deleteArgs []any
	upsertSQL  string
	upsertArgs []any
}

func persistStateDeltaTx(tx *sql.Tx, before, after *MemoryStore) error {
	builders := []func(*MemoryStore) []persistenceRow{
		identityRows,
		packageRows,
		contentRows,
		schedulingRows,
		commercialRows,
		engagementRows,
	}
	for _, build := range builders {
		if err := syncPersistenceRows(tx, build(before), build(after)); err != nil {
			return err
		}
	}
	return syncGrantPersistence(tx, before, after)
}

func syncPersistenceRows(tx *sql.Tx, beforeRows, afterRows []persistenceRow) error {
	before := make(map[string]persistenceRow, len(beforeRows))
	after := make(map[string]persistenceRow, len(afterRows))
	for _, row := range beforeRows {
		before[row.key] = row
	}
	for _, row := range afterRows {
		after[row.key] = row
	}
	removed := make([]string, 0)
	changed := make([]string, 0)
	for key := range before {
		if _, ok := after[key]; !ok {
			removed = append(removed, key)
		}
	}
	for key, row := range after {
		old, ok := before[key]
		if !ok || !reflect.DeepEqual(old.upsertArgs, row.upsertArgs) {
			changed = append(changed, key)
		}
	}
	sort.Strings(removed)
	sort.Strings(changed)
	for _, key := range removed {
		row := before[key]
		if _, err := tx.Exec(row.deleteSQL, row.deleteArgs...); err != nil {
			return fmt.Errorf("delete persistence row %s: %w", key, err)
		}
	}
	for _, key := range changed {
		row := after[key]
		if _, err := tx.Exec(row.upsertSQL, row.upsertArgs...); err != nil {
			return fmt.Errorf("upsert persistence row %s: %w", key, err)
		}
	}
	return nil
}

func rowKey(table string, values ...any) string {
	return fmt.Sprintf("%s:%q", table, values)
}

func simpleRow(table, keyColumn string, keyValue any, upsertSQL string, args ...any) persistenceRow {
	return persistenceRow{
		key: rowKey(table, keyValue), deleteSQL: "DELETE FROM " + table + " WHERE " + keyColumn + " = ?", deleteArgs: []any{keyValue},
		upsertSQL: upsertSQL, upsertArgs: args,
	}
}

func relationRow(table string, columns []string, values []any, upsertSQL string, args ...any) persistenceRow {
	where := ""
	for index, column := range columns {
		if index > 0 {
			where += " AND "
		}
		where += column + " = ?"
	}
	return persistenceRow{
		key: rowKey(table, values...), deleteSQL: "DELETE FROM " + table + " WHERE " + where, deleteArgs: values,
		upsertSQL: upsertSQL, upsertArgs: args,
	}
}
