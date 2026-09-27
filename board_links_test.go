package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeBoardWebURL(t *testing.T) {
	valid, err := normalizeBoardWebURL(" https://example.com/documento?id=2 ")
	if err != nil || valid != "https://example.com/documento?id=2" {
		t.Fatalf("URL valido rifiutato: %q, %v", valid, err)
	}
	if valid, err := normalizeBoardWebURL("example.com/documento"); err != nil || valid != "https://example.com/documento" {
		t.Fatalf("URL senza schema non normalizzato: %q, %v", valid, err)
	}
	for _, value := range []string{
		"javascript:alert(1)",
		"file:///C:/segreto.txt",
		"https://utente:password@example.com",
		"file:segreto.txt",
	} {
		if _, err := normalizeBoardWebURL(value); err == nil {
			t.Fatalf("URL pericoloso o incompleto accettato: %q", value)
		}
	}
}

func TestNormalizeBoardFolderPath(t *testing.T) {
	local := filepath.Join(t.TempDir(), "cartella")
	if normalized, err := normalizeBoardFolderPath(local, false); err != nil || normalized != filepath.Clean(local) {
		t.Fatalf("percorso locale valido rifiutato: %q, %v", normalized, err)
	}
	if _, err := normalizeBoardFolderPath(`\\server\condivisione\team`, true); err != nil {
		t.Fatalf("percorso UNC valido rifiutato: %v", err)
	}
	if _, err := normalizeBoardFolderPath(`\\server`, true); err != nil {
		t.Fatalf("radice server/NAS valida rifiutata: %v", err)
	}
	for _, value := range []string{`cartella\relativa`, `\\`, `\\?\C:\Windows`, `\\.\PhysicalDrive0`} {
		if _, err := normalizeBoardFolderPath(value, strings.HasPrefix(value, `\\`)); err == nil {
			t.Fatalf("percorso non sicuro accettato: %q", value)
		}
	}
}
