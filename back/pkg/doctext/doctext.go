package doctext

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

var ErrUnsupported = errors.New("unsupported file type: use .txt, .pdf or .docx")

func Extract(name string, data []byte) (string, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".txt", ".md", "":
		if !utf8.Valid(data) {
			return "", errors.New("text file is not valid UTF-8")
		}
		return string(bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})), nil
	case ".docx":
		return docx(data)
	case ".pdf":
		return pdfText(data)
	}
	return "", ErrUnsupported
}

func docx(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open docx body: %w", err)
		}
		defer func() { _ = rc.Close() }()
		return docxBody(rc)
	}
	return "", errors.New("docx has no document body")
}

func docxBody(r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	var b strings.Builder
	inText := false
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return b.String(), nil
		}
		if err != nil {
			return "", fmt.Errorf("parse docx: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				b.WriteString("\t")
			case "br":
				b.WriteString("\n")
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				b.WriteString("\n\n")
			}
		case xml.CharData:
			if inText {
				b.Write(t)
			}
		}
	}
}

func pdfText(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	var b strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		rows, err := page.GetTextByRow()
		if err != nil {
			return "", fmt.Errorf("read pdf page %d: %w", i, err)
		}
		for _, row := range rows {
			for _, word := range row.Content {
				b.WriteString(word.S)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}
