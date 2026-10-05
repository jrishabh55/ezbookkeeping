package pnb

import (
	"bytes"
	"encoding/csv"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"

	"github.com/mayswind/ezbookkeeping/pkg/converters/converter"
	ebkcsv "github.com/mayswind/ezbookkeeping/pkg/converters/csv"
	"github.com/mayswind/ezbookkeeping/pkg/converters/datatable"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

var pnbOneAccountNumberPattern = regexp.MustCompile(`Account Number\s*(\d+)`)
var pnbOneTransactionDatePattern = regexp.MustCompile(`^\d{2}/\d{2}/\d{4}$`)

// pnbOneTransactionDataCsvFileImporter defines the structure of punjab national bank (pnb one) account statement csv importer for transaction data
type pnbOneTransactionDataCsvFileImporter struct{}

// Initialize a pnb one account statement csv file importer singleton instance
var (
	PnbOneTransactionDataCsvFileImporter = &pnbOneTransactionDataCsvFileImporter{}
)

// ParseImportedData returns the imported data by parsing the pnb one account statement csv data
func (c *pnbOneTransactionDataCsvFileImporter) ParseImportedData(ctx core.Context, user *models.User, data []byte, defaultTimezone *time.Location, additionalOptions converter.TransactionDataImporterOptions, accountMap map[string]*models.Account, expenseCategoryMap map[string]map[string]*models.TransactionCategory, incomeCategoryMap map[string]map[string]*models.TransactionCategory, transferCategoryMap map[string]map[string]*models.TransactionCategory, tagMap map[string]*models.TransactionTag) (models.ImportedTransactionSlice, []*models.Account, []*models.TransactionCategory, []*models.TransactionCategory, []*models.TransactionCategory, []*models.TransactionTag, error) {
	content, _, err := transform.Bytes(unicode.BOMOverride(unicode.UTF8.NewDecoder()), data)

	if err != nil {
		return nil, nil, nil, nil, nil, nil, errs.ErrInvalidCSVFile
	}

	dataTable, accountName, err := createNewPnbOneTransactionBasicDataTable(ctx, string(content))

	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	commonDataTable := datatable.CreateNewCommonDataTableFromBasicDataTable(dataTable)
	transactionRowParser := createPnbOneTransactionDataRowParser(accountName)
	transactionDataTable := datatable.CreateNewTransactionDataTableFromCommonDataTable(commonDataTable, pnbOneTransactionSupportedColumns, transactionRowParser)
	dataTableImporter := converter.CreateNewSimpleImporterWithTypeNameMapping(pnbOneTransactionTypeNameMapping)

	return dataTableImporter.ParseImportedData(ctx, user, transactionDataTable, defaultTimezone, additionalOptions, accountMap, expenseCategoryMap, incomeCategoryMap, transferCategoryMap, tagMap)
}

// createNewPnbOneTransactionBasicDataTable returns the transaction rows below the header line and the account name from the first line.
// Lines are parsed one at a time and reading stops at the first line that is not a transaction,
// because the notes after the table contain free text with unbalanced quotes that a whole-file csv parse would choke on.
func createNewPnbOneTransactionBasicDataTable(ctx core.Context, content string) (datatable.BasicDataTable, string, error) {
	accountNumber := ""
	headerFound := false
	allLines := make([][]string, 0)

	for lineIndex, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		if !headerFound {
			if matches := pnbOneAccountNumberPattern.FindStringSubmatch(line); accountNumber == "" && len(matches) > 1 {
				accountNumber = matches[1]
			}

			if strings.HasPrefix(line, pnbOneTransactionNumberColumnName+",") {
				items, err := parsePnbOneCsvLine(line)

				if err != nil || len(items) < 7 || items[1] != pnbOneTransactionDateColumnName {
					continue
				}

				headerFound = true
				allLines = append(allLines, items)
			}

			continue
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		items, err := parsePnbOneCsvLine(line)

		if err != nil || len(items) < len(allLines[0]) || !pnbOneTransactionDatePattern.MatchString(strings.TrimSpace(items[1])) {
			// the transaction table is over ("***Generated through PNB ONE ***" and the notes follow)
			log.Infof(ctx, "[pnb_one_transaction_data_csv_file_importer.createNewPnbOneTransactionBasicDataTable] transaction table ends before line %d", lineIndex)
			break
		}

		// rows have a trailing comma, so they carry one more (empty) field than the header
		allLines = append(allLines, items[:len(allLines[0])])
	}

	if !headerFound {
		log.Errorf(ctx, "[pnb_one_transaction_data_csv_file_importer.createNewPnbOneTransactionBasicDataTable] cannot find the transaction header line, this may not be a pnb one statement")
		return nil, "", errs.ErrInvalidFileHeader
	}

	if len(allLines) < 2 {
		log.Errorf(ctx, "[pnb_one_transaction_data_csv_file_importer.createNewPnbOneTransactionBasicDataTable] cannot parse import data, because there is no transaction in the statement")
		return nil, "", errs.ErrNotFoundTransactionDataInFile
	}

	accountName := pnbOneAccountNamePrefix

	if len(accountNumber) >= 4 {
		accountName = accountName + " " + accountNumber[len(accountNumber)-4:]
	}

	return ebkcsv.CreateNewCustomCsvBasicDataTable(allLines, true), accountName, nil
}

func parsePnbOneCsvLine(line string) ([]string, error) {
	reader := csv.NewReader(bytes.NewReader([]byte(line)))
	reader.FieldsPerRecord = -1

	items, err := reader.Read()

	if err != nil {
		return nil, err
	}

	for i := range items {
		items[i] = strings.TrimSpace(items[i])
	}

	return items, nil
}
