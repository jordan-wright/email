package email

import (
	"fmt"
	"strings"
	"testing"

	"bufio"
	"bytes"
	"crypto/rand"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/smtp"
	"net/textproto"
)

func prepareEmail() *Email {
	e := NewEmail()
	e.From = "Jordan Wright <test@example.com>"
	e.To = []string{"test@example.com"}
	e.Bcc = []string{"test_bcc@example.com"}
	e.Cc = []string{"test_cc@example.com"}
	e.Subject = "Awesome Subject"
	return e
}

func basicTests(t *testing.T, e *Email) *mail.Message {
	raw, err := e.Bytes()
	if err != nil {
		t.Fatal("Failed to render message: ", e)
	}

	msg, err := mail.ReadMessage(bytes.NewBuffer(raw))
	if err != nil {
		t.Fatal("Could not parse rendered message: ", err)
	}

	expectedHeaders := map[string]string{
		"To":      "<test@example.com>",
		"From":    "\"Jordan Wright\" <test@example.com>",
		"Cc":      "<test_cc@example.com>",
		"Subject": "Awesome Subject",
	}

	for header, expected := range expectedHeaders {
		if val := msg.Header.Get(header); val != expected {
			t.Errorf("Wrong value for message header %s: %v != %v", header, expected, val)
		}
	}
	return msg
}

func TestEmailText(t *testing.T) {
	e := prepareEmail()
	e.Text = []byte("Text Body is, of course, supported!\n")

	msg := basicTests(t, e)

	// Were the right headers set?
	ct := msg.Header.Get("Content-type")
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatal("Content-type header is invalid: ", ct)
	} else if mt != "text/plain" {
		t.Fatalf("Content-type expected \"text/plain\", not %v", mt)
	}
}

func TestEmailWithHTMLAttachments(t *testing.T) {
	e := prepareEmail()

	// Set plain text to exercise "mime/alternative"
	e.Text = []byte("Text Body is, of course, supported!\n")

	e.HTML = []byte("<html><body>This is a text.</body></html>")

	// Set HTML attachment to exercise "mime/related".
	attachment, err := e.Attach(bytes.NewBufferString("Rad attachment"), "rad.txt", "image/png; charset=utf-8")
	if err != nil {
		t.Fatal("Could not add an attachment to the message: ", err)
	}
	attachment.HTMLRelated = true

	b, err := e.Bytes()
	if err != nil {
		t.Fatal("Could not serialize e-mail:", err)
	}

	// Print the bytes for ocular validation and make sure no errors.
	//fmt.Println(string(b))

	// TODO: Verify the attachments.
	s := &trimReader{rd: bytes.NewBuffer(b)}
	tp := textproto.NewReader(bufio.NewReader(s))
	// Parse the main headers
	hdrs, err := tp.ReadMIMEHeader()
	if err != nil {
		t.Fatal("Could not parse the headers:", err)
	}
	// Recursively parse the MIME parts
	ps, err := parseMIMEParts(hdrs, tp.R)
	if err != nil {
		t.Fatal("Could not parse the MIME parts recursively:", err)
	}

	plainTextFound := false
	htmlFound := false
	imageFound := false
	if expected, actual := 3, len(ps); actual != expected {
		t.Error("Unexpected number of parts. Expected:", expected, "Was:", actual)
	}
	for _, part := range ps {
		// part has "header" and "body []byte"
		cd := part.header.Get("Content-Disposition")
		ct := part.header.Get("Content-Type")
		if strings.Contains(ct, "image/png") && strings.HasPrefix(cd, "inline") {
			imageFound = true
		}
		if strings.Contains(ct, "text/html") {
			htmlFound = true
		}
		if strings.Contains(ct, "text/plain") {
			plainTextFound = true
		}
	}

	if !plainTextFound {
		t.Error("Did not find plain text part.")
	}
	if !htmlFound {
		t.Error("Did not find HTML part.")
	}
	if !imageFound {
		t.Error("Did not find image part.")
	}
}

