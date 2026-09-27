package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestBuildBoardPDF(t *testing.T) {
	jpegData, err := base64.StdEncoding.DecodeString("/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////2wBDAf//////////////////////////////////////////////////////////////////////////////////////wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAf/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIQAxAAAAF//8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABBQJ//8QAFBEBAAAAAAAAAAAAAAAAAAAAAP/aAAgBAwEBPwF//8QAFBEBAAAAAAAAAAAAAAAAAAAAAP/aAAgBAgEBPwF//8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQAGPwJ//8QAFBABAAAAAAAAAAAAAAAAAAAAAP/aAAgBAQABPyF//9oADAMBAAIAAwAAABD/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oACAEDAQE/EB//xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oACAECAQE/EB//xAAUEAEAAAAAAAAAAAAAAAAAAAAA/9oACAEBAAE/EB//2Q==")
	if err != nil {
		t.Fatal(err)
	}
	pdf, err := buildBoardPDF(jpegData)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) || !bytes.Contains(pdf, []byte("/Subtype /Image")) || !bytes.HasSuffix(pdf, []byte("%%EOF\n")) {
		t.Fatal("PDF generato non valido")
	}
}

func TestBuildBoardPDFRejectsInvalidImage(t *testing.T) {
	if _, err := buildBoardPDF([]byte("non una jpeg")); err == nil {
		t.Fatal("immagine non valida accettata")
	}
}

func TestDecodeBoardExportDataURLValidatesSignature(t *testing.T) {
	png := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="
	if _, err := decodeBoardExportDataURL("data:image/png;base64,"+png, "data:image/png;base64,", "image/png"); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeBoardExportDataURL("data:image/png;base64,"+base64.StdEncoding.EncodeToString([]byte("falso")), "data:image/png;base64,", "image/png"); err == nil {
		t.Fatal("firma PNG non valida accettata")
	}
}
