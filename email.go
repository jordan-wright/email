// Package email is designed to provide an "email interface for humans."
// Designed to be robust and flexible, the email package aims to make sending email easy without getting in the way.
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
	"path"
	"path/filepath"
	"strings"
)

const (
	// MaxLineLength is the maximum line length per RFC 2045
	MaxLineLength = 76
)

// Email is the type used for email messages
type Email struct {
	From        string
	To          []string
	Bcc         []string
	Cc          []string
	Subject     string
	Text        string // Plaintext message (optional)
	HTML        string // Html message (optional)
	Headers     textproto.MIMEHeader
	Attachments []*Attachment
	ReadReceipt []string
}

// NewEmail creates an Email, and returns the pointer to it.
func NewEmail() *Email {
	return &Email{Headers: textproto.MIMEHeader{}}
}

// Attach is used to attach content from an io.Reader to the email.
// Required parameters include an io.Reader, the desired filename for the attachment, and the Content-Type
// The function will return the created Attachment for reference, as well as nil for the error, if successful.
func (e *Email) Attach(r io.Reader, filename string, c string) (a *Attachment, err error) {
	var buffer bytes.Buffer
	if _, err = io.Copy(&buffer, r); err != nil {
		return
	}
	at := &Attachment{
		Filename: filename,
		Header:   textproto.MIMEHeader{},
		Content:  buffer.Bytes(),
	}
	// Get the Content-Type to be used in the MIMEHeader
	if c != "" {
		at.Header.Set("Content-Type", c)
	} else {
		// If the Content-Type is blank, set the Content-Type to "application/octet-stream"
		at.Header.Set("Content-Type", "application/octet-stream")
	}
	at.Header.Set("Content-Disposition", fmt.Sprintf("attachment;\r\n filename=\"%s\"", filename))
	at.Header.Set("Content-Transfer-Encoding", "base64")
	e.Attachments = append(e.Attachments, at)
	return at, nil
}

// AttachFile is used to attach content to the email.
// It attempts to open the file referenced by filename and, if successful, creates an Attachment.
// This Attachment is then appended to the slice of Email.Attachments.
// The function will then return the Attachment for reference, as well as nil for the error, if successful.
func (e *Email) AttachFile(filename string) (a *Attachment, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	ct := mime.TypeByExtension(filepath.Ext(filename))
	basename := path.Base(filename)
	return e.Attach(f, basename, ct)
}