func TestEmailWithHTMLAttachmentsHTMLOnly(t *testing.T) {
	e := prepareEmail()

	e.HTML = []byte("<html><body>This is a text.</body></html>")

	// Set HTML attachment to exercise "mime/related".
	attachment, err := e.Attach(bytes.NewBufferString("Rad attachment"), "rad.txt", "image/png; charset=utf-8")
	if err != nil {
		t.Fatal("Could not add an attachment to the message: ", err)
	}
	attachment.HTMLRelated = true

	b, err := e.Bytes()
	if err != nil {
		t.Fatal("Could not serialize e-mail:", err)
	}

	// Print the bytes for ocular validation and make sure no errors.
	//fmt.Println(string(b))

	// TODO: Verify the attachments.
	s := &trimReader{rd: bytes.NewBuffer(b)}
	tp := textproto.NewReader(bufio.NewReader(s))
	// Parse the main headers
	hdrs, err := tp.ReadMIMEHeader()
	if err != nil {
		t.Fatal("Could not parse the headers:", err)
	}

	if !strings.HasPrefix(hdrs.Get("Content-Type"), "multipart/related") {
		t.Error("Envelope Content-Type is not multipart/related: ", hdrs["Content-Type"])
	}

	// Recursively parse the MIME parts
	ps, err := parseMIMEParts(hdrs, tp.R)
	if err != nil {
		t.Fatal("Could not parse the MIME parts recursively:", err)
	}

	htmlFound := false
	imageFound := false
	if expected, actual := 2, len(ps); actual != expected {
		t.Error("Unexpected number of parts. Expected:", expected, "Was:", actual)
	}
	for _, part := range ps {
		// part has "header" and "body []byte"
		ct := part.header.Get("Content-Type")
		if strings.Contains(ct, "image/png") {
			imageFound = true
		}
		if strings.Contains(ct, "text/html") {
			htmlFound = true
		}
	}

	if !htmlFound {
		t.Error("Did not find HTML part.")
	}
	if !imageFound {
		t.Error("Did not find image part.")
	}
}

func TestEmailHTML(t *testing.T) {
	e := prepareEmail()
	e.HTML = []byte("<h1>Fancy Html is supported, too!</h1>\n")

	msg := basicTests(t, e)

	// Were the right headers set?
	ct := msg.Header.Get("Content-type")
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatalf("Content-type header is invalid: %#v", ct)
	} else if mt != "text/html" {
		t.Fatalf("Content-type expected \"text/html\", not %v", mt)
	}
}

func TestEmailTextAttachment(t *testing.T) {
	e := prepareEmail()
	e.Text = []byte("Text Body is, of course, supported!\n")
	_, err := e.Attach(bytes.NewBufferString("Rad attachment"), "rad.txt", "text/plain; charset=utf-8")
	if err != nil {
		t.Fatal("Could not add an attachment to the message: ", err)
	}

	msg := basicTests(t, e)

	// Were the right headers set?
	ct := msg.Header.Get("Content-type")
	mt, params, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatal("Content-type header is invalid: ", ct)
	} else if mt != "multipart/mixed" {
		t.Fatalf("Content-type expected \"multipart/mixed\", not %v", mt)
	}
	b := params["boundary"]
	if b == "" {
		t.Fatalf("Invalid or missing boundary parameter: %#v", b)
	}
	if len(params) != 1 {
		t.Fatal("Unexpected content-type parameters")
	}

	// Is the generated message parsable?
	mixed := multipart.NewReader(msg.Body, params["boundary"])

	text, err := mixed.NextPart()
	if err != nil {
		t.Fatalf("Could not find text component of email: %s", err)
	}

	// Does the text portion match what we expect?
	mt, _, err = mime.ParseMediaType(text.Header.Get("Content-type"))
	if err != nil {
		t.Fatal("Could not parse message's Content-Type")
	} else if mt != "text/plain" {
		t.Fatal("Message missing text/plain")
	}
	plainText, err := ioutil.ReadAll(text)
	if err != nil {
		t.Fatal("Could not read plain text component of message: ", err)
	}
	if !bytes.Equal(plainText, []byte("Text Body is, of course, supported!\r\n")) {
		t.Fatalf("Plain text is broken: %#q", plainText)
	}

	// Check attachments.
	_, err = mixed.NextPart()
	if err != nil {
		t.Fatalf("Could not find attachment component of email: %s", err)
	}

	if _, err = mixed.NextPart(); err != io.EOF {
		t.Error("Expected only text and one attachment!")
	}
}

