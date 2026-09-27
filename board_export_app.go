package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxBoardPDFImageBytes = 40 * 1024 * 1024

func decodeBoardExportDataURL(dataURL, prefix, mimeType string) ([]byte, error) {
	if !strings.HasPrefix(dataURL, prefix) || len(dataURL) > base64.StdEncoding.EncodedLen(maxBoardPDFImageBytes)+len(prefix) {
		return nil, fmt.Errorf("dati immagine non validi")
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, prefix))
	if err != nil || len(data) == 0 || len(data) > maxBoardPDFImageBytes || http.DetectContentType(data) != mimeType {
		return nil, fmt.Errorf("dati immagine non validi")
	}
	return data, nil
}

func buildBoardPDF(jpegData []byte) ([]byte, error) {
	if len(jpegData) == 0 || len(jpegData) > maxBoardPDFImageBytes {
		return nil, fmt.Errorf("immagine PDF vuota o troppo grande")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(jpegData))
	if err != nil || format != "jpeg" || config.Width <= 0 || config.Height <= 0 || config.Width > 20000 || config.Height > 20000 {
		return nil, fmt.Errorf("immagine JPEG non valida")
	}

	pageWidth, pageHeight := 595.28, 841.89
	if config.Width > config.Height {
		pageWidth, pageHeight = pageHeight, pageWidth
	}
	const margin = 24.0
	scale := min((pageWidth-margin*2)/float64(config.Width), (pageHeight-margin*2)/float64(config.Height))
	drawWidth := float64(config.Width) * scale
	drawHeight := float64(config.Height) * scale
	x := (pageWidth - drawWidth) / 2
	y := (pageHeight - drawHeight) / 2
	content := fmt.Sprintf("q %.3f 0 0 %.3f %.3f %.3f cm /Im0 Do Q\n", drawWidth, drawHeight, x, y)

	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>"),
		[]byte(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources << /XObject << /Im0 4 0 R >> >> /Contents 5 0 R >>", pageWidth, pageHeight)),
		append([]byte(fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n", config.Width, config.Height, len(jpegData))), append(jpegData, []byte("\nendstream")...)...),
		[]byte(fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content)),
	}

	var output bytes.Buffer
	output.WriteString("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = output.Len()
		fmt.Fprintf(&output, "%d 0 obj\n", index+1)
		output.Write(object)
		output.WriteString("\nendobj\n")
	}
	xrefOffset := output.Len()
	fmt.Fprintf(&output, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for index := 1; index <= len(objects); index++ {
		fmt.Fprintf(&output, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(&output, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefOffset)
	return output.Bytes(), nil
}

func (a *App) SaveBoardPDF(dataURL, suggestedName string) (string, error) {
	const prefix = "data:image/jpeg;base64,"
	jpegData, err := decodeBoardExportDataURL(dataURL, prefix, "image/jpeg")
	if err != nil {
		return "", err
	}
	pdfData, err := buildBoardPDF(jpegData)
	if err != nil {
		return "", err
	}
	name := filepath.Base(strings.TrimSpace(suggestedName))
	if name == "." || name == "" {
		name = "lavagna.pdf"
	}
	if !strings.EqualFold(filepath.Ext(name), ".pdf") {
		name += ".pdf"
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Esporta lavagna in PDF",
		DefaultFilename: name,
		Filters: []wailsruntime.FileFilter{{
			DisplayName: "Documento PDF (*.pdf)",
			Pattern:     "*.pdf",
		}},
	})
	if err != nil || path == "" {
		return path, err
	}
	if !strings.EqualFold(filepath.Ext(path), ".pdf") {
		path += ".pdf"
	}
	if err := os.WriteFile(path, pdfData, 0600); err != nil {
		return "", fmt.Errorf("impossibile salvare il PDF: %w", err)
	}
	return path, nil
}

func (a *App) SaveBoardPNG(dataURL, suggestedName string) (string, error) {
	const prefix = "data:image/png;base64,"
	pngData, err := decodeBoardExportDataURL(dataURL, prefix, "image/png")
	if err != nil {
		return "", err
	}
	name := filepath.Base(strings.TrimSpace(suggestedName))
	if name == "." || name == "" {
		name = "lavagna.png"
	}
	if !strings.EqualFold(filepath.Ext(name), ".png") {
		name += ".png"
	}
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Esporta lavagna in PNG",
		DefaultFilename: name,
		Filters: []wailsruntime.FileFilter{{
			DisplayName: "Immagine PNG (*.png)",
			Pattern:     "*.png",
		}},
	})
	if err != nil || path == "" {
		return path, err
	}
	if !strings.EqualFold(filepath.Ext(path), ".png") {
		path += ".png"
	}
	if err := os.WriteFile(path, pngData, 0600); err != nil {
		return "", fmt.Errorf("impossibile salvare il PNG: %w", err)
	}
	return path, nil
}
