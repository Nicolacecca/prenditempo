package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeBoardVideoPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dimostrazione.MP4")
	if err := os.WriteFile(path, []byte("video di prova"), 0600); err != nil {
		t.Fatal(err)
	}
	video, err := normalizeBoardVideoPath(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if video.Path != filepath.Clean(path) || video.FileName != "dimostrazione.MP4" || video.Size == 0 {
		t.Fatalf("video normalizzato in modo inatteso: %+v", video)
	}
}

func TestNormalizeBoardVideoPathRejectsUnsafeValues(t *testing.T) {
	for _, value := range []string{
		`video.mp4`,
		`C:\documento.exe`,
		`\\?\C:\video.mp4`,
		`\\.\PhysicalDrive0.mp4`,
	} {
		if _, err := normalizeBoardVideoPath(value, false); err == nil {
			t.Fatalf("percorso non sicuro accettato: %q", value)
		}
	}
}
