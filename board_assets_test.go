package main

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestValidateBoardImageUsesFileSignature(t *testing.T) {
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	if mimeType, err := validateBoardImage(png, "pixel.png"); err != nil || mimeType != "image/png" {
		t.Fatalf("PNG valido rifiutato: %q, %v", mimeType, err)
	}
	if _, err := validateBoardImage([]byte("<svg><script>alert(1)</script></svg>"), "immagine.svg"); err == nil {
		t.Fatal("SVG attivo accettato")
	}
	if _, err := validateBoardImage(png, "pixel.jpg"); err == nil {
		t.Fatal("estensione incoerente accettata")
	}
	if _, err := validateBoardImage([]byte(strings.Repeat("x", maxBoardImageBytes+1)), "grande.png"); err == nil {
		t.Fatal("immagine oltre limite accettata")
	}
}

func TestGetBoardAssetChecksProjectOwnership(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()
	first, _ := app.CreateProject("Primo", "")
	second, _ := app.CreateProject("Secondo", "")
	data := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}
	assetID := "0123456789abcdef0123456789abcdef"
	if _, err := app.db.Exec(`
		INSERT INTO board_assets (id, project_id, file_name, mime_type, data, size, sha256)
		VALUES (?, ?, 'test.png', 'image/png', ?, ?, 'hash')`, assetID, first, data, len(data)); err != nil {
		t.Fatal(err)
	}
	if _, err := app.GetBoardAsset(int(first), assetID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.GetBoardAsset(int(second), assetID); err == nil {
		t.Fatal("accesso immagine da un altro progetto consentito")
	}
}

func TestCleanupBoardAssetsKeepsOnlyReferencedImages(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()
	projectID, _ := app.CreateProject("Progetto", "")
	first := "0123456789abcdef0123456789abcdef"
	second := "abcdef0123456789abcdef0123456789"
	for index, assetID := range []string{first, second} {
		if _, err := app.db.Exec(`
			INSERT INTO board_assets (id, project_id, file_name, mime_type, data, size, sha256)
			VALUES (?, ?, ?, 'image/png', ?, 1, ?)`, assetID, projectID, fmt.Sprintf("%d.png", index), []byte{byte(index)}, fmt.Sprintf("hash-%d", index)); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.CleanupBoardAssets(int(projectID), []string{first}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := app.db.QueryRow(`SELECT COUNT(*) FROM board_assets WHERE project_id = ?`, projectID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("immagini rimaste = %d, atteso 1", count)
	}
	if _, err := app.GetBoardAsset(int(projectID), first); err != nil {
		t.Fatal("l'immagine referenziata è stata eliminata")
	}
}
