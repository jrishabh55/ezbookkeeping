package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

var alertTestDbReady bool

// newAlertTestUser creates a user with the given INR accounts in a temporary sqlite database
func newAlertTestUser(t *testing.T, accountNames ...string) (core.Context, int64) {
	t.Helper()
	ctx := core.NewNullContext()
	if !alertTestDbReady {
		// Deliberately not t.TempDir(): that directory is removed when the *first* test that
		// created it finishes, but this database is shared (via alertTestDbReady) by every
		// test in the package, so its directory must outlive any single test.
		dir, err := os.MkdirTemp("", "alerts-test-*")
		if err != nil {
			t.Fatal(err)
		}
		cfg := &settings.Config{DatabaseConfig: &settings.DatabaseConfig{DatabaseType: settings.Sqlite3DbType,
			DatabasePath: filepath.Join(dir, "alerts.db"), MaxIdleConnection: 2, ConnectionMaxLifeTime: 14400}, UuidGeneratorType: settings.InternalUuidGeneratorType}
		settings.SetCurrentConfig(cfg)
		if err := datastore.InitializeDataStore(cfg); err != nil {
			t.Fatal(err)
		}
		if err := uuid.InitializeUuidGenerator(cfg); err != nil {
			t.Fatal(err)
		}
		for _, m := range []any{new(models.User), new(models.Account), new(models.Transaction), new(models.TransactionCategory), new(models.TransactionTag),
			new(models.TransactionTagIndex), new(models.AlertMessage)} {
			if err := datastore.Container.UserDataStore.SyncStructs(m); err != nil {
				t.Fatal(err)
			}
		}
		alertTestDbReady = true
	}
	username := "u" + strings.ReplaceAll(time.Now().Format("150405.000000000"), ".", "")
	// The brief's helper used a fixed email, but CreateUser rejects a duplicate email and the
	// underlying sqlite database is shared (and kept) across every test in this package, so
	// each test user needs its own email too.
	user := &models.User{Username: username, Email: username + "@example.com", Nickname: "x", Password: "test123456", DefaultCurrency: "INR"}
	if err := Users.CreateUser(ctx, user, false); err != nil {
		t.Fatal(err)
	}
	for _, name := range accountNames {
		a := &models.Account{Uid: user.Uid, Name: name, Category: models.ACCOUNT_CATEGORY_SAVINGS_ACCOUNT, Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Icon: 1, Color: "000000", Currency: "INR"}
		if err := Accounts.CreateAccounts(ctx, a, 0, nil, nil, time.UTC); err != nil {
			t.Fatal(err)
		}
	}
	return ctx, user.Uid
}

func countTransactions(t *testing.T, ctx core.Context, uid int64) int {
	n, err := datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).Where("uid=? AND deleted=? AND type<>?", uid, false, models.TRANSACTION_DB_TYPE_TRANSFER_IN).Count(&models.Transaction{})
	if err != nil {
		t.Fatal(err)
	}
	return int(n)
}

func accountNameOf(t *testing.T, ctx core.Context, uid int64, transactionId int64) string {
	tx, err := Transactions.GetTransactionByTransactionId(ctx, uid, transactionId)
	if err != nil {
		t.Fatal(err)
	}
	a, err := Accounts.GetAccountByAccountId(ctx, uid, tx.AccountId)
	if err != nil {
		t.Fatal(err)
	}
	return a.Name
}

func accountIdByName(t *testing.T, ctx core.Context, uid int64, name string) int64 {
	all, err := Accounts.GetAllAccountsByUid(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range all {
		if a.Name == name {
			return a.AccountId
		}
	}
	t.Fatalf("no account %q", name)
	return 0
}

func tagNamesOf(t *testing.T, ctx core.Context, uid int64, transactionId int64) []string {
	idx, err := TransactionTags.GetAllTagIdsOfTransactions(ctx, uid, []int64{transactionId})
	if err != nil {
		t.Fatal(err)
	}
	tags, err := TransactionTags.GetTagsByTagIds(ctx, uid, idx[transactionId])
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, tg := range tags {
		names = append(names, tg.Name)
	}
	return names
}

func importRow(uid int64, accountName string, typ models.TransactionDbType, amount int64, at time.Time, narration string) *models.ImportTransaction {
	return &models.ImportTransaction{Transaction: &models.Transaction{Uid: uid, Type: typ, Amount: amount, TransactionTime: at.Unix() * 1000, Comment: narration},
		OriginalSourceAccountName: accountName}
}
