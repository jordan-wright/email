//Email is designed to be a package providing an "email interface for humans."
//Designed to be robust and flexible, the email package aims to make sending email easy without getting in the way.
package email

import (
	"io/ioutil"
	"log"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
)

//Email is the type used for email messages
type Email struct {
	From        string
	To          []string
	Bcc         []string
	Cc          []string
	Subject     string
	Text        string //Plaintext message (optional)
	Html        string //Html message (optional)
	Headers     []mail.Header
	Attachments map[string]*Attachment
}

//NewEmail creates an Email, and returns the pointer to it.
func NewEmail() *Email {
	return &Email{Attachments: make(map[string]*Attachment)}
}

//Attach is used to attach a file to the email.
//It attempts to open the file reference by filename and, if successful, creates an Attachment.
//This Attachment is then appended to the slice of Email.Attachments.
//The function will then return the Attachment for reference, as well as nil for the error if successful.
func (e *Email) Attach(filename string) (a *Attachment, err error) {
	//Check if the file exists, return any error
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Fatal("%s does not exist", filename)
		return nil, err
	}
	buffer, _ := ioutil.ReadFile(filename)
	e.Attachments[filename] = &Attachment{
		Filename: filename,
		Header:   textproto.MIMEHeader{},
		Content:  buffer}
	return e.Attachments[filename], nil
}

//Bytes converts the Email object to a []byte representation of it, including all needed MIMEHeaders, boundaries, etc.
func (e *Email) Bytes() []byte {
	return []byte{}
}

//Send an email using the given host and SMTP auth (optional)
func (e *Email) Send(addr string, a smtp.Auth) {

}

//Attachment is a struct representing an email attachment.
//Based on the mime/multipart.FileHeader struct, Attachment contains the name, MIMEHeader, and content of the attachment in question
type Attachment struct {
	Filename string
	Header   textproto.MIMEHeader
	Content  []byte
}
