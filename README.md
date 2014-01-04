email
=====

[![Build Status](https://travis-ci.org/jordan-wright/email.png?branch=master)](https://travis-ci.org/jordan-wright/email)

Robust and flexible email library for Go

### Email for humans
The ```email``` package is designed to be simple to use, but flexible enough so as not to be restrictive. The goal is to provide an *email interface for humans*.

The ```email``` package currently supports the following:
*  From, To, Bcc, and Cc fields
*  Email addresses in both "test@example.com" and "First Last &lt;test@example.com&gt;" format
*  Text and Html Message Body
*  Attachments
*  Read Receipts
*  Custom headers
*  More to come!

### Installation
```go get github.com/jordan-wright/email```

*Note: Requires go version 1.1 and above*

### Examples
#### Sending email using Gmail
```
e := NewEmail()
e.From = "Jordan Wright <test@gmail.com>"
e.To = []string{"test@example.com"}
e.Bcc = []string{"test_bcc@example.com"}
e.Cc = []string{"test_cc@example.com"}
e.Subject = "Awesome Subject"
e.Text = "Text Body is, of course, supported!"
e.Html = "<h1>Fancy Html is supported, too!</h1>"
e.Send("smtp.gmail.com:587", smtp.PlainAuth("", "test@gmail.com", "password123", "smtp.gmail.com"))
```

#### Attaching a File
```
e := NewEmail()
e.Attach("test.txt")
```

### Documentation
[http://godoc.org/github.com/jordan-wright/email](http://godoc.org/github.com/jordan-wright/email)

### Other Sources
Sections inspired by the handy [gophermail](https://github.com/jpoehls/gophermail) project.