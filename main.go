package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/xuri/excelize/v2"
)

var DiffOutputDir string
var (
	DataOutputDir string
	DocsDir       string
	ColumnName    string
	Values        []string
	Overwrite     []string
	Default       string
)

func init() {
	now := time.Now().Format("2006-01-02_15-04-05")
	DiffOutputDir = fmt.Sprintf("diff-%s", now)

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func loadEnv() error {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  .env not found, using system environment variables")
	}

	DataOutputDir = os.Getenv("DATA_OUTPUT_DIR")
	if DataOutputDir == "" {
		return fmt.Errorf("DATA_OUTPUT_DIR is not set")
	}

	DocsDir = os.Getenv("DOCS_DIR")
	if DocsDir == "" {
		return fmt.Errorf("DOCS_DIR is not set")
	}

	ColumnName = os.Getenv("COLUMN_NAME")
	if ColumnName == "" {
		return fmt.Errorf("COLUMN_NAME is not set")
	}

	ValuesStr := os.Getenv("VALUES")
	if ValuesStr == "" {
		return fmt.Errorf("VALUES is not set")
	}
	Values = strings.Split(ValuesStr, ",")

	OverwriteStr := os.Getenv("OVERWRITE")
	if OverwriteStr == "" {
		return fmt.Errorf("OVERWRITE is not set")
	}
	Overwrite = strings.Split(OverwriteStr, ",")

	Default = os.Getenv("DEFAULT")
	if Default == "" {
		return fmt.Errorf("DEFAULT is not set")
	}

	return nil
}

func main() {
	if err := loadEnv(); err != nil {
		log.Fatalf("❌ %v", err)
	}

	if err := os.MkdirAll(DataOutputDir, os.ModePerm); err != nil {
		log.Fatalf("❌ failure to create folder %s: %v", DataOutputDir, err)
	}

	pattern := filepath.Join(DocsDir, "*.*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		log.Fatalf("❌ failed to list files in DOCS_DIR: %v", err)
	}

	for _, filePath := range files {
		ext := strings.ToLower(filepath.Ext(filePath))
		base := strings.TrimSuffix(filepath.Base(filePath), ext)
		outDir := filepath.Join(DataOutputDir, base)

		if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
			log.Fatalf("❌ cannot create folder %s: %v", outDir, err)
			continue
		}

		switch ext {
		case ".xlsx":

			csvPath := filepath.Join(outDir, base+".csv")
			if err := convertXLSXToCSV(filePath, csvPath); err != nil {
				log.Fatalf("❌ error converting %s: %v", filePath, err)
				continue
			}

			if err := processCSV(csvPath, outDir); err != nil {
				log.Fatalf("❌ error sanitizing %s: %v", csvPath, err)
			}

		case ".csv":

			if err := processCSV(filePath, outDir); err != nil {
				log.Fatalf("❌ error sanitizing %s: %v", filePath, err)
			}

		default:
			log.Printf("Skipping unsupported file %s", filePath)
		}
	}

	log.Println("✅ All files processed.")
}

func convertXLSXToCSV(xlsxPath, csvPath string) error {
	f, err := excelize.OpenFile(xlsxPath)
	if err != nil {
		return err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("no sheets in %s", xlsxPath)
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return err
	}

	out, err := os.Create(csvPath)
	if err != nil {
		return err
	}
	defer out.Close()

	w := csv.NewWriter(out)
	defer w.Flush()

	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func processCSV(path string, outDir string) error {
	inFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer inFile.Close()

	reader := csv.NewReader(inFile)
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}
	if len(records) < 1 {
		return fmt.Errorf("empty CSV: %s", path)
	}

	header := records[0]
	colIdx := -1
	for i, name := range header {
		if name == ColumnName {
			colIdx = i
			break
		}
	}
	if colIdx == -1 {
		return fmt.Errorf("column %s not found in %s", ColumnName, path)
	}

	outPath := filepath.Join(outDir, "sanitized_"+filepath.Base(path))
	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	writer.Write(header)

	for _, row := range records[1:] {
		original := row[colIdx]
		mapped := Default
		for i, v := range Values {
			if v == original {
				if i < len(Overwrite) {
					mapped = Overwrite[i]
				}
				break
			}
		}
		row[colIdx] = mapped
		writer.Write(row)
	}
	return nil
}
