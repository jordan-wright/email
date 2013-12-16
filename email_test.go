package email

import (
	"testing"
)

func TestEmail(*testing.T) {
	e := NewEmail()
	e.From = "Jordan Wright <test@example.com>"
	e.To = []string{"test@example.com"}
	e.Bcc = []string{"test_bcc@example.com"}
	e.Cc = []string{"test_cc@example.com"}
	e.Subject = "Awesome Subject"
	e.Text = "Text Body is, of course, supported!"
	e.Html = "<h1>Fancy Html is supported, too!</h1>"
}
