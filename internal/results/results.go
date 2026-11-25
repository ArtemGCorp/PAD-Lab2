// Package results manages the append-only results.xlsx journal.
//
// Columns: Группа | Фамилия | Имя | Номер билета | Дата и время | Повтор (да/нет).
// Rows are only ever appended; every save goes through tmp + fsync + rename so
// a crash never leaves a half-written workbook.
package results

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// ErrLocked — results.xlsx is open in an office application; retry later.
var ErrLocked = errors.New("файл results.xlsx занят другой программой (например, открыт в Excel)")

// ErrBlankSheet — the sheet has rows but all are blank; refuse to guess.
var ErrBlankSheet = errors.New("лист results.xlsx содержит только пустые строки")

// TimeLayout is the format of the "Дата и время" column.
const TimeLayout = "2006-01-02 15:04:05"

// Headers of results.xlsx.
var Headers = []string{"Группа", "Фамилия", "Имя", "Номер билета", "Дата и время", "Повтор (да/нет)"}

// Record is one journal row.
type Record struct {
	Group  string
	Last   string
	First  string
	Ticket int
	When   time.Time
	Repeat bool
}

// Journal is the results.xlsx file.
type Journal struct {
	path string
}

// New builds a Journal over path.
func New(path string) *Journal { return &Journal{path: path} }

// Path returns the file path.
func (j *Journal) Path() string { return j.path }

func (j *Journal) lockMarkers() []string {
	dir, base := filepath.Dir(j.path), filepath.Base(j.path)
	return []string{
		filepath.Join(dir, "~$"+base),          // Microsoft Excel
		filepath.Join(dir, ".~lock."+base+"#"), // LibreOffice / ONLYOFFICE
	}
}

// IsLocked reports whether an office lock marker exists next to the file.
func (j *Journal) IsLocked() bool {
	for _, m := range j.lockMarkers() {
		if _, err := os.Stat(m); err == nil {
			return true
		}
	}
	return false
}

// ReadAll returns all records in file order (chronological). A missing file
// yields no records and no error.
func (j *Journal) ReadAll() ([]Record, error) {
	f, err := excelize.OpenFile(j.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("не удалось открыть %s: %w", j.path, err)
	}
	defer f.Close()
	rows, err := f.GetRows(f.GetSheetName(0))
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать %s: %w", j.path, err)
	}
	return parseRows(rows), nil
}

func parseRows(rows [][]string) []Record {
	var out []Record
	for i, row := range rows {
		if i == 0 || len(row) < 4 {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(row[3]))
		if err != nil {
			continue
		}
		rec := Record{
			Group:  strings.TrimSpace(row[0]),
			Last:   strings.TrimSpace(row[1]),
			First:  strings.TrimSpace(row[2]),
			Ticket: n,
		}
		if len(row) > 4 {
			rec.When, _ = time.ParseInLocation(TimeLayout, strings.TrimSpace(row[4]), time.Local)
		}
		if len(row) > 5 {
			rec.Repeat = strings.EqualFold(strings.TrimSpace(row[5]), "да")
		}
		out = append(out, rec)
	}
	return out
}

// FirstTicket returns the ticket number from the earliest record of the
// student (group + last + first), comparing case-insensitively.
func FirstTicket(recs []Record, group, last, first string) (int, bool) {
	for _, r := range recs {
		if strings.EqualFold(r.Group, group) &&
			strings.EqualFold(r.Last, last) &&
			strings.EqualFold(r.First, first) {
			return r.Ticket, true
		}
	}
	return 0, false
}

// Append writes one row after the last non-blank row. Creates the file with a
// header if needed. Existing rows are not modified.
func (j *Journal) Append(rec Record) error {
	if j.IsLocked() {
		return ErrLocked
	}

	var (
		f     *excelize.File
		sheet string
		row   int
	)
	if _, err := os.Stat(j.path); errors.Is(err, fs.ErrNotExist) {
		f = excelize.NewFile()
		sheet = "Sheet1"
		if err := writeHeader(f, sheet); err != nil {
			return err
		}
		row = 2
	} else {
		f, err = excelize.OpenFile(j.path)
		if err != nil {
			if errors.Is(err, fs.ErrPermission) {
				return ErrLocked
			}
			return fmt.Errorf("не удалось открыть %s: %w", j.path, err)
		}
		sheet = f.GetSheetName(0)
		rows, err := f.GetRows(sheet)
		if err != nil {
			f.Close()
			return fmt.Errorf("не удалось прочитать %s: %w", j.path, err)
		}
		if len(rows) == 0 {
			if err := writeHeader(f, sheet); err != nil {
				f.Close()
				return err
			}
			row = 2
		} else if row, err = nextRow(rows); err != nil {
			f.Close()
			return err
		}
	}
	defer f.Close()

	repeat := "нет"
	if rec.Repeat {
		repeat = "да"
	}
	vals := []any{rec.Group, rec.Last, rec.First, rec.Ticket, rec.When.Format(TimeLayout), repeat}
	if err := f.SetSheetRow(sheet, fmt.Sprintf("A%d", row), &vals); err != nil {
		return fmt.Errorf("не удалось записать строку: %w", err)
	}
	return saveAtomic(f, j.path)
}

func nextRow(rows [][]string) (int, error) {
	for i := len(rows) - 1; i >= 0; i-- {
		for _, c := range rows[i] {
			if strings.TrimSpace(c) != "" {
				return i + 2, nil
			}
		}
	}
	return 0, ErrBlankSheet
}

func writeHeader(f *excelize.File, sheet string) error {
	if err := f.SetSheetRow(sheet, "A1", &Headers); err != nil {
		return fmt.Errorf("не удалось записать шапку: %w", err)
	}
	style, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return err
	}
	if err := f.SetRowStyle(sheet, 1, 1, style); err != nil {
		return err
	}
	return f.SetColWidth(sheet, "A", "F", 18)
}

// saveAtomic: SaveAs(tmp.xlsx) → fsync → rename. The tmp name keeps the .xlsx
// extension because excelize refuses to save other extensions.
func saveAtomic(f *excelize.File, path string) error {
	ext := filepath.Ext(path)
	tmp := strings.TrimSuffix(path, ext) + ".tmp" + ext
	fail := func(err error, what string) error {
		os.Remove(tmp)
		if errors.Is(err, fs.ErrPermission) {
			return ErrLocked
		}
		return fmt.Errorf("%s: %w", what, err)
	}
	if err := f.SaveAs(tmp); err != nil {
		return fail(err, "не удалось сохранить временный файл")
	}
	tf, err := os.OpenFile(tmp, os.O_RDWR, 0)
	if err != nil {
		return fail(err, "не удалось открыть временный файл")
	}
	if err := tf.Sync(); err != nil {
		tf.Close()
		return fail(err, "не удалось синхронизировать временный файл")
	}
	if err := tf.Close(); err != nil {
		return fail(err, "не удалось закрыть временный файл")
	}
	if err := os.Rename(tmp, path); err != nil {
		return fail(err, "не удалось заменить results.xlsx")
	}
	return nil
}
