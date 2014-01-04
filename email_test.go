package email

import (
	"net/smtp"
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
	e.HTML = "<h1>Fancy HTML is supported, too!</h1>"
}

func ExampleGmail() {
	e := NewEmail()
	e.From = "Jordan Wright <test@gmail.com>"
	e.To = []string{"test@example.com"}
	e.Bcc = []string{"test_bcc@example.com"}
	e.Cc = []string{"test_cc@example.com"}
	e.Subject = "Awesome Subject"
	e.Text = "Text Body is, of course, supported!"
	e.HTML = "<h1>Fancy HTML is supported, too!</h1>"
	e.Send("smtp.gmail.com:587", smtp.PlainAuth("", e.From, "password123", "smtp.gmail.com"))
}

func ExampleAttach() {
	e := NewEmail()
	e.Attach("test.txt")
}
