package hdfc

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/converters/converter"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

func TestHdfcBankTransactionDataXlsImporterParseImportedData(t *testing.T) {
	importer := HdfcBankTransactionDataXlsFileImporter
	context := core.NewNullContext()

	user := &models.User{
		Uid:             1234567890,
		DefaultCurrency: "USD",
	}

	testdata, err := os.ReadFile("../../../testdata/hdfc_bank_statement_test_file.xls")
	assert.Nil(t, err)

	allNewTransactions, allNewAccounts, allNewSubExpenseCategories, allNewSubIncomeCategories, allNewSubTransferCategories, allNewTags, err := importer.ParseImportedData(context, user, testdata, time.UTC, converter.DefaultImporterOptions, nil, nil, nil, nil, nil)
	assert.Nil(t, err)

	// the summary rows below the closing "****" line are not transactions
	assert.Equal(t, 5, len(allNewTransactions))
	assert.Equal(t, 1, len(allNewAccounts))
	assert.Equal(t, 1, len(allNewSubExpenseCategories))
	assert.Equal(t, 1, len(allNewSubIncomeCategories))
	assert.Equal(t, 0, len(allNewSubTransferCategories))
	assert.Equal(t, 0, len(allNewTags))

	// account name uses the last 4 digits of the account number, currency comes from the statement, not the user default
	assert.Equal(t, "HDFC Bank 9999", allNewAccounts[0].Name)
	assert.Equal(t, "INR", allNewAccounts[0].Currency)

	assert.Equal(t, models.TRANSACTION_DB_TYPE_EXPENSE, allNewTransactions[0].Type)
	assert.Equal(t, "2026-04-01 00:00:00", utils.FormatUnixTimeToLongDateTime(utils.GetUnixTimeFromTransactionTime(allNewTransactions[0].TransactionTime), time.UTC))
	assert.Equal(t, int64(25000), allNewTransactions[0].Amount)
	assert.Equal(t, "HDFC Bank 9999", allNewTransactions[0].OriginalSourceAccountName)
	assert.Equal(t, "UPI-TEST GROCERY STORE-TESTSTORE@OKAXIS", allNewTransactions[0].Comment)

	assert.Equal(t, models.TRANSACTION_DB_TYPE_INCOME, allNewTransactions[1].Type)
	assert.Equal(t, "2026-04-03 00:00:00", utils.FormatUnixTimeToLongDateTime(utils.GetUnixTimeFromTransactionTime(allNewTransactions[1].TransactionTime), time.UTC))
	assert.Equal(t, int64(5000000), allNewTransactions[1].Amount)

	// the narration wrapped onto the following row is joined
	assert.Equal(t, models.TRANSACTION_DB_TYPE_EXPENSE, allNewTransactions[2].Type)
	assert.Equal(t, int64(200000), allNewTransactions[2].Amount)
	assert.Equal(t, "ATW-000000XXXXXX0000-TEST ATM CASH WITHDRAWAL", allNewTransactions[2].Comment)

	assert.Equal(t, int64(14950), allNewTransactions[3].Amount)

	assert.Equal(t, models.TRANSACTION_DB_TYPE_INCOME, allNewTransactions[4].Type)
	assert.Equal(t, "2026-04-30 00:00:00", utils.FormatUnixTimeToLongDateTime(utils.GetUnixTimeFromTransactionTime(allNewTransactions[4].TransactionTime), time.UTC))
	assert.Equal(t, int64(1235), allNewTransactions[4].Amount)
}

func TestHdfcBankTransactionDataXlsImporterParseImportedData_NotHdfcStatement(t *testing.T) {
	importer := HdfcBankTransactionDataXlsFileImporter
	context := core.NewNullContext()

	user := &models.User{
		Uid:             1234567890,
		DefaultCurrency: "INR",
	}

	testdata, err := os.ReadFile("../../../testdata/simple_excel_file.xls")
	assert.Nil(t, err)

	_, _, _, _, _, _, err = importer.ParseImportedData(context, user, testdata, time.UTC, converter.DefaultImporterOptions, nil, nil, nil, nil, nil)
	assert.EqualError(t, err, errs.ErrInvalidFileHeader.Message)
}

func TestParseHdfcBankAmount(t *testing.T) {
	cases := map[string]int64{
		"":                   0,
		"250":                25000,
		"12.5":               1250,
		"1,234.56":           123456,
		"1234.5600000000002": 123456,
	}

	for input, expected := range cases {
		actual, err := parseHdfcBankAmount(input)
		assert.Nil(t, err, input)
		assert.Equal(t, expected, actual, input)
	}

	_, err := parseHdfcBankAmount("abc")
	assert.NotNil(t, err)
}