// Bytes converts the Email object to a []byte representation, including all needed MIMEHeaders, boundaries, etc.
func (e *Email) Bytes() ([]byte, error) {
	buff := &bytes.Buffer{}
	w := multipart.NewWriter(buff)
	// Set the appropriate headers (overwriting any conflicts)
	// Leave out Bcc (only included in envelope headers)
	if e.Headers == nil {
		e.Headers = textproto.MIMEHeader{}
	}

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

	// Write the envelope headers (including any custom headers)
	if err := headerToBytes(buff, e.Headers); err != nil {
		return nil, fmt.Errorf("Failed to render message headers: %s", err)
	}
	// Start the multipart/mixed part
	fmt.Fprintf(buff, "--%s\r\n", w.Boundary())
	header := textproto.MIMEHeader{}
	// Check to see if there is a Text or HTML field
	if e.Text != "" || e.HTML != "" {
		subWriter := multipart.NewWriter(buff)
		// Create the multipart alternative part
		header.Set("Content-Type", fmt.Sprintf("multipart/alternative;\r\n boundary=%s\r\n", subWriter.Boundary()))
		// Write the header
		if err := headerToBytes(buff, header); err != nil {
			return nil, fmt.Errorf("Failed to render multipart message headers: %s", err)
		}
		// Create the body sections
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
	// Create attachment part, if necessary
	for _, a := range e.Attachments {
		ap, err := w.CreatePart(a.Header)
		if err != nil {
			return nil, err
		}
		// Write the base64Wrapped content to the part
		base64Wrap(ap, a.Content)
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

// Send an email using the given host and SMTP auth (optional), returns any error thrown by smtp.SendMail
// This function merges the To, Cc, and Bcc fields and calls the smtp.SendMail function using the Email.Bytes() output as the message
func (e *Email) Send(addr string, a smtp.Auth) error {
	// Merge the To, Cc, and Bcc fields
	to := make([]string, 0, len(e.To)+len(e.Cc)+len(e.Bcc))
	to = append(append(append(to, e.To...), e.Cc...), e.Bcc...)
	// Check to make sure there is at least one recipient and one "From" address
	if e.From == "" || len(to) == 0 {
		return errors.New("Must specify at least one From address and one To address")
	}
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

// Attachment is a struct representing an email attachment.
// Based on the mime/multipart.FileHeader struct, Attachment contains the name, MIMEHeader, and content of the attachment in question
type Attachment struct {
	Filename string
	Header   textproto.MIMEHeader
	Content  []byte
}

// quotePrintEncode writes the quoted-printable text to the IO Writer (according to RFC 2045)
func quotePrintEncode(w io.Writer, s string) error {
	var buf [3]byte
	mc := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		// We're assuming Unix style text formats as input (LF line break), and
		// quoted-printble uses CRLF line breaks. (Literal CRs will become
		// "=0D", but probably shouldn't be there to begin with!)
		if c == '\n' {
			io.WriteString(w, "\r\n")
			mc = 0
			continue
		}

		var nextOut []byte
		if isPrintable(c) {
			nextOut = append(buf[:0], c)
		} else {
			nextOut = buf[:]
			qpEscape(nextOut, c)
		}

		// Add a soft line break if the next (encoded) byte would push this line
		// to or past the limit.
		if mc+len(nextOut) >= MaxLineLength {
			if _, err := io.WriteString(w, "=\r\n"); err != nil {
				return err
			}
			mc = 0
		}

		if _, err := w.Write(nextOut); err != nil {
			return err
		}
		mc += len(nextOut)
	}
	// No trailing end-of-line?? Soft line break, then. TODO: is this sane?
	if mc > 0 {
		io.WriteString(w, "=\r\n")
	}
	return nil
}

// isPrintable returns true if the rune given is "printable" according to RFC 2045, false otherwise
func isPrintable(c byte) bool {
	return (c >= '!' && c <= '<') || (c >= '>' && c <= '~') || (c == ' ' || c == '\n' || c == '\t')
}

// qpEscape is a helper function for quotePrintEncode which escapes a
// non-printable byte. Expects len(dest) == 3.
func qpEscape(dest []byte, c byte) {
	const nums = "0123456789ABCDEF"
	dest[0] = '='
	dest[1] = nums[(c&0xf0)>>4]
	dest[2] = nums[(c & 0xf)]
}

// base64Wrap encodeds the attachment content, and wraps it according to RFC 2045 standards (every 76 chars)
// The output is then written to the specified io.Writer
func base64Wrap(w io.Writer, b []byte) {
	// 57 raw bytes per 76-byte base64 line.
	const maxRaw = 57
	// Buffer for each line, including trailing CRLF.
	var buffer [MaxLineLength + len("\r\n")]byte
	copy(buffer[MaxLineLength:], "\r\n")
	// Process raw chunks until there's no longer enough to fill a line.
	for len(b) >= maxRaw {
		base64.StdEncoding.Encode(buffer[:], b[:maxRaw])
		w.Write(buffer[:])
		b = b[maxRaw:]
	}
	// Handle the last chunk of bytes.
	if len(b) > 0 {
		out := buffer[:base64.StdEncoding.EncodedLen(len(b))]
		base64.StdEncoding.Encode(out, b)
		out = append(out, "\r\n"...)
		w.Write(out)
	}
}

// headerToBytes enumerates the key and values in the header, and writes the results to the IO Writer
func headerToBytes(w io.Writer, t textproto.MIMEHeader) error {
	for k, v := range t {
		// Write the header key
		_, err := fmt.Fprintf(w, "%s:", k)
		if err != nil {
			return err
		}
		// Write each value in the header
		for _, c := range v {
			_, err := fmt.Fprintf(w, " %s\r\n", c)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