func TestEmailTextHtmlAttachment(t *testing.T) {
	e := prepareEmail()
	e.Text = []byte("Text Body is, of course, supported!\n")
	e.HTML = []byte("<h1>Fancy Html is supported, too!</h1>\n")
	_, err := e.Attach(bytes.NewBufferString("Rad attachment"), "rad.txt", "text/plain; charset=utf-8")
	if err != nil {
		t.Fatal("Could not add an attachment to the message: ", err)
	}

	msg := basicTests(t, e)

	// Were the right headers set?
	ct := msg.Header.Get("Content-type")
	mt, params, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatal("Content-type header is invalid: ", ct)
	} else if mt != "multipart/mixed" {
		t.Fatalf("Content-type expected \"multipart/mixed\", not %v", mt)
	}
	b := params["boundary"]
	if b == "" {
		t.Fatal("Unexpected empty boundary parameter")
	}
	if len(params) != 1 {
		t.Fatal("Unexpected content-type parameters")
	}

	// Is the generated message parsable?
	mixed := multipart.NewReader(msg.Body, params["boundary"])

	text, err := mixed.NextPart()
	if err != nil {
		t.Fatalf("Could not find text component of email: %s", err)
	}

	// Does the text portion match what we expect?
	mt, params, err = mime.ParseMediaType(text.Header.Get("Content-type"))
	if err != nil {
		t.Fatal("Could not parse message's Content-Type")
	} else if mt != "multipart/alternative" {
		t.Fatal("Message missing multipart/alternative")
	}
	mpReader := multipart.NewReader(text, params["boundary"])
	part, err := mpReader.NextPart()
	if err != nil {
		t.Fatal("Could not read plain text component of message: ", err)
	}
	plainText, err := ioutil.ReadAll(part)
	if err != nil {
		t.Fatal("Could not read plain text component of message: ", err)
	}
	if !bytes.Equal(plainText, []byte("Text Body is, of course, supported!\r\n")) {
		t.Fatalf("Plain text is broken: %#q", plainText)
	}

	// Check attachments.
	_, err = mixed.NextPart()
	if err != nil {
		t.Fatalf("Could not find attachment component of email: %s", err)
	}

	if _, err = mixed.NextPart(); err != io.EOF {
		t.Error("Expected only text and one attachment!")
	}
}

func TestEmailAttachment(t *testing.T) {
	e := prepareEmail()
	_, err := e.Attach(bytes.NewBufferString("Rad attachment"), "rad.txt", "text/plain; charset=utf-8")
	if err != nil {
		t.Fatal("Could not add an attachment to the message: ", err)
	}
	msg := basicTests(t, e)

	// Were the right headers set?
	ct := msg.Header.Get("Content-type")
	mt, params, err := mime.ParseMediaType(ct)
	if err != nil {
		t.Fatal("Content-type header is invalid: ", ct)
	} else if mt != "multipart/mixed" {
		t.Fatalf("Content-type expected \"multipart/mixed\", not %v", mt)
	}
	b := params["boundary"]
	if b == "" {
		t.Fatal("Unexpected empty boundary parameter")
	}
	if len(params) != 1 {
		t.Fatal("Unexpected content-type parameters")
	}

	// Is the generated message parsable?
	mixed := multipart.NewReader(msg.Body, params["boundary"])

	// Check attachments.
	a, err := mixed.NextPart()
	if err != nil {
		t.Fatalf("Could not find attachment component of email: %s", err)
	}

	if !strings.HasPrefix(a.Header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("Content disposition is not attachment: %s", a.Header.Get("Content-Disposition"))
	}

	if _, err = mixed.NextPart(); err != io.EOF {
		t.Error("Expected only one attachment!")
	}
}

func TestHeaderEncoding(t *testing.T) {
	cases := []struct {
		field string
		have  string
		want  string
	}{
		{
			field: "From",
			have:  "Needs Encóding <encoding@example.com>, Only ASCII <foo@example.com>",
			want:  "=?utf-8?q?Needs_Enc=C3=B3ding?= <encoding@example.com>, \"Only ASCII\" <foo@example.com>\r\n",
		},
		{
			field: "To",
			have:  "Keith Moore <moore@cs.utk.edu>, Keld Jørn Simonsen <keld@dkuug.dk>",
			want:  "\"Keith Moore\" <moore@cs.utk.edu>, =?utf-8?q?Keld_J=C3=B8rn_Simonsen?= <keld@dkuug.dk>\r\n",
		},
		{
			field: "Cc",
			have:  "Needs Encóding <encoding@example.com>, \"Test :)\" <test@localhost>",
			want:  "=?utf-8?q?Needs_Enc=C3=B3ding?= <encoding@example.com>, \"Test :)\" <test@localhost>\r\n",
		},
		{
			field: "Subject",
			have:  "Subject with a 🐟",
			want:  "=?UTF-8?q?Subject_with_a_=F0=9F=90=9F?=\r\n",
		},
		{
			field: "Subject",
			have:  "Subject with only ASCII",
			want:  "Subject with only ASCII\r\n",
		},
	}
	buff := &bytes.Buffer{}
	for _, c := range cases {
		header := make(textproto.MIMEHeader)
		header.Add(c.field, c.have)
		buff.Reset()
		headerToBytes(buff, header)
		want := fmt.Sprintf("%s: %s", c.field, c.want)
		got := buff.String()
		if got != want {
			t.Errorf("invalid utf-8 header encoding. \nwant:%#v\ngot: %#v", want, got)
		}
	}
}

