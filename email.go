//Package email is designed to provide an "email interface for humans."
//Designed to be robust and flexible, the email package aims to make sending email easy without getting in the way.
package email

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
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
	Headers     textproto.MIMEHeader
	Attachments map[string]*Attachment
	ReadReceipt []string
}

//NewEmail creates an Email, and returns the pointer to it.
func NewEmail() *Email {
	return &Email{Attachments: make(map[string]*Attachment), Headers: textproto.MIMEHeader{}}
}

//Attach is used to attach a file to the email.
//It attempts to open the file referenced by filename and, if successful, creates an Attachment.
//This Attachment is then appended to the slice of Email.Attachments.
//The function will then return the Attachment for reference, as well as nil for the error, if successful.
func (e *Email) Attach(filename string) (a *Attachment, err error) {
	//Check if the file exists, return any error
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Fatal("%s does not exist", filename)
		return nil, err
	}
	//Read the file, and set the appropriate headers
	buffer, _ := ioutil.ReadFile(filename)
	e.Attachments[filename] = &Attachment{
		Filename: filename,
		Header:   textproto.MIMEHeader{},
		Content:  buffer}
	at := e.Attachments[filename]
	//Get the Content-Type to be used in the MIMEHeader
	ct := mime.TypeByExtension(filepath.Ext(filename))
	if ct != "" {
		at.Header.Set("Content-Type", ct)
	} else {
		//If the Content-Type is blank, set the Content-Type to "application/octet-stream"
		at.Header.Set("Content-Type", "application/octet-stream")
	}
	at.Header.Set("Content-Disposition", fmt.Sprintf("attachment;\r\n filename=%s", filename))
	return e.Attachments[filename], nil
}

//Bytes converts the Email object to a []byte representation, including all needed MIMEHeaders, boundaries, etc.
func (e *Email) Bytes() ([]byte, error) {
	buff := &bytes.Buffer{}
	w := multipart.NewWriter(buff)
	//Set the appropriate headers (overwriting any conflicts)
	//Leave out Bcc (only included in envelope headers)
	//TODO: Support wrapping on 76 characters (ref: MIME RFC)
	e.Headers.Set("To", strings.Join(e.To, ","))
	if e.Cc != nil {
		e.Headers.Set("Cc", strings.Join(e.Cc, ","))
	}
	e.Headers.Set("From", e.From)
	e.Headers.Set("Subject", e.Subject)
	if len(e.ReadReceipt) != 0 {
		e.Headers.Set("Disposition-Notification-To", strings.Join(e.ReadReceipt, ","))
	}
	e.Headers.Set("MIME-Version", "1.0")
	e.Headers.Set("Content-Type", fmt.Sprintf("multipart/mixed;\r\n boundary=%s\r\n", w.Boundary()))

	//Write the envelope headers (including any custom headers)
	if err := headerToBytes(buff, e.Headers); err != nil {
	}
	//Start the multipart/mixed part
	fmt.Fprintf(buff, "--%s\r\n", w.Boundary())
	header := textproto.MIMEHeader{}
	//Check to see if there is a Text or HTML field
	if e.Text != "" || e.Html != "" {
		altWriter := multipart.NewWriter(buff)
		//Create the multipart alternative part
		header.Set("Content-Type", fmt.Sprintf("multipart/alternative;\r\n boundary=%s\r\n", altWriter.Boundary()))
		//Write the header
		if err := headerToBytes(buff, header); err != nil {

		}
		//Create the body sections
		if e.Text != "" {
			header.Set("Content-Type", fmt.Sprintf("text/plain; charset=UTF-8"))
			part, err := altWriter.CreatePart(header)
			if err != nil {

			}
			// Write the text
			if err := writeMIME(part, e.Text); err != nil {

			}
		}
		if e.Html != "" {
			header.Set("Content-Type", fmt.Sprintf("text/html; charset=UTF-8"))
			altWriter.CreatePart(header)
			// Write the text
			if err := writeMIME(buff, e.Html); err != nil {

			}
		}
		altWriter.Close()
	}
	//Create attachment part, if necessary
	if e.Attachments != nil {
	}
	w.Close()
	return buff.Bytes(), nil
}

//Send an email using the given host and SMTP auth (optional), returns any error thrown by smtp.SendMail
//This function merges the To, Cc, and Bcc fields and calls the smtp.SendMail function using the Email.Bytes() output as the message
func (e *Email) Send(addr string, a smtp.Auth) error {
	//Check to make sure there is at least one recipient and one "From" address
	if e.From == "" || (len(e.To) == 0 && len(e.Cc) == 0 && len(e.Bcc) == 0) {
		return errors.New("Must specify at least one From address and one To address")
	}
	// Merge the To, Cc, and Bcc fields
	to := append(append(e.To, e.Cc...), e.Bcc...)
	from, err := mail.ParseAddress(e.From)
	if err != nil {
		return err
	}
	raw, err := e.Bytes()
	if err != nil {
		return err
	}
	return smtp.SendMail(addr, a, from.Address, to, raw)
}

//writeMIME writes the quoted-printable text to the IO Writer
func writeMIME(w io.Writer, t string) error {
	_, err := fmt.Fprintf(w, "%s\r\n", t)
	if err != nil {
		return err
	}
	return nil
}

//headerToBytes enumerates the key and values in the header, and writes the results to the IO Writer
func headerToBytes(w io.Writer, t textproto.MIMEHeader) error {
	for k, v := range t {
		//Write the header key
		_, err := fmt.Fprintf(w, "%s: ", k)
		if err != nil {
			return err
		}
		//Write each value in the header
		for _, c := range v {
			_, err := fmt.Fprintf(w, "%s\r\n", c)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

//Attachment is a struct representing an email attachment.
//Based on the mime/multipart.FileHeader struct, Attachment contains the name, MIMEHeader, and content of the attachment in question
type Attachment struct {
	Filename string
	Header   textproto.MIMEHeader
	Content  []byte
}
