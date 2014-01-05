//Package email is designed to provide an "email interface for humans."
//Designed to be robust and flexible, the email package aims to make sending email easy without getting in the way.
package email

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
)

const (
	//MaxLineLength is the maximum line length per RFC 2045
	MaxLineLength = 76
)

//Email is the type used for email messages
type Email struct {
	From        string
	To          []string
	Bcc         []string
	Cc          []string
	Subject     string
	Text        string //Plaintext message (optional)
	HTML        string //Html message (optional)
	Headers     textproto.MIMEHeader
	Attachments map[string]*Attachment
	ReadReceipt []string
}

//NewEmail creates an Email, and returns the pointer to it.
func NewEmail() *Email {
	return &Email{Attachments: make(map[string]*Attachment), Headers: textproto.MIMEHeader{}}
}

//Attach is used to attach content from an io.Reader to the email.
//Required parameters include an io.Reader, the desired filename for the attachment, and the Content-Type
//The function will return the created Attachment for reference, as well as nil for the error, if successful.
func (e *Email) Attach(r io.Reader, filename string, c string) (a *Attachment, err error) {
	buffer := new(bytes.Buffer)
	_, err = buffer.ReadFrom(r)
	if err != nil {
		return nil, err
	}
	e.Attachments[filename] = &Attachment{
		Filename: filename,
		Header:   textproto.MIMEHeader{},
		Content:  buffer.Bytes()}
	at := e.Attachments[filename]
	//Get the Content-Type to be used in the MIMEHeader
	if c != "" {
		at.Header.Set("Content-Type", c)
	} else {
		//If the Content-Type is blank, set the Content-Type to "application/octet-stream"
		at.Header.Set("Content-Type", "application/octet-stream")
	}
	at.Header.Set("Content-Disposition", fmt.Sprintf("attachment;\r\n filename=\"%s\"", filename))
	at.Header.Set("Content-Transfer-Encoding", "base64")
	return at, nil
}

//AttachFile is used to attach content to the email.
//It attempts to open the file referenced by filename and, if successful, creates an Attachment.
//This Attachment is then appended to the slice of Email.Attachments.
//The function will then return the Attachment for reference, as well as nil for the error, if successful.
func (e *Email) AttachFile(filename string) (a *Attachment, err error) {
	//Check if the file exists, return any error
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, err
	}
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	//Get the Content-Type to be used in the MIMEHeader
	ct := mime.TypeByExtension(filepath.Ext(filename))
	return e.Attach(f, filename, ct)
}

//Bytes converts the Email object to a []byte representation, including all needed MIMEHeaders, boundaries, etc.
func (e *Email) Bytes() ([]byte, error) {
	buff := &bytes.Buffer{}
	w := multipart.NewWriter(buff)
	//Set the appropriate headers (overwriting any conflicts)
	//Leave out Bcc (only included in envelope headers)
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
	if e.Text != "" || e.HTML != "" {
		subWriter := multipart.NewWriter(buff)
		//Create the multipart alternative part
		header.Set("Content-Type", fmt.Sprintf("multipart/alternative;\r\n boundary=%s\r\n", subWriter.Boundary()))
		//Write the header
		if err := headerToBytes(buff, header); err != nil {

		}
		//Create the body sections
		if e.Text != "" {
			header.Set("Content-Type", fmt.Sprintf("text/plain; charset=UTF-8"))
			header.Set("Content-Transfer-Encoding", "quoted-printable")
			if _, err := subWriter.CreatePart(header); err != nil {
				return nil, err
			}
			// Write the text
			if err := quotePrintEncode(buff, e.Text); err != nil {
				return nil, err
			}
		}
		if e.HTML != "" {
			header.Set("Content-Type", fmt.Sprintf("text/html; charset=UTF-8"))
			header.Set("Content-Transfer-Encoding", "quoted-printable")
			if _, err := subWriter.CreatePart(header); err != nil {
				return nil, err
			}
			// Write the text
			if err := quotePrintEncode(buff, e.HTML); err != nil {
				return nil, err
			}
		}
		if err := subWriter.Close(); err != nil {
			return nil, err
		}
	}
	//Create attachment part, if necessary
	if e.Attachments != nil {
		for _, a := range e.Attachments {
			ap, err := w.CreatePart(a.Header)
			if err != nil {
				return nil, err
			}
			//Write the base64Wrapped content to the part
			base64Wrap(ap, a.Content)
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
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

//Attachment is a struct representing an email attachment.
//Based on the mime/multipart.FileHeader struct, Attachment contains the name, MIMEHeader, and content of the attachment in question
type Attachment struct {
	Filename string
	Header   textproto.MIMEHeader
	Content  []byte
}

//quotePrintEncode writes the quoted-printable text to the IO Writer (according to RFC 2045)
func quotePrintEncode(w io.Writer, s string) error {
	mc := 0
	for _, c := range s {
		// Handle the soft break for the EOL, if needed
		if mc == MaxLineLength-1 || (!isPrintable(c) && mc+len(fmt.Sprintf("%s%X", "=", c)) > MaxLineLength-1) {
			if _, err := fmt.Fprintf(w, "%s", "=\r\n"); err != nil {
				return err
			}
			mc = 0
		}
		//append the appropriate character
		if isPrintable(c) {
			//Printable character
			if _, err := fmt.Fprintf(w, "%s", string(c)); err != nil {
				return err
			}
			// Reset the counter if we wrote a newline
			if c == '\n' {
				mc = 0
			}
			mc++
			continue
		} else {
			//non-printable.. encode it (TODO)
			es := fmt.Sprintf("%s%X", "=", c)
			if _, err := fmt.Fprintf(w, "%s", es); err != nil {
				return err
			}
			//todo - increment correctly
			mc += len(es)
		}
	}
	return nil
}

//isPrintable returns true if the rune given is "printable" according to RFC 2045, false otherwise
func isPrintable(c rune) bool {
	return (c >= '!' && c <= '<') || (c >= '>' && c <= '~') || (c == ' ' || c == '\n' || c == '\t')
}

//base64Wrap encodeds the attachment content, and wraps it according to RFC 2045 standards (every 76 chars)
//The output is then written to the specified io.Writer
func base64Wrap(w io.Writer, b []byte) {
	encoded := base64.StdEncoding.EncodeToString(b)
	for i := 0; i < len(encoded); i += MaxLineLength {
		//Do we need to print 76 characters, or the rest of the string?
		if len(encoded)-i < MaxLineLength {
			fmt.Fprintf(w, "%s\r\n", encoded[i:])
		} else {
			fmt.Fprintf(w, "%s\r\n", encoded[i:i+MaxLineLength])
		}
	}
}

//headerToBytes enumerates the key and values in the header, and writes the results to the IO Writer
func headerToBytes(w io.Writer, t textproto.MIMEHeader) error {
	for k, v := range t {
		//Write the header key
		_, err := fmt.Fprintf(w, "%s:", k)
		if err != nil {
			return err
		}
		//Write each value in the header
		for _, c := range v {
			_, err := fmt.Fprintf(w, " %s\r\n", c)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