func TestEmailFromReader(t *testing.T) {
	ex := &Email{
		Subject: "Test Subject",
		To:      []string{"Jordan Wright <jmwright798@gmail.com>", "also@example.com"},
		From:    "Jordan Wright <jmwright798@gmail.com>",
		ReplyTo: []string{"Jordan Wright <jmwright798@gmail.com>"},
		Cc:      []string{"one@example.com", "Two <two@example.com>"},
		Bcc:     []string{"three@example.com", "Four <four@example.com>"},
		Text:    []byte("This is a test email with HTML Formatting. It also has very long lines so\nthat the content must be wrapped if using quoted-printable decoding.\n"),
		HTML:    []byte("<div dir=\"ltr\">This is a test email with <b>HTML Formatting.</b>\u00a0It also has very long lines so that the content must be wrapped if using quoted-printable decoding.</div>\n"),
	}
	raw := []byte(`
	MIME-Version: 1.0
Subject: Test Subject
From: Jordan Wright <jmwright798@gmail.com>
Reply-To: Jordan Wright <jmwright798@gmail.com>
To: Jordan Wright <jmwright798@gmail.com>, also@example.com
Cc: one@example.com, Two <two@example.com>
Bcc: three@example.com, Four <four@example.com>
Content-Type: multipart/alternative; boundary=001a114fb3fc42fd6b051f834280

--001a114fb3fc42fd6b051f834280
Content-Type: text/plain; charset=UTF-8

This is a test email with HTML Formatting. It also has very long lines so
that the content must be wrapped if using quoted-printable decoding.

--001a114fb3fc42fd6b051f834280
Content-Type: text/html; charset=UTF-8
Content-Transfer-Encoding: quoted-printable

<div dir=3D"ltr">This is a test email with <b>HTML Formatting.</b>=C2=A0It =
also has very long lines so that the content must be wrapped if using quote=
d-printable decoding.</div>

--001a114fb3fc42fd6b051f834280--`)
	e, err := NewEmailFromReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Error creating email %s", err.Error())
	}
	if e.Subject != ex.Subject {
		t.Fatalf("Incorrect subject. %#q != %#q", e.Subject, ex.Subject)
	}
	if !bytes.Equal(e.Text, ex.Text) {
		t.Fatalf("Incorrect text: %#q != %#q", e.Text, ex.Text)
	}
	if !bytes.Equal(e.HTML, ex.HTML) {
		t.Fatalf("Incorrect HTML: %#q != %#q", e.HTML, ex.HTML)
	}
	if e.From != ex.From {
		t.Fatalf("Incorrect \"From\": %#q != %#q", e.From, ex.From)
	}
	if len(e.To) != len(ex.To) {
		t.Fatalf("Incorrect number of \"To\" addresses: %v != %v", len(e.To), len(ex.To))
	}
	if e.To[0] != ex.To[0] {
		t.Fatalf("Incorrect \"To[0]\": %#q != %#q", e.To[0], ex.To[0])
	}
	if e.To[1] != ex.To[1] {
		t.Fatalf("Incorrect \"To[1]\": %#q != %#q", e.To[1], ex.To[1])
	}
	if len(e.Cc) != len(ex.Cc) {
		t.Fatalf("Incorrect number of \"Cc\" addresses: %v != %v", len(e.Cc), len(ex.Cc))
	}
	if e.Cc[0] != ex.Cc[0] {
		t.Fatalf("Incorrect \"Cc[0]\": %#q != %#q", e.Cc[0], ex.Cc[0])
	}
	if e.Cc[1] != ex.Cc[1] {
		t.Fatalf("Incorrect \"Cc[1]\": %#q != %#q", e.Cc[1], ex.Cc[1])
	}
	if len(e.Bcc) != len(ex.Bcc) {
		t.Fatalf("Incorrect number of \"Bcc\" addresses: %v != %v", len(e.Bcc), len(ex.Bcc))
	}
	if e.Bcc[0] != ex.Bcc[0] {
		t.Fatalf("Incorrect \"Bcc[0]\": %#q != %#q", e.Cc[0], ex.Cc[0])
	}
	if e.Bcc[1] != ex.Bcc[1] {
		t.Fatalf("Incorrect \"Bcc[1]\": %#q != %#q", e.Bcc[1], ex.Bcc[1])
	}
	if len(e.ReplyTo) != len(ex.ReplyTo) {
		t.Fatalf("Incorrect number of \"Reply-To\" addresses: %v != %v", len(e.ReplyTo), len(ex.ReplyTo))
	}
	if e.ReplyTo[0] != ex.ReplyTo[0] {
		t.Fatalf("Incorrect \"ReplyTo\": %#q != %#q", e.ReplyTo[0], ex.ReplyTo[0])
	}
}

