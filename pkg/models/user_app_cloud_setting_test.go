package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllowedCloudSettingKeysIncludeExtraDashboards(t *testing.T) {
	for _, key := range []string{"desktopOverviewExtraDashboards", "mobileOverviewExtraDashboards"} {
		settingType, exists := ALL_ALLOWED_CLOUD_SYNC_APP_SETTING_KEY_TYPES[key]
		assert.True(t, exists, key)
		assert.Equal(t, USER_APPLICATION_CLOUD_SETTING_TYPE_STRING, settingType, key)
	}
}
