package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyDatabaseCopiesAndPreservesOriginal(t *testing.T) {
	legacyDir := t.TempDir()
	destinationDir := t.TempDir()
	legacyPath := filepath.Join(legacyDir, databaseFileName)
	destinationPath := filepath.Join(destinationDir, databaseFileName)

	if err := os.WriteFile(legacyPath, []byte("database-v1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath+"-wal", []byte("wal-data"), 0600); err != nil {
		t.Fatal(err)
	}

	migrated, err := migrateLegacyDatabase(legacyPath, destinationPath)
	if err != nil {
		t.Fatalf("migrazione fallita: %v", err)
	}
	if !migrated {
		t.Fatal("migrazione non eseguita")
	}

	assertFileContent(t, destinationPath, "database-v1")
	assertFileContent(t, destinationPath+"-wal", "wal-data")
	assertFileContent(t, legacyPath, "database-v1")

	if err := os.WriteFile(legacyPath, []byte("database-v2"), 0600); err != nil {
		t.Fatal(err)
	}
	migrated, err = migrateLegacyDatabase(legacyPath, destinationPath)
	if err != nil {
		t.Fatal(err)
	}
	if migrated {
		t.Fatal("un database già presente non deve essere sovrascritto")
	}
	assertFileContent(t, destinationPath, "database-v1")
}

func assertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lettura %s: %v", path, err)
	}
	if string(content) != expected {
		t.Fatalf("contenuto %s = %q, atteso %q", path, content, expected)
	}
}
