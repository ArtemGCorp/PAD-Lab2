package students

import (
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestLoadGroupsAndStudents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "students.xlsx")
	f := excelize.NewFile()
	f.SetSheetName("Sheet1", "A-1")
	f.SetSheetRow("A-1", "A1", &[]any{"Фамилия", "Имя"})
	f.SetSheetRow("A-1", "A2", &[]any{" Иванов ", "Пётр"})
	f.SetSheetRow("A-1", "A4", &[]any{"Петрова", "Анна"}) // blank row 3 is skipped
	f.NewSheet("B-2")
	f.SetSheetRow("B-2", "A1", &[]any{"Фамилия", "Имя"})
	f.NewSheet("Empty")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}

	r, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if g := r.Groups(); len(g) != 3 || g[0] != "A-1" || g[1] != "B-2" || g[2] != "Empty" {
		t.Fatalf("groups = %v", g)
	}
	st := r.Students("A-1")
	if len(st) != 2 || st[0].Display() != "Иванов Пётр" || st[1].Display() != "Петрова Анна" {
		t.Fatalf("students = %+v", st)
	}
	if len(r.Students("B-2")) != 0 || len(r.Students("Empty")) != 0 || len(r.Students("nope")) != 0 {
		t.Fatal("empty / unknown groups must have no students")
	}
}

func TestFindByDisplayIsScopedToGroup(t *testing.T) {
	r := New([]string{"G1", "G2"}, map[string][]Student{
		"G1": {{Last: "Иванов", First: "Пётр"}},
		"G2": {{Last: "Петрова", First: "Анна"}},
	})
	if s, ok := r.FindByDisplay("G1", "Иванов Пётр"); !ok || s.Last != "Иванов" {
		t.Fatalf("got %+v %v", s, ok)
	}
	if _, ok := r.FindByDisplay("G2", "Иванов Пётр"); ok {
		t.Fatal("student must not be found in another group")
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "none.xlsx")); err == nil {
		t.Fatal("expected error")
	}
}
