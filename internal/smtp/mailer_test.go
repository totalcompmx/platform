package smtp

import (
	"errors"
	"strings"
	"testing"

	"github.com/jcroyoaun/totalcompmx/assets"
	"github.com/jcroyoaun/totalcompmx/internal/assert"
	"github.com/jcroyoaun/totalcompmx/internal/funcs"

	"github.com/resend/resend-go/v2"
	textTemplate "text/template"
)

func TestNewMailer(t *testing.T) {
	t.Run("Create mailer with valid configuration successfully", func(t *testing.T) {
		mailer := NewMailer("test_api_key", "from@example.com")

		assert.NotNil(t, mailer)
		assert.Equal(t, mailer.from, "from@example.com")
		assert.NotNil(t, mailer.client)
		assert.Equal(t, mailer.mockSend, false)
	})
}

func TestNewMockMailer(t *testing.T) {
	t.Run("Create mock mailer successfully", func(t *testing.T) {
		mailer := NewMockMailer("mock@example.com")

		assert.NotNil(t, mailer)
		assert.Equal(t, mailer.from, "mock@example.com")
		assert.Equal(t, mailer.mockSend, true)
		assert.Equal(t, len(mailer.SentMessages), 0)
	})
}

func TestSend(t *testing.T) {
	t.Run("Send email successfully with mock mailer", func(t *testing.T) {
		mailer := NewMockMailer("sender@example.com")

		err := mailer.Send("recipient@example.com", "test data", "testdata/test.tmpl")
		assert.Nil(t, err)
		assert.Equal(t, len(mailer.SentMessages), 1)
		assert.True(t, strings.Contains(mailer.SentMessages[0], "From: sender@example.com"))
		assert.True(t, strings.Contains(mailer.SentMessages[0], "To: recipient@example.com"))
		assert.True(t, strings.Contains(mailer.SentMessages[0], "Subject: Test subject"))
		assert.True(t, strings.Contains(mailer.SentMessages[0], "This is a test plaintext email with TEST DATA"))
	})

	t.Run("Send multiple emails and track all messages", func(t *testing.T) {
		mailer := NewMockMailer("sender@example.com")

		err := mailer.Send("recipient1@example.com", "test data", "testdata/test.tmpl")
		assert.Nil(t, err)

		err = mailer.Send("recipient2@example.com", "test data", "testdata/test.tmpl")
		assert.Nil(t, err)
		assert.Equal(t, len(mailer.SentMessages), 2)
		assert.True(t, strings.Contains(mailer.SentMessages[0], "To: recipient1@example.com"))
		assert.True(t, strings.Contains(mailer.SentMessages[1], "To: recipient2@example.com"))
	})

	t.Run("Returns error for non-existent email template file", func(t *testing.T) {
		mailer := NewMockMailer("sender@example.com")

		err := mailer.Send("recipient@example.com", nil, "testdata/non-existent-file.tmpl")
		assert.NotNil(t, err)
	})

	t.Run("Sends email through resend sender", func(t *testing.T) {
		sender := &fakeEmailSender{}
		mailer := NewMailer("test_api_key", "sender@example.com")
		mailer.emailSender = sender

		err := mailer.Send("recipient@example.com", "test data", "testdata/test.tmpl")
		assert.Nil(t, err)
		assert.Equal(t, "sender@example.com", sender.request.From)
		assert.Equal(t, "recipient@example.com", sender.request.To[0])
		assert.Equal(t, "Test subject", sender.request.Subject)
		assert.True(t, strings.Contains(sender.request.Html, "TEST DATA"))
	})

	t.Run("Returns resend send error", func(t *testing.T) {
		mailer := NewMailer("test_api_key", "sender@example.com")
		mailer.emailSender = &fakeEmailSender{err: errors.New("send failed")}

		err := mailer.Send("recipient@example.com", "test data", "testdata/test.tmpl")
		assert.NotNil(t, err)
	})

	t.Run("Sends plain text resend email without HTML", func(t *testing.T) {
		sender := &fakeEmailSender{}
		mailer := NewMailer("test_api_key", "sender@example.com")
		mailer.emailSender = sender

		err := mailer.Send("recipient@example.com", "test data", "testdata/plain.tmpl")
		assert.Nil(t, err)
		assert.Equal(t, "", sender.request.Html)
	})

	t.Run("Returns error when subject template is missing", func(t *testing.T) {
		mailer := NewMockMailer("sender@example.com")

		err := mailer.Send("recipient@example.com", "test data", "testdata/missing-subject.tmpl")
		assert.NotNil(t, err)
	})

	t.Run("Returns error when plain body template is missing", func(t *testing.T) {
		mailer := NewMockMailer("sender@example.com")

		err := mailer.Send("recipient@example.com", "test data", "testdata/missing-plain.tmpl")
		assert.NotNil(t, err)
	})
}

func TestExecuteHTMLTemplate(t *testing.T) {
	t.Run("Returns parse error for missing HTML template pattern", func(t *testing.T) {
		message, err := renderEmail("test data", emailTemplatePatterns([]string{"testdata/test.tmpl"}))
		assert.Nil(t, err)

		ts, err := parseEmailTextTemplate("testdata/test.tmpl")
		assert.Nil(t, err)
		assert.True(t, message.HTML != "")

		_, err = executeHTMLTemplate(ts, "test data", []string{"emails/testdata/non-existent.tmpl"})
		assert.NotNil(t, err)
	})
}

type fakeEmailSender struct {
	request *resend.SendEmailRequest
	err     error
}

func (s *fakeEmailSender) Send(params *resend.SendEmailRequest) (*resend.SendEmailResponse, error) {
	s.request = params
	return &resend.SendEmailResponse{Id: "test-id"}, s.err
}

func parseEmailTextTemplate(pattern string) (*textTemplate.Template, error) {
	return textTemplate.New("").Funcs(funcs.TemplateFuncs).ParseFS(assets.EmbeddedFiles, "emails/"+pattern)
}