func TestNonAsciiEmailFromReader(t *testing.T) {
	ex := &Email{
		Subject: "Test Subject",
		To:      []string{"Anaïs <anais@example.org>"},
		Cc:      []string{"Patrik Fältström <paf@example.com>"},
		From:    "Mrs ValÃ©rie Dupont <valerie.dupont@example.com>",
		Text:    []byte("This is a test message!"),
	}
	raw := []byte(`
	MIME-Version: 1.0
Subject: =?UTF-8?Q?Test Subject?=
From: Mrs =?ISO-8859-1?Q?Val=C3=A9rie=20Dupont?= <valerie.dupont@example.com>
To: =?utf-8?q?Ana=C3=AFs?= <anais@example.org>
Cc: =?ISO-8859-1?Q?Patrik_F=E4ltstr=F6m?= <paf@example.com>
Content-type: text/plain; charset=ISO-8859-1

This is a test message!`)
	e, err := NewEmailFromReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Error creating email %s", err.Error())
	}
	if e.Subject != ex.Subject {
		t.Fatalf("Incorrect subject. %#q != %#q", e.Subject, ex.Subject)
	}
	if e.From != ex.From {
		t.Fatalf("Incorrect \"From\": %#q != %#q", e.From, ex.From)
	}
	if e.To[0] != ex.To[0] {
		t.Fatalf("Incorrect \"To\": %#q != %#q", e.To, ex.To)
	}
	if e.Cc[0] != ex.Cc[0] {
		t.Fatalf("Incorrect \"Cc\": %#q != %#q", e.Cc, ex.Cc)
	}
}

func TestNonMultipartEmailFromReader(t *testing.T) {
	ex := &Email{
		Text:    []byte("This is a test message!"),
		Subject: "Example Subject (no MIME Type)",
		Headers: textproto.MIMEHeader{},
	}
	ex.Headers.Add("Content-Type", "text/plain; charset=us-ascii")
	ex.Headers.Add("Message-ID", "<foobar@example.com>")
	raw := []byte(`From: "Foo Bar" <foobar@example.com>
Content-Type: text/plain
To: foobar@example.com
Subject: Example Subject (no MIME Type)
Message-ID: <foobar@example.com>

This is a test message!`)
	e, err := NewEmailFromReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Error creating email %s", err.Error())
	}
	if ex.Subject != e.Subject {
		t.Errorf("Incorrect subject. %#q != %#q\n", ex.Subject, e.Subject)
	}
	if !bytes.Equal(ex.Text, e.Text) {
		t.Errorf("Incorrect body. %#q != %#q\n", ex.Text, e.Text)
	}
	if ex.Headers.Get("Message-ID") != e.Headers.Get("Message-ID") {
		t.Errorf("Incorrect message ID. %#q != %#q\n", ex.Headers.Get("Message-ID"), e.Headers.Get("Message-ID"))
	}
}

func TestBase64EmailFromReader(t *testing.T) {
	ex := &Email{
		Subject: "Test Subject",
		To:      []string{"Jordan Wright <jmwright798@gmail.com>"},
		From:    "Jordan Wright <jmwright798@gmail.com>",
		Text:    []byte("This is a test email with HTML Formatting. It also has very long lines so that the content must be wrapped if using quoted-printable decoding."),
		HTML:    []byte("<div dir=\"ltr\">This is a test email with <b>HTML Formatting.</b>\u00a0It also has very long lines so that the content must be wrapped if using quoted-printable decoding.</div>\n"),
	}
	raw := []byte(`
		MIME-Version: 1.0
Subject: Test Subject
From: Jordan Wright <jmwright798@gmail.com>
To: Jordan Wright <jmwright798@gmail.com>
Content-Type: multipart/alternative; boundary=001a114fb3fc42fd6b051f834280

--001a114fb3fc42fd6b051f834280
Content-Type: text/plain; charset=UTF-8
Content-Transfer-Encoding: base64

VGhpcyBpcyBhIHRlc3QgZW1haWwgd2l0aCBIVE1MIEZvcm1hdHRpbmcuIEl0IGFsc28gaGFzIHZl
cnkgbG9uZyBsaW5lcyBzbyB0aGF0IHRoZSBjb250ZW50IG11c3QgYmUgd3JhcHBlZCBpZiB1c2lu
ZyBxdW90ZWQtcHJpbnRhYmxlIGRlY29kaW5nLg==

--001a114fb3fc42fd6b051f834280
Content-Type: text/html; charset=UTF-8
Content-Transfer-Encoding: quoted-printable

<div dir=3D"ltr">This is a test email with <b>HTML Formatting.</b>=C2=A0It =
also has very long lines so that the content must be wrapped if using quote=
d-printable decoding.</div>

--001a114fb3fc42fd6b051f834280--`)
	e, err := NewEmailFromReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Error creating email %s", err.Error())
	}
	if e.Subject != ex.Subject {
		t.Fatalf("Incorrect subject. %#q != %#q", e.Subject, ex.Subject)
	}
	if !bytes.Equal(e.Text, ex.Text) {
		t.Fatalf("Incorrect text: %#q != %#q", e.Text, ex.Text)
	}
	if !bytes.Equal(e.HTML, ex.HTML) {
		t.Fatalf("Incorrect HTML: %#q != %#q", e.HTML, ex.HTML)
	}
	if e.From != ex.From {
		t.Fatalf("Incorrect \"From\": %#q != %#q", e.From, ex.From)
	}
}

