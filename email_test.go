package email

import (
	"net/smtp"
	"testing"

	"bytes"
	"crypto/rand"
	"io/ioutil"
)

func TestEmail(*testing.T) {
	e := NewEmail()
	e.From = "Jordan Wright <test@example.com>"
	e.To = []string{"test@example.com"}
	e.Bcc = []string{"test_bcc@example.com"}
	e.Cc = []string{"test_cc@example.com"}
	e.Subject = "Awesome Subject"
	e.Text = "Text Body is, of course, supported!"
	e.HTML = "<h1>Fancy Html is supported, too!</h1>"
}

func ExampleGmail() {
	e := NewEmail()
	e.From = "Jordan Wright <test@gmail.com>"
	e.To = []string{"test@example.com"}
	e.Bcc = []string{"test_bcc@example.com"}
	e.Cc = []string{"test_cc@example.com"}
	e.Subject = "Awesome Subject"
	e.Text = "Text Body is, of course, supported!"
	e.HTML = "<h1>Fancy Html is supported, too!</h1>"
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
