package models

// AlertMessage represents alert message data stored in database
type AlertMessage struct {
	AlertId          int64  `xorm:"PK"`
	Uid              int64  `xorm:"INDEX(IDX_alert_message_uid_ref) INDEX(IDX_alert_message_uid_time) NOT NULL"`
	Sender           string `xorm:"VARCHAR(32) NOT NULL"`
	Text             string `xorm:"VARCHAR(2048) NOT NULL"`
	Reference        string `xorm:"INDEX(IDX_alert_message_uid_ref) VARCHAR(32) NOT NULL"`
	PayeeKey         string `xorm:"VARCHAR(64) NOT NULL DEFAULT ''"`
	TextHash         string `xorm:"VARCHAR(16) NOT NULL DEFAULT ''"`
	Outcome          string `xorm:"VARCHAR(16) NOT NULL"`
	TransactionId    int64  `xorm:"NOT NULL DEFAULT 0"`
	ReceivedUnixTime int64  `xorm:"INDEX(IDX_alert_message_uid_time) NOT NULL"`
	CreatedUnixTime  int64
}

// AlertIngestRequest represents all parameters of alert ingestion request
type AlertIngestRequest struct {
	Sender     string `json:"sender" binding:"max=32"`
	Text       string `json:"text" binding:"required,max=2048"`
	ReceivedAt int64  `json:"receivedAt" binding:"min=0"`
}

// AlertIngestResponse represents the response of alert ingestion
type AlertIngestResponse struct {
	Result  string `json:"result"`
	Summary string `json:"summary,omitempty"`
}

// AlertTokenCreateRequest represents all parameters of alert token creation request
type AlertTokenCreateRequest struct {
	Password string `json:"password" binding:"required"`
}

// AlertTokenCreateResponse represents the response of alert token creation
type AlertTokenCreateResponse struct {
	Token       string `json:"token"`
	ShortcutUrl string `json:"shortcutUrl"`
}

// AlertStatusResponse represents the response of alert status query
type AlertStatusResponse struct {
	Configured     bool                 `json:"configured"`
	LastReceivedAt int64                `json:"lastReceivedAt"`
	LastOutcome    string               `json:"lastOutcome"`
	Counts         map[string]int64     `json:"counts"`
	ShortcutUrl    string               `json:"shortcutUrl"`
	RecentUnparsed []*AlertUnparsedItem `json:"recentUnparsed"`
}

// AlertUnparsedItem represents one recent alert message that could not be read, so the user can
// add it manually
type AlertUnparsedItem struct {
	ReceivedAt int64  `json:"receivedAt"`
	Sender     string `json:"sender"`
	Text       string `json:"text"`
}
