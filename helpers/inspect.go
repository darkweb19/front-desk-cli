package inspect

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/beevik/etree"
)

type Text struct {
	Value string `xml:",chardata"`
}

type Cell struct {
	Texts []Text `xml:"p>r>t"`
}

type Row struct {
	Cells []Cell `xml:"tc"`
}

type Table struct {
	Rows []Row `xml:"tr"`
}

type Document struct {
	Tables []Table `xml:"body>tbl"`
}

func InspectDocx(path string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			return err
		}

		var doc Document

		err = xml.Unmarshal(data, &doc)
		if err != nil {
			return fmt.Errorf("error parsing document.xml: %w", err)
		}

		for tableIndex, table := range doc.Tables {
			fmt.Printf("\nTABLE %d\n", tableIndex)

			for rowIndex, row := range table.Rows {
				fmt.Printf("Row %d: ", rowIndex)

				for cellIndex, cell := range row.Cells {
					var parts []string

					for _, text := range cell.Texts {
						parts = append(parts, text.Value)
					}

					cellText := strings.Join(parts, "")

					fmt.Printf("[%d] %s ", cellIndex, cellText)
				}

				fmt.Println()
			}
		}

		return nil
	}

	return fmt.Errorf("word/document.xml not found")
}

func ReadDocx(path string) (*Document, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, err
		}

		// Parse full Word XML using etree
		xmlDoc := etree.NewDocument()

		err = xmlDoc.ReadFromBytes(data)
		if err != nil {
			return nil, fmt.Errorf("error reading XML with etree: %w", err)
		}

		// Find every table, including nested tables
		tables := xmlDoc.FindElements("//w:tbl")

		fmt.Println("Tables found with etree:", len(tables))

		for tableIndex, table := range tables {
			fmt.Printf("\nETREE TABLE %d:\n", tableIndex)

			textElements := table.FindElements(".//w:t")

			for _, textElement := range textElements {
				fmt.Print(textElement.Text())
				fmt.Print(" ")
			}

			fmt.Println()
		}

		// Keep your existing simplified parsing
		var doc Document

		err = xml.Unmarshal(data, &doc)
		if err != nil {
			return nil, fmt.Errorf("error parsing document.xml: %w", err)
		}

		return &doc, nil
	}

	return nil, fmt.Errorf("word/document.xml not found")
}

func ModifyActivityTable(path string, entries []struct {
	Time    time.Time
	Message string
}) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()

	var documentXML []byte

	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return err
		}

		documentXML, err = io.ReadAll(rc)
		rc.Close()

		if err != nil {
			return err
		}

		break
	}

	if documentXML == nil {
		return fmt.Errorf("word/document.xml not found")
	}

	xmlDoc := etree.NewDocument()

	err = xmlDoc.ReadFromBytes(documentXML)
	if err != nil {
		return err
	}

	tables := xmlDoc.FindElements("//w:tbl")

	if len(tables) < 2 {
		return fmt.Errorf("activity table not found")
	}

	activityTable := tables[1]

	rows := activityTable.FindElements("./w:tr")

	fmt.Println("Activity rows found:", len(rows))

	return nil
}

func InspectActivityTable(path string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()

	var documentXML []byte

	// Find and read word/document.xml
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return err
		}

		documentXML, err = io.ReadAll(rc)
		rc.Close()

		if err != nil {
			return err
		}

		break
	}

	if documentXML == nil {
		return fmt.Errorf("word/document.xml not found")
	}

	// Parse the XML with etree
	xmlDoc := etree.NewDocument()

	err = xmlDoc.ReadFromBytes(documentXML)
	if err != nil {
		return fmt.Errorf("error parsing XML: %w", err)
	}

	// Find all tables
	tables := xmlDoc.FindElements("//w:tbl")

	if len(tables) < 2 {
		return fmt.Errorf("activity table not found")
	}

	// We already confirmed etree table 1
	// is the Description of Duties Performed table.
	activityTable := tables[1]

	rows := activityTable.FindElements("./w:tr")

	fmt.Println("Activity rows found:", len(rows))

	if len(rows) <= 2 {
		return fmt.Errorf("expected row 2 in activity table")
	}

	// Get the two cells from Row 2
	row2Cells := rows[2].FindElements("./w:tc")

	if len(row2Cells) < 2 {
		return fmt.Errorf("expected 2 cells in row 2")
	}

	// Change Row 2
	setEtreeCellText(row2Cells[0], "07:15")

	setEtreeCellText(
		row2Cells[1],
		"Got updates from Mario and checked emails/shift reports.",
	)

	// Read the modified time cell back
	var timeText strings.Builder

	for _, textElement := range row2Cells[0].FindElements(".//w:t") {
		timeText.WriteString(textElement.Text())
	}

	// Read the modified description cell back
	var messageText strings.Builder

	for _, textElement := range row2Cells[1].FindElements(".//w:t") {
		messageText.WriteString(textElement.Text())
	}

	fmt.Println("Modified etree row 2:")
	fmt.Println("Time:", timeText.String())
	fmt.Println("Message:", messageText.String())

	return nil
}


func setEtreeCellText(cell *etree.Element, value string) {
	textElements := cell.FindElements(".//w:t")

	// The cell already contains one or more text elements.
	if len(textElements) > 0 {
		textElements[0].SetText(value)

		// Word may have split the original text across multiple <w:t>
		// elements, so clear everything after the first one.
		for _, textElement := range textElements[1:] {
			textElement.SetText("")
		}

		return
	}

	// Empty cells may not contain a <w:t>,
	// so create the required elements.
	paragraph := cell.FindElement("./w:p")
	if paragraph == nil {
		paragraph = cell.CreateElement("w:p")
	}

	run := paragraph.FindElement("./w:r")
	if run == nil {
		run = paragraph.CreateElement("w:r")
	}

	text := run.CreateElement("w:t")
	text.SetText(value)
}
