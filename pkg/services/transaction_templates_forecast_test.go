package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestForecastTemplateIsSchedule(t *testing.T) {
	tpl := &models.TransactionTemplate{TemplateType: models.TRANSACTION_TEMPLATE_TYPE_FORECAST}
	assert.True(t, tpl.HasSchedule())
	assert.True(t, (&models.TransactionTemplate{TemplateType: models.TRANSACTION_TEMPLATE_TYPE_SCHEDULE}).HasSchedule())
	assert.False(t, (&models.TransactionTemplate{TemplateType: models.TRANSACTION_TEMPLATE_TYPE_NORMAL}).HasSchedule())
}

func TestScheduledCronIgnoresForecastTemplates(t *testing.T) {
	// The cron query filters template_type = SCHEDULE. Pin it so nobody widens it to "has schedule".
	src := readSource(t, "transactions.go")
	assert.Contains(t, src, `" AND template_type=?"`)
	assert.Contains(t, src, "models.TRANSACTION_TEMPLATE_TYPE_SCHEDULE")
}

func readSource(t *testing.T, name string) string {
	b, err := os.ReadFile(name)

	if err != nil {
		// go test's working directory is not guaranteed to be this package's
		// directory in every environment (e.g. when run from the repo root);
		// fall back to the package-relative path.
		b, err = os.ReadFile(filepath.Join("pkg", "services", name))
	}

	assert.NoError(t, err)
	return string(b)
}
