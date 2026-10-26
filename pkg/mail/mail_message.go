package mail

// MailAttachment represents a file attached to an email
type MailAttachment struct {
	Name    string
	Content []byte
}

// MailMessage represents an email entity
type MailMessage struct {
	To          string
	Subject     string
	Body        string
	Attachments []MailAttachment
}
