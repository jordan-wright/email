//Email is designed to be a package providing an "email interface for humans."
//Designed to be robust and flexible, the email package aims to make sending email easy without getting in the way.
package email

import (
	"net/mail"
)

//Email is the type used for email messages
type Email struct {
	From        string
	To          []string
	Bcc         []string
	Cc          []string
	Subject     []string
	Text        []byte //Plaintext message (optional)
	Html        []byte //Html message (optional)
	Headers     []mail.Header
	Attachments []Attachment //Might be a map soon - stay tuned.
}

//Send an email using the given host and SMTP auth (optional)
func (e Email) Send() {

}

type Attachment struct {
	Filename string
}
