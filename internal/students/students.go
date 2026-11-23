// Package students reads students.xlsx: one sheet per group (sheet name =
// group name), header in row 1, data from row 2, column A = last name,
// column B = first name.
package students

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Student is one person in a group.
type Student struct {
	Last  string
	First string
}

// Display renders the student as "Фамилия Имя".
func (s Student) Display() string {
	return strings.TrimSpace(s.Last + " " + s.First)
}

// Roster is the content of students.xlsx.
type Roster struct {
	groups []string
	byName map[string][]Student
}

// Load reads the workbook at path.
func Load(path string) (*Roster, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть %s: %w", path, err)
	}
	defer f.Close()

	r := &Roster{byName: map[string][]Student{}}
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			return nil, fmt.Errorf("лист %q: %w", sheet, err)
		}
		r.groups = append(r.groups, sheet)
		r.byName[sheet] = FromRows(rows)
	}
	return r, nil
}

// FromRows converts sheet rows (row 0 = header) into students. Rows whose
// last and first name are both blank are ignored.
func FromRows(rows [][]string) []Student {
	var out []Student
	for i, row := range rows {
		if i == 0 {
			continue // header
		}
		var last, first string
		if len(row) > 0 {
			last = strings.TrimSpace(row[0])
		}
		if len(row) > 1 {
			first = strings.TrimSpace(row[1])
		}
		if last == "" && first == "" {
			continue
		}
		out = append(out, Student{Last: last, First: first})
	}
	return out
}

// New builds a roster in memory (used by tests).
func New(groups []string, byName map[string][]Student) *Roster {
	return &Roster{groups: groups, byName: byName}
}

// Groups returns group names in workbook order.
func (r *Roster) Groups() []string {
	return append([]string(nil), r.groups...)
}

// Students returns the students of a group (nil for an unknown or empty group).
func (r *Roster) Students(group string) []Student {
	return append([]Student(nil), r.byName[group]...)
}

// FindByDisplay looks up a student of a group by its "Фамилия Имя" label.
func (r *Roster) FindByDisplay(group, display string) (Student, bool) {
	for _, s := range r.byName[group] {
		if s.Display() == display {
			return s, true
		}
	}
	return Student{}, false
}