func TestAttachmentEmailFromReader(t *testing.T) {
	ex := &Email{
		Subject: "Test Subject",
		To:      []string{"Jordan Wright <jmwright798@gmail.com>"},
		From:    "Jordan Wright <jmwright798@gmail.com>",
		Text:    []byte("Simple text body"),
		HTML:    []byte("<div dir=\"ltr\">Simple HTML body</div>\n"),
	}
	a, err := ex.Attach(bytes.NewReader([]byte("Let's just pretend this is raw JPEG data.")), "cat.jpeg", "image/jpeg")
	if err != nil {
		t.Fatalf("Error attaching image %s", err.Error())
	}
	b, err := ex.Attach(bytes.NewReader([]byte("Let's just pretend this is raw JPEG data.")), "cat-inline.jpeg", "image/jpeg")
	if err != nil {
		t.Fatalf("Error attaching inline image %s", err.Error())
	}
	raw := []byte(`
From: Jordan Wright <jmwright798@gmail.com>
Date: Thu, 17 Oct 2019 08:55:37 +0100
Mime-Version: 1.0
Content-Type: multipart/mixed;
 boundary=35d10c2224bd787fe700c2c6f4769ddc936eb8a0b58e9c8717e406c5abb7
To: Jordan Wright <jmwright798@gmail.com>
Subject: Test Subject

--35d10c2224bd787fe700c2c6f4769ddc936eb8a0b58e9c8717e406c5abb7
Content-Type: multipart/alternative;
 boundary=b10ca5b1072908cceb667e8968d3af04503b7ab07d61c9f579c15b416d7c

--b10ca5b1072908cceb667e8968d3af04503b7ab07d61c9f579c15b416d7c
Content-Transfer-Encoding: quoted-printable
Content-Type: text/plain; charset=UTF-8

Simple text body
--b10ca5b1072908cceb667e8968d3af04503b7ab07d61c9f579c15b416d7c
Content-Transfer-Encoding: quoted-printable
Content-Type: text/html; charset=UTF-8

<div dir=3D"ltr">Simple HTML body</div>

--b10ca5b1072908cceb667e8968d3af04503b7ab07d61c9f579c15b416d7c--

--35d10c2224bd787fe700c2c6f4769ddc936eb8a0b58e9c8717e406c5abb7
Content-Disposition: attachment;
 filename="cat.jpeg"
Content-Id: <cat.jpeg>
Content-Transfer-Encoding: base64
Content-Type: image/jpeg

TGV0J3MganVzdCBwcmV0ZW5kIHRoaXMgaXMgcmF3IEpQRUcgZGF0YS4=

--35d10c2224bd787fe700c2c6f4769ddc936eb8a0b58e9c8717e406c5abb7
Content-Disposition: inline;
 filename="cat-inline.jpeg"
Content-Id: <cat-inline.jpeg>
Content-Transfer-Encoding: base64
Content-Type: image/jpeg

TGV0J3MganVzdCBwcmV0ZW5kIHRoaXMgaXMgcmF3IEpQRUcgZGF0YS4=

--35d10c2224bd787fe700c2c6f4769ddc936eb8a0b58e9c8717e406c5abb7--`)
	e, err := NewEmailFromReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Error creating email %s", err.Error())
	}
	if e.Subject != ex.Subject {
		t.Fatalf("Incorrect subject. %#q != %#q", e.Subject, ex.Subject)
	}
	if !bytes.Equal(e.Text, ex.Text) {
		t.Fatalf("Incorrect text: %#q != %#q", e.Text, ex.Text)
	}
	if !bytes.Equal(e.HTML, ex.HTML) {
		t.Fatalf("Incorrect HTML: %#q != %#q", e.HTML, ex.HTML)
	}
	if e.From != ex.From {
		t.Fatalf("Incorrect \"From\": %#q != %#q", e.From, ex.From)
	}
	if len(e.Attachments) != 2 {
		t.Fatalf("Incorrect number of attachments %d != %d", len(e.Attachments), 1)
	}
	if e.Attachments[0].Filename != a.Filename {
		t.Fatalf("Incorrect attachment filename %s != %s", e.Attachments[0].Filename, a.Filename)
	}
	if !bytes.Equal(e.Attachments[0].Content, a.Content) {
		t.Fatalf("Incorrect attachment content %#q != %#q", e.Attachments[0].Content, a.Content)
	}
	if e.Attachments[1].Filename != b.Filename {
		t.Fatalf("Incorrect attachment filename %s != %s", e.Attachments[1].Filename, b.Filename)
	}
	if !bytes.Equal(e.Attachments[1].Content, b.Content) {
		t.Fatalf("Incorrect attachment content %#q != %#q", e.Attachments[1].Content, b.Content)
	}
}

