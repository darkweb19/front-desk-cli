package report

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/beevik/etree"

	"tm/internal/entries"
)

const (
	headerTableIndex    = 0
	headerDateRowIndex  = 2
	headerDateCellIndex = 1

	activityTableIndex = 1
	fixedActivityRow   = 2
	firstEntryRow      = 3
	maxEntries         = 24

	fixedActivityTime    = "07:15"
	fixedActivityMessage = "Got updates from Mario and checked emails/shift reports."
)

// Generate creates a report from templateData without changing the template.
func Generate(outputPath string, templateData []byte, items []entries.Entry, now time.Time) error {
	if len(items) > maxEntries {
		return fmt.Errorf("too many entries: maximum is %d, got %d", maxEntries, len(items))
	}

	location, err := time.LoadLocation("America/Toronto")
	if err != nil {
		return fmt.Errorf("load Toronto timezone: %w", err)
	}

	templateArchive, documentFile, xmlDocument, err := readTemplate(templateData)
	if err != nil {
		return err
	}

	headerDateCell, activityRows, err := validateTemplate(xmlDocument)
	if err != nil {
		return err
	}

	setCellText(headerDateCell, "Date: "+now.In(location).Format("02 January, 2006."))
	fixedCells := activityRows[fixedActivityRow].FindElements("./w:tc")
	setCellText(fixedCells[0], fixedActivityTime)
	setCellText(fixedCells[1], fixedActivityMessage)

	for index, item := range items {
		cells := activityRows[firstEntryRow+index].FindElements("./w:tc")
		setCellText(cells[0], item.Time.In(location).Format("15:04"))
		setCellText(cells[1], item.Message)
	}

	modifiedXML, err := xmlDocument.WriteToBytes()
	if err != nil {
		return fmt.Errorf("serialize word/document.xml: %w", err)
	}

	if err := writeReport(outputPath, templateArchive, documentFile, modifiedXML); err != nil {
		return err
	}

	return nil
}

func readTemplate(templateData []byte) (*zip.Reader, *zip.File, *etree.Document, error) {
	archive, err := zip.NewReader(bytes.NewReader(templateData), int64(len(templateData)))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open embedded DOCX template: %w", err)
	}

	for _, file := range archive.File {
		if file.Name != "word/document.xml" {
			continue
		}

		reader, err := file.Open()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("open word/document.xml: %w", err)
		}

		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			return nil, nil, nil, fmt.Errorf("read word/document.xml: %w", readErr)
		}
		if closeErr != nil {
			return nil, nil, nil, fmt.Errorf("close word/document.xml: %w", closeErr)
		}

		document := etree.NewDocument()
		if err := document.ReadFromBytes(data); err != nil {
			return nil, nil, nil, fmt.Errorf("parse word/document.xml: %w", err)
		}

		return archive, file, document, nil
	}

	return nil, nil, nil, fmt.Errorf("word/document.xml not found in template")
}

func validateTemplate(document *etree.Document) (*etree.Element, []*etree.Element, error) {
	tables := document.FindElements("//w:tbl")
	if len(tables) <= activityTableIndex {
		return nil, nil, fmt.Errorf("activity table not found in template")
	}

	headerRows := tables[headerTableIndex].FindElements("./w:tr")
	if len(headerRows) <= headerDateRowIndex {
		return nil, nil, fmt.Errorf("header date row not found in template")
	}
	headerCells := headerRows[headerDateRowIndex].FindElements("./w:tc")
	if len(headerCells) <= headerDateCellIndex {
		return nil, nil, fmt.Errorf("header date cell not found in template")
	}
	if !strings.HasPrefix(strings.TrimSpace(elementText(headerCells[headerDateCellIndex])), "Date:") {
		return nil, nil, fmt.Errorf("expected date label not found in header table")
	}

	activityRows := tables[activityTableIndex].FindElements("./w:tr")
	requiredRows := firstEntryRow + maxEntries
	if len(activityRows) < requiredRows {
		return nil, nil, fmt.Errorf("activity table has %d rows; need at least %d", len(activityRows), requiredRows)
	}
	activityHeaderCells := activityRows[0].FindElements("./w:tc")
	if len(activityHeaderCells) < 2 ||
		strings.TrimSpace(elementText(activityHeaderCells[0])) != "Time" ||
		!strings.Contains(elementText(activityHeaderCells[1]), "Description of Duties Performed") {
		return nil, nil, fmt.Errorf("expected activity table headings not found in template")
	}

	for rowIndex := fixedActivityRow; rowIndex < requiredRows; rowIndex++ {
		if cells := activityRows[rowIndex].FindElements("./w:tc"); len(cells) < 2 {
			return nil, nil, fmt.Errorf("activity row %d does not contain two cells", rowIndex)
		}
	}

	return headerCells[headerDateCellIndex], activityRows, nil
}

