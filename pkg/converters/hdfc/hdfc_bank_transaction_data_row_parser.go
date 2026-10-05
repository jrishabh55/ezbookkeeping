package hdfc

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/converters/datatable"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

const hdfcBankAccountNamePrefix = "HDFC Bank"
const hdfcBankDefaultCurrency = "INR"
const hdfcBankTransactionDateFormat = "02/01/06"

const hdfcBankTransactionDateColumnName = "Date"
const hdfcBankTransactionNarrationColumnName = "Narration"
const hdfcBankTransactionWithdrawalAmountColumnName = "Withdrawal Amt."
const hdfcBankTransactionDepositAmountColumnName = "Deposit Amt."

var hdfcBankTransactionSupportedColumns = map[datatable.TransactionDataTableColumn]bool{
	datatable.TRANSACTION_DATA_TABLE_TRANSACTION_TIME:     true,
	datatable.TRANSACTION_DATA_TABLE_TRANSACTION_TYPE:     true,
	datatable.TRANSACTION_DATA_TABLE_SUB_CATEGORY:         true,
	datatable.TRANSACTION_DATA_TABLE_ACCOUNT_NAME:         true,
	datatable.TRANSACTION_DATA_TABLE_ACCOUNT_CURRENCY:     true,
	datatable.TRANSACTION_DATA_TABLE_AMOUNT:               true,
	datatable.TRANSACTION_DATA_TABLE_RELATED_ACCOUNT_NAME: true,
	datatable.TRANSACTION_DATA_TABLE_DESCRIPTION:          true,
}

var hdfcBankTransactionTypeNameMapping = map[models.TransactionType]string{
	models.TRANSACTION_TYPE_INCOME:  hdfcBankTransactionDepositAmountColumnName,
	models.TRANSACTION_TYPE_EXPENSE: hdfcBankTransactionWithdrawalAmountColumnName,
}

// hdfcBankTransactionDataRowParser defines the structure of hdfc bank transaction data row parser
type hdfcBankTransactionDataRowParser struct {
	accountName string
	currency    string
}

// Parse returns the converted transaction data row
// ponytail: withdrawals become expenses and deposits become income; self transfers and credit card payments are not detected, change their type in the import review
func (p *hdfcBankTransactionDataRowParser) Parse(ctx core.Context, user *models.User, dataRow datatable.CommonDataTableRow, rowId string) (rowData map[datatable.TransactionDataTableColumn]string, rowDataValid bool, err error) {
	transactionDate, err := time.Parse(hdfcBankTransactionDateFormat, dataRow.GetData(hdfcBankTransactionDateColumnName))

	if err != nil {
		log.Errorf(ctx, "[hdfc_bank_transaction_data_row_parser.Parse] cannot parse date \"%s\" of transaction in row \"%s\"", dataRow.GetData(hdfcBankTransactionDateColumnName), rowId)
		return nil, false, errs.ErrTransactionTimeInvalid
	}

	withdrawalAmount, err := parseHdfcBankAmount(dataRow.GetData(hdfcBankTransactionWithdrawalAmountColumnName))

	if err != nil {
		log.Errorf(ctx, "[hdfc_bank_transaction_data_row_parser.Parse] cannot parse withdrawal amount \"%s\" of transaction in row \"%s\"", dataRow.GetData(hdfcBankTransactionWithdrawalAmountColumnName), rowId)
		return nil, false, errs.ErrAmountInvalid
	}

	depositAmount, err := parseHdfcBankAmount(dataRow.GetData(hdfcBankTransactionDepositAmountColumnName))

	if err != nil {
		log.Errorf(ctx, "[hdfc_bank_transaction_data_row_parser.Parse] cannot parse deposit amount \"%s\" of transaction in row \"%s\"", dataRow.GetData(hdfcBankTransactionDepositAmountColumnName), rowId)
		return nil, false, errs.ErrAmountInvalid
	}

	transactionType := ""
	amount := int64(0)

	if withdrawalAmount > 0 && depositAmount == 0 {
		transactionType = hdfcBankTransactionTypeNameMapping[models.TRANSACTION_TYPE_EXPENSE]
		amount = withdrawalAmount
	} else if depositAmount > 0 && withdrawalAmount == 0 {
		transactionType = hdfcBankTransactionTypeNameMapping[models.TRANSACTION_TYPE_INCOME]
		amount = depositAmount
	} else {
		log.Warnf(ctx, "[hdfc_bank_transaction_data_row_parser.Parse] skip parsing transaction in row \"%s\", because it does not have exactly one of withdrawal and deposit amount", rowId)
		return nil, false, nil
	}

	return map[datatable.TransactionDataTableColumn]string{
		datatable.TRANSACTION_DATA_TABLE_TRANSACTION_TIME:     transactionDate.Format("2006-01-02") + " 00:00:00",
		datatable.TRANSACTION_DATA_TABLE_TRANSACTION_TYPE:     transactionType,
		datatable.TRANSACTION_DATA_TABLE_SUB_CATEGORY:         "",
		datatable.TRANSACTION_DATA_TABLE_ACCOUNT_NAME:         p.accountName,
		datatable.TRANSACTION_DATA_TABLE_ACCOUNT_CURRENCY:     p.currency,
		datatable.TRANSACTION_DATA_TABLE_AMOUNT:               utils.FormatAmount(amount),
		datatable.TRANSACTION_DATA_TABLE_RELATED_ACCOUNT_NAME: "",
		datatable.TRANSACTION_DATA_TABLE_DESCRIPTION:          dataRow.GetData(hdfcBankTransactionNarrationColumnName),
	}, true, nil
}

// parseHdfcBankAmount parses amounts like "1,234.5" or "" (no amount) into the amount in the smallest currency unit,
// rounding because excel number cells can carry float noise like "1234.5600000000002"
func parseHdfcBankAmount(amount string) (int64, error) {
	amount = strings.ReplaceAll(amount, ",", "")

	if amount == "" {
		return 0, nil
	}

	value, err := strconv.ParseFloat(amount, 64)

	if err != nil || value < 0 {
		return 0, errs.ErrAmountInvalid
	}

	return int64(math.Round(value * 100)), nil
}

// createHdfcBankTransactionDataRowParser returns hdfc bank transaction data row parser
func createHdfcBankTransactionDataRowParser(accountName string, currency string) datatable.CommonTransactionDataRowParser {
	if currency == "" {
		currency = hdfcBankDefaultCurrency
	}

	return &hdfcBankTransactionDataRowParser{
		accountName: accountName,
		currency:    currency,
	}
}
