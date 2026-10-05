package pnb

import (
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/converters/datatable"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

const pnbOneAccountNamePrefix = "PNB"
const pnbOneCurrency = "INR"
const pnbOneTransactionDateFormat = "02/01/2006"

const pnbOneTransactionNumberColumnName = "Txn No."
const pnbOneTransactionDateColumnName = "Txn Date"
const pnbOneTransactionDescriptionColumnName = "Description"
const pnbOneTransactionDebitAmountColumnName = "Dr Amount"
const pnbOneTransactionCreditAmountColumnName = "Cr Amount"

var pnbOneTransactionSupportedColumns = map[datatable.TransactionDataTableColumn]bool{
	datatable.TRANSACTION_DATA_TABLE_TRANSACTION_TIME:     true,
	datatable.TRANSACTION_DATA_TABLE_TRANSACTION_TYPE:     true,
	datatable.TRANSACTION_DATA_TABLE_SUB_CATEGORY:         true,
	datatable.TRANSACTION_DATA_TABLE_ACCOUNT_NAME:         true,
	datatable.TRANSACTION_DATA_TABLE_ACCOUNT_CURRENCY:     true,
	datatable.TRANSACTION_DATA_TABLE_AMOUNT:               true,
	datatable.TRANSACTION_DATA_TABLE_RELATED_ACCOUNT_NAME: true,
	datatable.TRANSACTION_DATA_TABLE_DESCRIPTION:          true,
}

var pnbOneTransactionTypeNameMapping = map[models.TransactionType]string{
	models.TRANSACTION_TYPE_INCOME:  pnbOneTransactionCreditAmountColumnName,
	models.TRANSACTION_TYPE_EXPENSE: pnbOneTransactionDebitAmountColumnName,
}

// pnbOneTransactionDataRowParser defines the structure of pnb one transaction data row parser
type pnbOneTransactionDataRowParser struct {
	accountName string
}

// Parse returns the converted transaction data row
// ponytail: debits become expenses and credits become income; transfers between own accounts are not detected, change their type in the import review
func (p *pnbOneTransactionDataRowParser) Parse(ctx core.Context, user *models.User, dataRow datatable.CommonDataTableRow, rowId string) (rowData map[datatable.TransactionDataTableColumn]string, rowDataValid bool, err error) {
	transactionDate, err := time.Parse(pnbOneTransactionDateFormat, dataRow.GetData(pnbOneTransactionDateColumnName))

	if err != nil {
		log.Errorf(ctx, "[pnb_one_transaction_data_row_parser.Parse] cannot parse date \"%s\" of transaction in row \"%s\"", dataRow.GetData(pnbOneTransactionDateColumnName), rowId)
		return nil, false, errs.ErrTransactionTimeInvalid
	}

	debitAmount, err := utils.ParseAmount(strings.ReplaceAll(dataRow.GetData(pnbOneTransactionDebitAmountColumnName), ",", ""))

	if err != nil || debitAmount < 0 {
		log.Errorf(ctx, "[pnb_one_transaction_data_row_parser.Parse] cannot parse debit amount \"%s\" of transaction in row \"%s\"", dataRow.GetData(pnbOneTransactionDebitAmountColumnName), rowId)
		return nil, false, errs.ErrAmountInvalid
	}

	creditAmount, err := utils.ParseAmount(strings.ReplaceAll(dataRow.GetData(pnbOneTransactionCreditAmountColumnName), ",", ""))

	if err != nil || creditAmount < 0 {
		log.Errorf(ctx, "[pnb_one_transaction_data_row_parser.Parse] cannot parse credit amount \"%s\" of transaction in row \"%s\"", dataRow.GetData(pnbOneTransactionCreditAmountColumnName), rowId)
		return nil, false, errs.ErrAmountInvalid
	}

	transactionType := ""
	amount := int64(0)

	if debitAmount > 0 && creditAmount == 0 {
		transactionType = pnbOneTransactionTypeNameMapping[models.TRANSACTION_TYPE_EXPENSE]
		amount = debitAmount
	} else if creditAmount > 0 && debitAmount == 0 {
		transactionType = pnbOneTransactionTypeNameMapping[models.TRANSACTION_TYPE_INCOME]
		amount = creditAmount
	} else {
		log.Warnf(ctx, "[pnb_one_transaction_data_row_parser.Parse] skip parsing transaction in row \"%s\", because it does not have exactly one of debit and credit amount", rowId)
		return nil, false, nil
	}

	return map[datatable.TransactionDataTableColumn]string{
		datatable.TRANSACTION_DATA_TABLE_TRANSACTION_TIME:     transactionDate.Format("2006-01-02") + " 00:00:00",
		datatable.TRANSACTION_DATA_TABLE_TRANSACTION_TYPE:     transactionType,
		datatable.TRANSACTION_DATA_TABLE_SUB_CATEGORY:         "",
		datatable.TRANSACTION_DATA_TABLE_ACCOUNT_NAME:         p.accountName,
		datatable.TRANSACTION_DATA_TABLE_ACCOUNT_CURRENCY:     pnbOneCurrency,
		datatable.TRANSACTION_DATA_TABLE_AMOUNT:               utils.FormatAmount(amount),
		datatable.TRANSACTION_DATA_TABLE_RELATED_ACCOUNT_NAME: "",
		datatable.TRANSACTION_DATA_TABLE_DESCRIPTION:          dataRow.GetData(pnbOneTransactionDescriptionColumnName),
	}, true, nil
}

// createPnbOneTransactionDataRowParser returns pnb one transaction data row parser
func createPnbOneTransactionDataRowParser(accountName string) datatable.CommonTransactionDataRowParser {
	return &pnbOneTransactionDataRowParser{
		accountName: accountName,
	}
}
