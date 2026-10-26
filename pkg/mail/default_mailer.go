package mail

import (
	"bytes"
	"crypto/tls"
	"net"

	"gopkg.in/mail.v2"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// DefaultMailer represents default mailer
type DefaultMailer struct {
	dialer      *mail.Dialer
	fromAddress string
}

// NewDefaultMailer returns a new default mailer
func NewDefaultMailer(smtpConfig *settings.SMTPConfig) (*DefaultMailer, error) {
	host, portStr, err := net.SplitHostPort(smtpConfig.SMTPHost)

	if err != nil {
		return nil, errs.ErrSMTPServerHostInvalid
	}

	port, err := utils.StringToInt(portStr)

	if err != nil {
		return nil, errs.ErrSMTPServerHostInvalid
	}

	dialer := mail.NewDialer(host, port, smtpConfig.SMTPUser, smtpConfig.SMTPPasswd)
	dialer.TLSConfig = &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: smtpConfig.SMTPSkipTLSVerify,
	}

	mailer := &DefaultMailer{
		dialer:      dialer,
		fromAddress: smtpConfig.FromAddress,
	}

	return mailer, nil
}

// SendMail sends an email according to argument
func (m *DefaultMailer) SendMail(message *MailMessage) error {
	if m.dialer == nil {
		return errs.ErrSMTPServerNotEnabled
	}

	err := m.dialer.DialAndSend(m.buildMessage(message))

	return err
}

// buildMessage converts a mail message into the gopkg.in/mail.v2 entity with its attachments
func (m *DefaultMailer) buildMessage(message *MailMessage) *mail.Message {
	mailMessage := mail.NewMessage()
	mailMessage.SetHeader("From", m.fromAddress)
	mailMessage.SetHeader("To", message.To)
	mailMessage.SetHeader("Subject", message.Subject)
	mailMessage.SetBody("text/html", message.Body)

	for _, attachment := range message.Attachments {
		mailMessage.AttachReader(attachment.Name, bytes.NewReader(attachment.Content))
	}

	return mailMessage
}
