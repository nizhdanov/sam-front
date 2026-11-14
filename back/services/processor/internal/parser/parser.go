package parser

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"sam/pkg/swfilter"
	"sam/services/processor/internal/domain"
)

const (
	ColumnSerial    = "COMPUTERS.SERIAL_NUMBER"
	ColumnName      = "SOFTWARE.DISPLAYNAME"
	ColumnPublisher = "SOFTWARE.PUBLISHER"
	ColumnVersion   = "SOFTWARE.VERSION"
)

var ErrCritical = errors.New("critical format error")

type Limits struct {
	MaxRows       int
	MaxFieldBytes int
}

type RowError struct {
	Row     int
	Code    string
	Message string
}

type Result struct {
	Devices    map[string][]domain.RawSoftware
	TotalRows  int
	Duplicates int
	Excluded   int
	Errors     []RowError
	Orphans    int
}

type PreviewRow struct {
	Row          int    `json:"row"`
	SerialNumber string `json:"serial_number"`
	DisplayName  string `json:"display_name"`
	Publisher    string `json:"publisher"`
	Version      string `json:"version"`
	Excluded     bool   `json:"excluded"`
	Error        string `json:"error,omitempty"`
}

type Preview struct {
	Delimiter string       `json:"delimiter"`
	Columns   []string     `json:"columns"`
	Rows      []PreviewRow `json:"rows"`
}

func open(r io.Reader) (*csv.Reader, map[string]int, []string, rune, error) {
	br := bufio.NewReaderSize(r, 64<<10)
	if bom, err := br.Peek(3); err == nil && bytes.Equal(bom, []byte{0xEF, 0xBB, 0xBF}) {
		_, _ = br.Discard(3)
	}
	delimiter, err := sniffDelimiter(br)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	reader := csv.NewReader(br)
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = true
	reader.LazyQuotes = true

	header, err := reader.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil, nil, 0, fmt.Errorf("%w: file is empty", ErrCritical)
	}
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("%w: read header: %v", ErrCritical, err)
	}
	cols, err := columnIndex(header)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	return reader, cols, append([]string(nil), header...), delimiter, nil
}

func ReadPreview(r io.Reader, limit, maxFieldBytes int) (Preview, error) {
	reader, cols, header, delimiter, err := open(r)
	if err != nil {
		return Preview{}, err
	}
	preview := Preview{Delimiter: string(delimiter), Columns: header, Rows: []PreviewRow{}}
	for row := 2; len(preview.Rows) < limit; row++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var parseErr *csv.ParseError
			if errors.As(err, &parseErr) {
				preview.Rows = append(preview.Rows, PreviewRow{Row: row, Error: parseErr.Err.Error()})
				continue
			}
			return preview, fmt.Errorf("%w: read row %d: %v", ErrCritical, row, err)
		}
		item := PreviewRow{
			Row:          row,
			SerialNumber: field(record, cols[ColumnSerial]),
			DisplayName:  field(record, cols[ColumnName]),
			Publisher:    field(record, cols[ColumnPublisher]),
			Version:      field(record, cols[ColumnVersion]),
		}
		item.Excluded = swfilter.Excluded(item.DisplayName)
		if rowErr := validate(row, item.SerialNumber, item.DisplayName, item.Publisher, item.Version, maxFieldBytes); rowErr != nil {
			item.Error = rowErr.Message
		}
		preview.Rows = append(preview.Rows, item)
	}
	return preview, nil
}

func Parse(r io.Reader, limits Limits) (Result, error) {
	reader, cols, _, _, err := open(r)
	if err != nil {
		return Result{}, err
	}

	res := Result{Devices: make(map[string][]domain.RawSoftware)}
	seen := make(map[[3]string]int)
	for row := 2; ; row++ {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var parseErr *csv.ParseError
			if errors.As(err, &parseErr) {
				res.Errors = append(res.Errors, RowError{Row: row, Code: "MALFORMED_ROW", Message: parseErr.Err.Error()})
				res.Orphans++
				continue
			}
			return res, fmt.Errorf("%w: read row %d: %v", ErrCritical, row, err)
		}
		res.TotalRows++
		if res.TotalRows > limits.MaxRows {
			return res, fmt.Errorf("%w: file has more than %d rows", ErrCritical, limits.MaxRows)
		}

		serial, name := field(record, cols[ColumnSerial]), field(record, cols[ColumnName])
		publisher, version := field(record, cols[ColumnPublisher]), field(record, cols[ColumnVersion])
		if rowErr := validate(row, serial, name, publisher, version, limits.MaxFieldBytes); rowErr != nil {
			res.Errors = append(res.Errors, *rowErr)
			if serial == "" {
				res.Orphans++
			}
			continue
		}

		if swfilter.Excluded(name) {
			res.Excluded++
			continue
		}

		key := [3]string{serial, strings.ToLower(name), strings.ToLower(version)}
		if i, dup := seen[key]; dup {
			res.Duplicates++
			if res.Devices[serial][i].Publisher == "" {
				res.Devices[serial][i].Publisher = publisher
			}
			continue
		}
		seen[key] = len(res.Devices[serial])
		res.Devices[serial] = append(res.Devices[serial], domain.RawSoftware{DisplayName: name, Publisher: publisher, Version: version})
	}
	if res.TotalRows == 0 {
		return res, fmt.Errorf("%w: file has no data rows", ErrCritical)
	}
	return res, nil
}

func sniffDelimiter(br *bufio.Reader) (rune, error) {
	line, err := br.Peek(4096)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, fmt.Errorf("%w: read file: %v", ErrCritical, err)
	}
	if i := bytes.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	if !utf8.Valid(line) {
		return 0, fmt.Errorf("%w: file is not valid UTF-8", ErrCritical)
	}
	if bytes.Count(line, []byte(";")) > bytes.Count(line, []byte(",")) {
		return ';', nil
	}
	return ',', nil
}

func columnIndex(header []string) (map[string]int, error) {
	cols := map[string]int{ColumnPublisher: -1, ColumnVersion: -1}
	found := map[string]bool{}
	for i, h := range header {
		name := strings.ToUpper(strings.TrimSpace(h))
		switch name {
		case ColumnSerial, ColumnName, ColumnPublisher, ColumnVersion:
			if found[name] {
				return nil, fmt.Errorf("%w: duplicate column %s", ErrCritical, name)
			}
			found[name] = true
			cols[name] = i
		}
	}
	for _, required := range []string{ColumnSerial, ColumnName} {
		if !found[required] {
			return nil, fmt.Errorf("%w: missing required column %s", ErrCritical, required)
		}
	}
	return cols, nil
}

func field(record []string, i int) string {
	if i < 0 || i >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[i])
}

func validate(row int, serial, name, publisher, version string, maxBytes int) *RowError {
	switch {
	case serial == "":
		return &RowError{Row: row, Code: "MISSING_SERIAL_NUMBER", Message: "serial number is empty"}
	case name == "":
		return &RowError{Row: row, Code: "MISSING_DISPLAY_NAME", Message: "software name is empty"}
	case len(serial) > maxBytes || len(name) > maxBytes || len(publisher) > maxBytes || len(version) > maxBytes:
		return &RowError{Row: row, Code: "FIELD_TOO_LONG", Message: fmt.Sprintf("field exceeds %d bytes", maxBytes)}
	case !utf8.ValidString(serial + name + publisher + version):
		return &RowError{Row: row, Code: "INVALID_ENCODING", Message: "row is not valid UTF-8"}
	}
	return nil
}
