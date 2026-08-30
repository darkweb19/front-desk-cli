package inspect

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

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