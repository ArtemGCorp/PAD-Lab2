// Command sampledata writes demo students.xlsx and tickets.docx into the
// current directory. tickets.docx intentionally contains one ticket with too
// few questions and one malformed header, to show that they are skipped.
//
//	go run ./cmd/sampledata
package main

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/xuri/excelize/v2"
)

func main() {
	if err := writeStudents("students.xlsx"); err != nil {
		log.Fatal(err)
	}
	if err := writeTickets("tickets.docx"); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Созданы students.xlsx и tickets.docx")
}

func writeStudents(path string) error {
	groups := []struct {
		name string
		rows [][2]string
	}{
		{"TI-221", [][2]string{{"Иванов", "Пётр"}, {"Петрова", "Анна"}, {"Сидоров", "Максим"}, {"Кузнецова", "Мария"}}},
		{"TI-222", [][2]string{{"Popescu", "Ion"}, {"Rusu", "Elena"}, {"Ceban", "Andrei"}}},
		{"TI-223", nil}, // пустая группа — проверка требования «Пустая группа»
	}
	f := excelize.NewFile()
	defer f.Close()
	for i, g := range groups {
		if i == 0 {
			if err := f.SetSheetName("Sheet1", g.name); err != nil {
				return err
			}
		} else if _, err := f.NewSheet(g.name); err != nil {
			return err
		}
		if err := f.SetSheetRow(g.name, "A1", &[]any{"Фамилия", "Имя"}); err != nil {
			return err
		}
		for r, s := range g.rows {
			if err := f.SetSheetRow(g.name, fmt.Sprintf("A%d", r+2), &[]any{s[0], s[1]}); err != nil {
				return err
			}
		}
		_ = f.SetColWidth(g.name, "A", "B", 18)
	}
	return f.SaveAs(path)
}

var topics = []string{
	"Распределённые системы: определение и свойства",
	"Модель клиент-сервер",
	"Удалённый вызов процедур (RPC)",
	"Обмен сообщениями и брокеры сообщений",
	"Согласованность данных в распределённых системах",
	"Теорема CAP",
	"Репликация данных",
	"Шардирование",
	"Алгоритмы консенсуса: Raft",
	"Алгоритмы консенсуса: Paxos",
	"Логические часы Лэмпорта",
	"Векторные часы",
	"Микросервисная архитектура",
	"API Gateway и балансировка нагрузки",
	"Контейнеризация и Docker",
	"Оркестрация: Kubernetes",
	"Отказоустойчивость и паттерн Circuit Breaker",
	"Кэширование в распределённых системах",
	"Распределённые транзакции и паттерн Saga",
	"Мониторинг и трассировка распределённых систем",
}

func writeTickets(path string) error {
	var paras []string
	for n := 1; n <= 10; n++ {
		paras = append(paras, fmt.Sprintf("Билет %d", n))
		for q := 0; q < 3; q++ {
			t := topics[(n*3+q*7)%len(topics)]
			paras = append(paras, fmt.Sprintf("%d. %s", q+1, t))
		}
	}
	// Broken on purpose:
	paras = append(paras,
		"Билет 11",
		"1. Только один вопрос",
		"Билет двенадцать",
		"1. Вопрос",
		"2. Вопрос",
		"3. Вопрос",
	)

	var body strings.Builder
	for _, p := range paras {
		var esc strings.Builder
		_ = xml.EscapeText(&esc, []byte(p))
		fmt.Fprintf(&body, `<w:p><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p>`, esc.String())
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` +
			body.String() + `</w:body></w:document>`,
	}
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml"} {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(files[name])); err != nil {
			return err
		}
	}
	return zw.Close()
}
