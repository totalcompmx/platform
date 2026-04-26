package smtp

import (
	"bytes"

	"github.com/jcroyoaun/totalcompmx/assets"
	"github.com/jcroyoaun/totalcompmx/internal/funcs"

	"github.com/resend/resend-go/v2"

	htmlTemplate "html/template"
	textTemplate "text/template"
)

type Mailer struct {
	client       *resend.Client
	emailSender  resendEmailSender
	from         string
	mockSend     bool
	SentMessages []string
}

type resendEmailSender interface {
	Send(*resend.SendEmailRequest) (*resend.SendEmailResponse, error)
}

// NewMailer creates a new Mailer using Resend API
func NewMailer(apiKey, from string) *Mailer {
	client := resend.NewClient(apiKey)
	return &Mailer{
		client:      client,
		emailSender: client.Emails,
		from:        from,
	}
}

// NewMockMailer creates a mock mailer for testing
func NewMockMailer(from string) *Mailer {
	return &Mailer{
		from:     from,
		mockSend: true,
	}
}

// Send sends an email using Resend
// patterns should be template file paths relative to assets/emails/
func (m *Mailer) Send(recipient string, data any, patterns ...string) error {
	patterns = emailTemplatePatterns(patterns)
	message, err := renderEmail(data, patterns)
	if err != nil {
		return err
	}
	if m.mockSend {
		m.recordMockEmail(recipient, message)
		return nil
	}
	return m.sendResendEmail(recipient, message)
}

type emailMessage struct {
	Subject string
	Plain   string
	HTML    string
}

func emailTemplatePatterns(patterns []string) []string {
	for i := range patterns {
		patterns[i] = "emails/" + patterns[i]
	}
	return patterns
}

func renderEmail(data any, patterns []string) (emailMessage, error) {
	ts, err := textTemplate.New("").Funcs(funcs.TemplateFuncs).ParseFS(assets.EmbeddedFiles, patterns...)
	if err != nil {
		return emailMessage{}, err
	}
	subject, err := executeTextTemplate(ts, "subject", data)
	if err != nil {
		return emailMessage{}, err
	}
	plainBody, err := executeTextTemplate(ts, "plainBody", data)
	if err != nil {
		return emailMessage{}, err
	}
	htmlBody, err := executeHTMLTemplate(ts, data, patterns)
	return emailMessage{Subject: subject, Plain: plainBody, HTML: htmlBody}, err
}

func executeTextTemplate(ts *textTemplate.Template, name string, data any) (string, error) {
	buf := new(bytes.Buffer)
	err := ts.ExecuteTemplate(buf, name, data)
	return buf.String(), err
}

func executeHTMLTemplate(ts *textTemplate.Template, data any, patterns []string) (string, error) {
	if ts.Lookup("htmlBody") == nil {
		return "", nil
	}
	htmlTs, err := htmlTemplate.New("").Funcs(funcs.TemplateFuncs).ParseFS(assets.EmbeddedFiles, patterns...)
	if err != nil {
		return "", err
	}
	htmlBuf := new(bytes.Buffer)
	err = htmlTs.ExecuteTemplate(htmlBuf, "htmlBody", data)
	return htmlBuf.String(), err
}

func (m *Mailer) recordMockEmail(recipient string, message emailMessage) {
	mockMessage := "To: " + recipient + "\n" +
		"From: " + m.from + "\n" +
		"Subject: " + message.Subject + "\n\n" +
		message.Plain
	m.SentMessages = append(m.SentMessages, mockMessage)
}

func (m *Mailer) sendResendEmail(recipient string, message emailMessage) error {
	params := &resend.SendEmailRequest{
		From:    m.from,
		To:      []string{recipient},
		Subject: message.Subject,
		Text:    message.Plain,
	}
	if message.HTML != "" {
		params.Html = message.HTML
	}
	_, err := m.emailSender.Send(params)
	return err
}