func setCellText(cell *etree.Element, value string) {
	textElements := cell.FindElements(".//w:t")
	if len(textElements) > 0 {
		textElements[0].SetText(value)
		for _, textElement := range textElements[1:] {
			textElement.SetText("")
		}
		return
	}

	paragraph := cell.FindElement("./w:p")
	if paragraph == nil {
		paragraph = cell.CreateElement("w:p")
	}
	run := paragraph.FindElement("./w:r")
	if run == nil {
		run = paragraph.CreateElement("w:r")
	}
	run.CreateElement("w:t").SetText(value)
}

func writeReport(outputPath string, templateArchive *zip.Reader, documentFile *zip.File, modifiedXML []byte) (returnErr error) {
	temporary, err := os.CreateTemp(filepath.Dir(outputPath), "."+filepath.Base(outputPath)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary report: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		if err := os.Remove(temporaryPath); err != nil && !os.IsNotExist(err) && returnErr == nil {
			returnErr = fmt.Errorf("clean up temporary report: %w", err)
		}
	}()

	zipWriter := zip.NewWriter(temporary)
	for _, file := range templateArchive.File {
		if file.Name != documentFile.Name {
			if err := zipWriter.Copy(file); err != nil {
				zipWriter.Close()
				temporary.Close()
				return fmt.Errorf("copy DOCX entry %s: %w", file.Name, err)
			}
			continue
		}

		header := file.FileHeader
		writer, err := zipWriter.CreateHeader(&header)
		if err != nil {
			zipWriter.Close()
			temporary.Close()
			return fmt.Errorf("create word/document.xml entry: %w", err)
		}
		if _, err := writer.Write(modifiedXML); err != nil {
			zipWriter.Close()
			temporary.Close()
			return fmt.Errorf("write word/document.xml entry: %w", err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		temporary.Close()
		return fmt.Errorf("close report ZIP: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary report: %w", err)
	}
	if err := validateReportArchive(temporaryPath); err != nil {
		return err
	}
	if err := replaceFile(temporaryPath, outputPath); err != nil {
		return fmt.Errorf("replace report: %w", err)
	}

	return nil
}

func validateReportArchive(path string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("validate generated DOCX: %w", err)
	}
	defer reader.Close()

	foundDocument := false
	for _, file := range reader.File {
		stream, err := file.Open()
		if err != nil {
			return fmt.Errorf("validate generated DOCX entry %s: %w", file.Name, err)
		}
		data, readErr := io.ReadAll(stream)
		closeErr := stream.Close()
		if readErr != nil {
			return fmt.Errorf("validate generated DOCX entry %s: %w", file.Name, readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close generated DOCX entry %s: %w", file.Name, closeErr)
		}
		if file.Name == "word/document.xml" {
			document := etree.NewDocument()
			if err := document.ReadFromBytes(data); err != nil {
				return fmt.Errorf("validate generated word/document.xml: %w", err)
			}
			foundDocument = true
		}
	}
	if !foundDocument {
		return fmt.Errorf("validate generated DOCX: word/document.xml not found")
	}
	return nil
}

func replaceFile(sourcePath, destinationPath string) error {
	info, err := os.Stat(destinationPath)
	if os.IsNotExist(err) {
		return os.Rename(sourcePath, destinationPath)
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("destination is not a regular file")
	}

	backupPath := sourcePath + ".previous"
	if err := os.Rename(destinationPath, backupPath); err != nil {
		return err
	}
	if err := os.Rename(sourcePath, destinationPath); err != nil {
		if restoreErr := os.Rename(backupPath, destinationPath); restoreErr != nil {
			return fmt.Errorf("install new file: %w; restore previous file: %v", err, restoreErr)
		}
		return err
	}
	_ = os.Remove(backupPath)
	return nil
}

func elementText(element *etree.Element) string {
	var text strings.Builder
	for _, node := range element.FindElements(".//w:t") {
		text.WriteString(node.Text())
	}
	return text.String()
}
