package notify

import (
	"bytes"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"qr-feedback-api/internal/config"
)

type Mailer struct {
	cfg config.Config
}

// NewMailer constructs a Mailer with the full app config.
func NewMailer(cfg config.Config) *Mailer {
	return &Mailer{cfg: cfg}
}

// SendReviewRequired sends only a minimal operational alert. Raw narratives,
// contact details and rule matches stay inside the authenticated review flow.
func (m *Mailer) SendReviewRequired(receiptID, intakeType, reviewPriority string) error {
	// If SMTP is not configured, skip sending (no error in dev)
	if m.cfg.SMTPHost == "" || m.cfg.SMTPUser == "" || m.cfg.SMTPPass == "" || len(m.cfg.NotifyTo) == 0 {
		return nil
	}

	from := m.cfg.SMTPUser
	to := m.cfg.NotifyTo

	subject := fmt.Sprintf("%s New feedback requires review", m.cfg.EmailSubjectPrefix)

	var body bytes.Buffer
	fmt.Fprintln(&body, "New feedback requires human review.")
	fmt.Fprintf(&body, "Receipt reference: %s\n", receiptID)
	fmt.Fprintf(&body, "Intake type: %s\n", intakeType)
	fmt.Fprintf(&body, "Review priority: %s\n", reviewPriority)
	fmt.Fprintln(&body, "Open the authenticated Feedback Review area to assess it.")

	// Build raw email
	var msg bytes.Buffer
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ",")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	msg.WriteString("\r\n")
	msg.Write(body.Bytes())

	addr := fmt.Sprintf("%s:%d", m.cfg.SMTPHost, m.cfg.SMTPPort)
	auth := smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPass, m.cfg.SMTPHost)

	return smtp.SendMail(addr, auth, from, to, msg.Bytes())
}

// SendHomeFeedbackReceived notifies the configured home-management recipients
// for every new visitor submission. It deliberately excludes narrative and
// contact fields; managers open the authenticated product to read the record.
func (m *Mailer) SendHomeFeedbackReceived(receiptID, home, submitterType, reviewPriority string, receivedAt time.Time) error {
	if m.cfg.SMTPHost == "" || m.cfg.SMTPUser == "" || m.cfg.SMTPPass == "" || len(m.cfg.NotifyTo) == 0 {
		return nil
	}

	subject, body := homeFeedbackNotification(m.cfg, receiptID, home, submitterType, reviewPriority, receivedAt)
	from := m.cfg.SMTPUser
	to := m.cfg.NotifyTo
	var msg bytes.Buffer
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(to, ",")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)

	addr := fmt.Sprintf("%s:%d", m.cfg.SMTPHost, m.cfg.SMTPPort)
	auth := smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPass, m.cfg.SMTPHost)
	return smtp.SendMail(addr, auth, from, to, msg.Bytes())
}

func homeFeedbackNotification(cfg config.Config, receiptID, home, submitterType, reviewPriority string, receivedAt time.Time) (string, string) {
	subject := fmt.Sprintf("%s New visitor feedback received", cfg.EmailSubjectPrefix)
	var body strings.Builder
	body.WriteString("New visitor feedback received.\n")
	fmt.Fprintf(&body, "Home: %s\n", home)
	fmt.Fprintf(&body, "Visitor type: %s\n", submitterType)
	fmt.Fprintf(&body, "Received: %s\n", receivedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&body, "Reference: %s\n", receiptID)
	if reviewPriority != "" {
		fmt.Fprintf(&body, "Review priority: %s\n", reviewPriority)
	}
	if cfg.ManagerReviewURL != "" {
		fmt.Fprintf(&body, "Open securely: %s\n", cfg.ManagerReviewURL)
	} else {
		body.WriteString("Open the authenticated Visitor Feedback area to review it.\n")
	}
	return subject, body.String()
}