func ExampleGmail() {
	e := NewEmail()
	e.From = "Jordan Wright <test@gmail.com>"
	e.To = []string{"test@example.com"}
	e.Bcc = []string{"test_bcc@example.com"}
	e.Cc = []string{"test_cc@example.com"}
	e.Subject = "Awesome Subject"
	e.Text = []byte("Text Body is, of course, supported!\n")
	e.HTML = []byte("<h1>Fancy Html is supported, too!</h1>\n")
	e.Send("smtp.gmail.com:587", smtp.PlainAuth("", e.From, "password123", "smtp.gmail.com"))
}

func ExampleAttach() {
	e := NewEmail()
	e.AttachFile("test.txt")
}

func Test_base64Wrap(t *testing.T) {
	file := "I'm a file long enough to force the function to wrap a\n" +
		"couple of lines, but I stop short of the end of one line and\n" +
		"have some padding dangling at the end."
	encoded := "SSdtIGEgZmlsZSBsb25nIGVub3VnaCB0byBmb3JjZSB0aGUgZnVuY3Rpb24gdG8gd3JhcCBhCmNv\r\n" +
		"dXBsZSBvZiBsaW5lcywgYnV0IEkgc3RvcCBzaG9ydCBvZiB0aGUgZW5kIG9mIG9uZSBsaW5lIGFu\r\n" +
		"ZApoYXZlIHNvbWUgcGFkZGluZyBkYW5nbGluZyBhdCB0aGUgZW5kLg==\r\n"

	var buf bytes.Buffer
	base64Wrap(&buf, []byte(file))
	if !bytes.Equal(buf.Bytes(), []byte(encoded)) {
		t.Fatalf("Encoded file does not match expected: %#q != %#q", string(buf.Bytes()), encoded)
	}
}

// *Since the mime library in use by ```email``` is now in the stdlib, this test is deprecated
func Test_quotedPrintEncode(t *testing.T) {
	var buf bytes.Buffer
	text := []byte("Dear reader!\n\n" +
		"This is a test email to try and capture some of the corner cases that exist within\n" +
		"the quoted-printable encoding.\n" +
		"There are some wacky parts like =, and this input assumes UNIX line breaks so\r\n" +
		"it can come out a little weird.  Also, we need to support unicode so here's a fish: 🐟\n")
	expected := []byte("Dear reader!\r\n\r\n" +
		"This is a test email to try and capture some of the corner cases that exist=\r\n" +
		" within\r\n" +
		"the quoted-printable encoding.\r\n" +
		"There are some wacky parts like =3D, and this input assumes UNIX line break=\r\n" +
		"s so\r\n" +
		"it can come out a little weird.  Also, we need to support unicode so here's=\r\n" +
		" a fish: =F0=9F=90=9F\r\n")
	qp := quotedprintable.NewWriter(&buf)
	if _, err := qp.Write(text); err != nil {
		t.Fatal("quotePrintEncode: ", err)
	}
	if err := qp.Close(); err != nil {
		t.Fatal("Error closing writer", err)
	}
	if b := buf.Bytes(); !bytes.Equal(b, expected) {
		t.Errorf("quotedPrintEncode generated incorrect results: %#q != %#q", b, expected)
	}
}

