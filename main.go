package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	inspect "tm/helpers"
)

type Entry struct {
	Time    time.Time
	Message string
}

func main() {

	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Println("Error getting config directory:", err)
		return
	}

	appDir := filepath.Join(configDir, "tm")
	tasksFile := filepath.Join(appDir, "tasks.json")

	err = os.MkdirAll(appDir, 0755)
	if err != nil {
		fmt.Println("Error creating app directory:", err)
		return
	}

	if len(os.Args) < 2 {
		fmt.Println("Please provide an argument.")
		return
	}

	// generate command
	if os.Args[1] == "--generate" {

		fmt.Println("Generate command detected")
		entries, err := loadEntries(tasksFile)

		if err != nil {
			fmt.Println("Error loading entries:", err)
			return
		}

		err = generateReport("./Templates.docx", "./202609012.docx", entries)
		if err != nil {
			fmt.Println("Error generating report:", err)
			return
		}

		return
	}

	entries, err := loadEntries(tasksFile)

	if err != nil {
		fmt.Println("Error loading entries:", err)
		return
	}
	entries = append(entries, Entry{Time: time.Now(), Message: strings.Join(os.Args[1:], " ")})

	err = saveEntries(tasksFile, entries)
	if err != nil {
		fmt.Println("Error saving entries:", err)
		return
	}
	fmt.Println(entries)

}

func loadEntries(tasksFile string) ([]Entry, error) {

	entries := []Entry{}
	file, err := os.Open(tasksFile)
	if err != nil {
		if os.IsNotExist(err) {
			// this is okay: first run
		} else {
			return nil, fmt.Errorf("error opening file: %w", err)
		}
	} else {
		defer file.Close()

		decoder := json.NewDecoder(file)
		err = decoder.Decode(&entries)
		if err != nil {

			return nil, fmt.Errorf("error decoding JSON: %v", err)
		}

	}
	return entries, nil

}

func saveEntries(tasksFile string, entries []Entry) error {
	file, err := os.Create(tasksFile)
	if err != nil {
		return fmt.Errorf("error creating file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(entries)
	if err != nil {
		return fmt.Errorf("error encoding JSON: %v", err)
	}

	return nil
}

func generateReport(templatePath string, outputPath string, entries []Entry) error {

	// open the template file
	templateFile, err := os.Open(templatePath)
	if err != nil {
		return fmt.Errorf("error opening template file: %w", err)
	}

	// create the output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}

	// copy bytes from template to output
	_, err = io.Copy(outputFile, templateFile)
	if err != nil {
		return fmt.Errorf("error copying bytes: %w", err)
	}

	//close the both files

	defer templateFile.Close()
	defer outputFile.Close()

	for _, entry := range entries {
		fmt.Printf("%s: %s\n", entry.Time.Format("15:04"), entry.Message)
	}

	doc, err := inspect.ReadDocx("./202609012.docx")
	if err != nil {
		fmt.Println("Error reading docx:", err)

	}

	err = populateActivityTable(doc, entries)
	if err != nil {
		fmt.Println("Error populating report:", err)

	}

	printActivityTable(doc)

	return nil
}

func setCellText(cell *inspect.Cell, value string) {
	cell.Texts = []inspect.Text{
		{Value: value},
	}
}

func populateActivityTable(doc *inspect.Document, entries []Entry) error {
	if len(doc.Tables) < 2 {
		return fmt.Errorf("activity table not found")
	}

	table := &doc.Tables[1]

	if len(entries) > 24 {
		return fmt.Errorf("too many entries: maximum is 24, got %d", len(entries))
	}

	// Fixed second entry
	setCellText(&table.Rows[2].Cells[0], "07:15")
	setCellText(
		&table.Rows[2].Cells[1],
		"Got updates from Mario and checked emails/shift reports.",
	)

	// Dynamic entries start at row 3
	for i, entry := range entries {
		rowIndex := i + 3

		setCellText(
			&table.Rows[rowIndex].Cells[0],
			entry.Time.Format("15:04"),
		)

		setCellText(
			&table.Rows[rowIndex].Cells[1],
			entry.Message,
		)
	}

	return nil
}

func printActivityTable(doc *inspect.Document) {
	table := doc.Tables[1]

	for i, row := range table.Rows {
		if i == 0 {
			continue
		}

		timeText := cellText(row.Cells[0])
		messageText := cellText(row.Cells[1])

		if timeText != "" || messageText != "" {
			fmt.Printf("Row %d: %s | %s\n", i, timeText, messageText)
		}
	}
}


func cellText(cell inspect.Cell) string {
	var parts []string

	for _, text := range cell.Texts {
		parts = append(parts, text.Value)
	}

	return strings.Join(parts, "")
}