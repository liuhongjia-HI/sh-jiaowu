package store

import (
	"database/sql"
	"testing"
)

func TestNoticeExternalIDSchemaSupportsGeneratedHomeworkStationIDs(t *testing.T) {
	if mysqlNoticeExternalIDDefinition != "VARCHAR(191) NOT NULL DEFAULT ''" {
		t.Fatalf("notices.external_id definition = %q, want indexed VARCHAR(191) with empty default", mysqlNoticeExternalIDDefinition)
	}

	// This shape is produced by publishing a newly created homework task and
	// then duplicating the official-account record for the in-app station.
	baseID := "notice-homework-homework-20260813224749.829123000-stu-001"
	stationID := stationNoticeID(baseID)
	if len(stationID) <= 64 {
		t.Fatalf("test fixture must reproduce the legacy VARCHAR(64) overflow: length=%d id=%q", len(stationID), stationID)
	}
	if len(stationID) > mysqlIndexedExternalIDLength {
		t.Fatalf("station ID length=%d exceeds notices.external_id capacity=%d", len(stationID), mysqlIndexedExternalIDLength)
	}
}

func TestNoticeExternalIDColumnMigrationRecognizesLegacyAndCurrentDefinitions(t *testing.T) {
	tests := []struct {
		name         string
		columnType   string
		isNullable   string
		defaultValue sql.NullString
		wantModify   bool
	}{
		{
			name: "legacy varchar 64", columnType: "varchar(64)", isNullable: "NO",
			defaultValue: sql.NullString{String: "", Valid: true}, wantModify: true,
		},
		{
			name: "missing not null", columnType: "varchar(191)", isNullable: "YES",
			defaultValue: sql.NullString{String: "", Valid: true}, wantModify: true,
		},
		{
			name: "missing default", columnType: "varchar(191)", isNullable: "NO",
			defaultValue: sql.NullString{}, wantModify: true,
		},
		{
			name: "current definition", columnType: "VARCHAR(191)", isNullable: "no",
			defaultValue: sql.NullString{String: "", Valid: true}, wantModify: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := columnDefinitionNeedsMigration(tt.columnType, tt.isNullable, tt.defaultValue); got != tt.wantModify {
				t.Fatalf("columnDefinitionNeedsMigration(%q, %q, %#v) = %t, want %t", tt.columnType, tt.isNullable, tt.defaultValue, got, tt.wantModify)
			}
		})
	}
}