func TestMultipartNoContentType(t *testing.T) {
	raw := []byte(`From: Mikhail Gusarov <dottedmag@dottedmag.net>
To: notmuch@notmuchmail.org
References: <20091117190054.GU3165@dottiness.seas.harvard.edu>
Date: Wed, 18 Nov 2009 01:02:38 +0600
Message-ID: <87iqd9rn3l.fsf@vertex.dottedmag>
MIME-Version: 1.0
Subject: Re: [notmuch] Working with Maildir storage?
Content-Type: multipart/mixed; boundary="===============1958295626=="

--===============1958295626==
Content-Type: multipart/signed; boundary="=-=-=";
    micalg=pgp-sha1; protocol="application/pgp-signature"

--=-=-=
Content-Transfer-Encoding: quoted-printable

Twas brillig at 14:00:54 17.11.2009 UTC-05 when lars@seas.harvard.edu did g=
yre and gimble:

--=-=-=
Content-Type: application/pgp-signature

-----BEGIN PGP SIGNATURE-----
Version: GnuPG v1.4.9 (GNU/Linux)

iQIcBAEBAgAGBQJLAvNOAAoJEJ0g9lA+M4iIjLYQAKp0PXEgl3JMOEBisH52AsIK
=/ksP
-----END PGP SIGNATURE-----
--=-=-=--

--===============1958295626==
Content-Type: text/plain; charset="us-ascii"
MIME-Version: 1.0
Content-Transfer-Encoding: 7bit
Content-Disposition: inline

Testing!
--===============1958295626==--
`)
	e, err := NewEmailFromReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Error when parsing email %s", err.Error())
	}
	if !bytes.Equal(e.Text, []byte("Testing!")) {
		t.Fatalf("Error incorrect text: %#q != %#q\n", e.Text, "Testing!")
	}
}

func TestNoMultipartHTMLContentTypeBase64Encoding(t *testing.T) {
	raw := []byte(`MIME-Version: 1.0
From: no-reply@example.com
To: tester@example.org
Date: 7 Jan 2021 03:07:44 -0800
Subject: Hello
Content-Type: text/html; charset=utf-8
Content-Transfer-Encoding: base64
Message-Id: <20210107110744.547DD70532@example.com>

PGh0bWw+PGhlYWQ+PHRpdGxlPnRlc3Q8L3RpdGxlPjwvaGVhZD48Ym9keT5IZWxsbyB3
b3JsZCE8L2JvZHk+PC9odG1sPg==
`)
	e, err := NewEmailFromReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("Error when parsing email %s", err.Error())
	}
	if !bytes.Equal(e.HTML, []byte("<html><head><title>test</title></head><body>Hello world!</body></html>")) {
		t.Fatalf("Error incorrect text: %#q != %#q\n", e.Text, "<html>...</html>")
	}
}

// *Since the mime library in use by ```email``` is now in the stdlib, this test is deprecated
func Test_quotedPrintDecode(t *testing.T) {
	text := []byte("Dear reader!\r\n\r\n" +
		"This is a test email to try and capture some of the corner cases that exist=\r\n" +
		" within\r\n" +
		"the quoted-printable encoding.\r\n" +
		"There are some wacky parts like =3D, and this input assumes UNIX line break=\r\n" +
		"s so\r\n" +
		"it can come out a little weird.  Also, we need to support unicode so here's=\r\n" +
		" a fish: =F0=9F=90=9F\r\n")
	expected := []byte("Dear reader!\r\n\r\n" +
		"This is a test email to try and capture some of the corner cases that exist within\r\n" +
		"the quoted-printable encoding.\r\n" +
		"There are some wacky parts like =, and this input assumes UNIX line breaks so\r\n" +
		"it can come out a little weird.  Also, we need to support unicode so here's a fish: 🐟\r\n")
	qp := quotedprintable.NewReader(bytes.NewReader(text))
	got, err := ioutil.ReadAll(qp)
	if err != nil {
		t.Fatal("quotePrintDecode: ", err)
	}

	if !bytes.Equal(got, expected) {
		t.Errorf("quotedPrintDecode generated incorrect results: %#q != %#q", got, expected)
	}
}

func Benchmark_base64Wrap(b *testing.B) {
	// Reasonable base case; 128K random bytes
	file := make([]byte, 128*1024)
	if _, err := rand.Read(file); err != nil {
		panic(err)
	}
	for i := 0; i <= b.N; i++ {
		base64Wrap(ioutil.Discard, file)
	}
}

func TestParseSender(t *testing.T) {
	var cases = []struct {
		e      Email
		want   string
		haserr bool
	}{
		{
			Email{From: "from@test.com"},
			"from@test.com",
			false,
		},
		{
			Email{Sender: "sender@test.com", From: "from@test.com"},
			"sender@test.com",
			false,
		},
		{
			Email{Sender: "bad_address_sender"},
			"",
			true,
		},
		{
			Email{Sender: "good@sender.com", From: "bad_address_from"},
			"good@sender.com",
			false,
		},
	}

	for i, testcase := range cases {
		got, err := testcase.e.parseSender()
		if got != testcase.want || (err != nil) != testcase.haserr {
			t.Errorf(`%d: got %s != want %s or error "%t" != "%t"`, i+1, got, testcase.want, err != nil, testcase.haserr)
		}
	}
}
