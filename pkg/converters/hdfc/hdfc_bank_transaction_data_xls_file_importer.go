package hdfc

import (
	"bytes"
	"regexp"
	"strings"
	"time"

	"github.com/extrame/xls"

	"github.com/mayswind/ezbookkeeping/pkg/converters/converter"
	"github.com/mayswind/ezbookkeeping/pkg/converters/csv"
	"github.com/mayswind/ezbookkeeping/pkg/converters/datatable"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// the statement has at most these 7 columns: Date, Narration, Chq./Ref.No., Value Dt, Withdrawal Amt., Deposit Amt., Closing Balance
const hdfcBankStatementMaxColumnCount = 7

var hdfcBankAccountNumberPattern = regexp.MustCompile(`Account No\s*:\s*(\d+)`)
var hdfcBankCurrencyPattern = regexp.MustCompile(`Currency\s*:\s*([A-Z]{3})`)

// hdfcBankTransactionDataXlsFileImporter defines the structure of hdfc bank account statement xls importer for transaction data
type hdfcBankTransactionDataXlsFileImporter struct{}

// Initialize a hdfc bank account statement xls file importer singleton instance
var (
	HdfcBankTransactionDataXlsFileImporter = &hdfcBankTransactionDataXlsFileImporter{}
)

// ParseImportedData returns the imported data by parsing the hdfc bank account statement xls data
func (c *hdfcBankTransactionDataXlsFileImporter) ParseImportedData(ctx core.Context, user *models.User, data []byte, defaultTimezone *time.Location, additionalOptions converter.TransactionDataImporterOptions, accountMap map[string]*models.Account, expenseCategoryMap map[string]map[string]*models.TransactionCategory, incomeCategoryMap map[string]map[string]*models.TransactionCategory, transferCategoryMap map[string]map[string]*models.TransactionCategory, tagMap map[string]*models.TransactionTag) (models.ImportedTransactionSlice, []*models.Account, []*models.TransactionCategory, []*models.TransactionCategory, []*models.TransactionCategory, []*models.TransactionTag, error) {
	allRows, err := readHdfcBankStatementRows(data)

	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	dataTable, accountName, currency, err := createNewHdfcBankTransactionBasicDataTable(ctx, allRows)

	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	commonDataTable := datatable.CreateNewCommonDataTableFromBasicDataTable(dataTable)
	transactionRowParser := createHdfcBankTransactionDataRowParser(accountName, currency)
	transactionDataTable := datatable.CreateNewTransactionDataTableFromCommonDataTable(commonDataTable, hdfcBankTransactionSupportedColumns, transactionRowParser)
	dataTableImporter := converter.CreateNewSimpleImporterWithTypeNameMapping(hdfcBankTransactionTypeNameMapping)

	return dataTableImporter.ParseImportedData(ctx, user, transactionDataTable, defaultTimezone, additionalOptions, accountMap, expenseCategoryMap, incomeCategoryMap, transferCategoryMap, tagMap)
}

// readHdfcBankStatementRows reads all non-empty rows of the first sheet.
// The generic excel data table stops at the first empty row, but hdfc statements have empty rows above and around the header.
func readHdfcBankStatementRows(data []byte) ([][]string, error) {
	workbook, err := xls.OpenReader(bytes.NewReader(data), "")

	if err != nil {
		return nil, errs.ErrInvalidFileHeader
	}

	if workbook.NumSheets() < 1 || workbook.GetSheet(0) == nil {
		return nil, errs.ErrNotFoundTransactionDataInFile
	}

	sheet := workbook.GetSheet(0)
	allRows := make([][]string, 0, int(sheet.MaxRow)+1)

	for i := 0; i <= int(sheet.MaxRow); i++ {
		row := sheet.Row(i)

		if row == nil {
			continue
		}

		items := make([]string, hdfcBankStatementMaxColumnCount)

		for j := 0; j < hdfcBankStatementMaxColumnCount; j++ {
			items[j] = strings.TrimSpace(row.Col(j))
		}

		allRows = append(allRows, items)
	}

	return allRows, nil
}

// createNewHdfcBankTransactionBasicDataTable returns the transaction rows between the header row and the "****" line closing the table,
// and the account name / currency found in the statement preamble
func createNewHdfcBankTransactionBasicDataTable(ctx core.Context, allRows [][]string) (datatable.BasicDataTable, string, string, error) {
	accountNumber := ""
	currency := ""
	headerFound := false
	allLines := make([][]string, 0, len(allRows))

	for rowIndex, items := range allRows {
		if !headerFound {
			for _, item := range items {
				if matches := hdfcBankAccountNumberPattern.FindStringSubmatch(item); accountNumber == "" && len(matches) > 1 {
					accountNumber = matches[1]
				}

				if matches := hdfcBankCurrencyPattern.FindStringSubmatch(item); currency == "" && len(matches) > 1 {
					currency = matches[1]
				}
			}

			if items[0] == hdfcBankTransactionDateColumnName && items[1] == hdfcBankTransactionNarrationColumnName {
				headerFound = true
				allLines = append(allLines, items)
			}

			continue
		}

		if strings.HasPrefix(items[0], "*") {
			// the first "****" line is right below the header, the next one closes the transaction table
			if len(allLines) > 1 {
				break
			}

			continue
		}

		if items[0] == "" {
			// a wrapped narration continues on the next row without a date
			if items[1] != "" && len(allLines) > 1 {
				lastLine := allLines[len(allLines)-1]
				lastLine[1] = lastLine[1] + " " + items[1]
			} else if items[1] != "" {
				log.Warnf(ctx, "[hdfc_bank_transaction_data_xls_file_importer.createNewHdfcBankTransactionBasicDataTable] skip row \"%d\" without date before any transaction", rowIndex)
			}

			continue
		}

		allLines = append(allLines, items)
	}

	if !headerFound {
		log.Errorf(ctx, "[hdfc_bank_transaction_data_xls_file_importer.createNewHdfcBankTransactionBasicDataTable] cannot find the transaction header row, this may not be a hdfc bank statement")
		return nil, "", "", errs.ErrInvalidFileHeader
	}

	if len(allLines) < 2 {
		log.Errorf(ctx, "[hdfc_bank_transaction_data_xls_file_importer.createNewHdfcBankTransactionBasicDataTable] cannot parse import data, because there is no transaction in the statement")
		return nil, "", "", errs.ErrNotFoundTransactionDataInFile
	}

	accountName := hdfcBankAccountNamePrefix

	if len(accountNumber) >= 4 {
		accountName = accountName + " " + accountNumber[len(accountNumber)-4:]
	}

	return csv.CreateNewCustomCsvBasicDataTable(allLines, true), accountName, currency, nil
}
