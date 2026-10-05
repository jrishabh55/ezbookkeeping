package pnb

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

func TestPnbOneTransactionDataCsvImporterParseImportedData(t *testing.T) {
	importer := PnbOneTransactionDataCsvFileImporter
	context := core.NewNullContext()

	user := &models.User{
		Uid:             1234567890,
		DefaultCurrency: "USD",
	}

	testdata, err := os.ReadFile("../../../testdata/pnb_one_statement_test_file.csv")
	assert.Nil(t, err)

	allNewTransactions, allNewAccounts, allNewSubExpenseCategories, allNewSubIncomeCategories, allNewSubTransferCategories, allNewTags, err := importer.ParseImportedData(context, user, testdata, time.UTC, converter.DefaultImporterOptions, nil, nil, nil, nil, nil)
	assert.Nil(t, err)

	// the notes after the table (with unbalanced quotes) are not read
	assert.Equal(t, 5, len(allNewTransactions))
	assert.Equal(t, 1, len(allNewAccounts))
	assert.Equal(t, 1, len(allNewSubExpenseCategories))
	assert.Equal(t, 1, len(allNewSubIncomeCategories))
	assert.Equal(t, 0, len(allNewSubTransferCategories))
	assert.Equal(t, 0, len(allNewTags))

	assert.Equal(t, "PNB 9999", allNewAccounts[0].Name)
	assert.Equal(t, "INR", allNewAccounts[0].Currency)

	// imported transactions are in chronological order, the statement is newest first
	assert.Equal(t, models.TRANSACTION_DB_TYPE_EXPENSE, allNewTransactions[0].Type)
	assert.Equal(t, "2026-04-01 00:00:00", utils.FormatUnixTimeToLongDateTime(utils.GetUnixTimeFromTransactionTime(allNewTransactions[0].TransactionTime), time.UTC))
	assert.Equal(t, int64(73820), allNewTransactions[0].Amount)
	assert.Equal(t, "PNB 9999", allNewTransactions[0].OriginalSourceAccountName)
	assert.Equal(t, "IMPS-OUT/000000000002/HDFC0000001/0000000000", allNewTransactions[0].Comment)

	assert.Equal(t, models.TRANSACTION_DB_TYPE_EXPENSE, allNewTransactions[1].Type)
	assert.Equal(t, int64(1180), allNewTransactions[1].Amount)

	assert.Equal(t, models.TRANSACTION_DB_TYPE_INCOME, allNewTransactions[2].Type)
	assert.Equal(t, "2026-04-10 00:00:00", utils.FormatUnixTimeToLongDateTime(utils.GetUnixTimeFromTransactionTime(allNewTransactions[2].TransactionTime), time.UTC))
	assert.Equal(t, int64(5000000), allNewTransactions[2].Amount)

	// quoted description with a comma and an amount with a thousands separator
	assert.Equal(t, models.TRANSACTION_DB_TYPE_EXPENSE, allNewTransactions[3].Type)
	assert.Equal(t, int64(100000), allNewTransactions[3].Amount)
	assert.Equal(t, "NEFT_OUT:TEST0000001/Test User, Savings/HDFC0000001", allNewTransactions[3].Comment)

	assert.Equal(t, models.TRANSACTION_DB_TYPE_INCOME, allNewTransactions[4].Type)
	assert.Equal(t, int64(1250), allNewTransactions[4].Amount)
}

func TestPnbOneTransactionDataCsvImporterParseImportedData_NotPnbStatement(t *testing.T) {
	importer := PnbOneTransactionDataCsvFileImporter
	context := core.NewNullContext()

	user := &models.User{
		Uid:             1234567890,
		DefaultCurrency: "INR",
	}

	_, _, _, _, _, _, err := importer.ParseImportedData(context, user, []byte("Date,Description,Amount\n01/04/2026,Test,100\n"), time.UTC, converter.DefaultImporterOptions, nil, nil, nil, nil, nil)
	assert.EqualError(t, err, errs.ErrInvalidFileHeader.Message)
}
