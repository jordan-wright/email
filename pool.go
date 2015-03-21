package email

import (
	"crypto/tls"
	"errors"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"sync"
	"syscall"
	"time"
)

type Pool struct {
	addr    string
	auth    smtp.Auth
	max     int
	created int
	ch      chan *smtp.Client
	decs    chan struct{}
	mut     *sync.Mutex
}

var ErrTimeout = errors.New("timed out")

func NewPool(address string, auth smtp.Auth, count int) *Pool {
	return &Pool{
		addr: address,
		auth: auth,
		max:  count,
		ch:   make(chan *smtp.Client, count),
		decs: make(chan struct{}),
		mut:  &sync.Mutex{},
	}
}

func (p *Pool) get(timeout time.Duration) *smtp.Client {
	select {
	case c := <-p.ch:
		return c
	default:
	}

	if p.created < p.max {
		p.makeOne()
	}

	deadline := time.After(timeout)
	for {
		select {
		case c := <-p.ch:
			return c
		case <-p.decs:
			p.makeOne()
		case <-deadline:
			return nil
		}
	}
}

func shouldReuse(err error) bool {
	// probably needs tweaking, but might be close:
	//  - textproto.Errors were valid SMTP over a valid connection,
	//    but resulted from an SMTP error response
	//  - textproto.ProtocolErrors result from connections going down,
	//    invalid SMTP, that sort of thing
	//  - syscall.Errno is probably down connection/bad pipe, but
	//    passed straight through by textproto instead of becoming a
	//    ProtocolError
	//  - if we don't recognize the error, don't reuse the connection
	switch err.(type) {
	case *textproto.Error:
		return true
	case *textproto.ProtocolError, textproto.ProtocolError:
		return false
	case syscall.Errno:
		return false
	default:
		return false
	}
}

func (p *Pool) replace(c *smtp.Client) {
	p.ch <- c
}

func (p *Pool) inc() bool {
	if p.created >= p.max {
		return false
	}

	p.mut.Lock()
	defer p.mut.Unlock()

	if p.created >= p.max {
		return false
	}
	p.created++
	return true
}

func (p *Pool) dec() {
	p.mut.Lock()
	p.created--
	p.mut.Unlock()

	select {
	case p.decs <- struct{}{}:
	default:
	}
}

func (p *Pool) makeOne() {
	go func() {
		if p.inc() {
			if c, err := p.build(); err == nil {
				p.ch <- c
			} else {
				p.dec()
			}
		}
	}()
}

func (p *Pool) build() (*smtp.Client, error) {
	c, err := smtp.Dial(p.addr)
	if err != nil {
		return nil, err
	}

	onErr := func(err error) error {
		c.Quit()
		c.Close()
		return err
	}

	if ok, _ := c.Extension("STARTTLS"); ok {
		host, _, err := net.SplitHostPort(p.addr)
		if err != nil {
			return nil, onErr(err)
		}
		if err = c.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return nil, onErr(err)
		}
	}

	if p.auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(p.auth); err != nil {
				return nil, onErr(err)
			}
		}
	}

	return c, nil
}

func (p *Pool) Send(e *Email, timeout time.Duration) (err error) {
	c := p.get(timeout)
	if c == nil {
		return ErrTimeout
	}

	defer func() {
		if err != nil {
			if shouldReuse(err) {
				c.Reset()
				p.replace(c)
			} else {
				p.dec()
				c.Quit()
				c.Close()
			}
		} else {
			p.replace(c)
		}
	}()

	recipients, err := addressLists(e.To, e.Cc, e.Bcc)
	if err != nil {
		return
	}

	msg, err := e.Bytes()
	if err != nil {
		return
	}

	from, err := emailOnly(e.From)
	if err != nil {
		return
	}
	if err = c.Mail(from); err != nil {
		return
	}

	for _, recip := range recipients {
		if err = c.Rcpt(recip); err != nil {
			return
		}
	}

	w, err := c.Data()
	if err != nil {
		return
	}
	if _, err = w.Write(msg); err != nil {
		return
	}

	err = w.Close()

	return
}

func emailOnly(full string) (string, error) {
	addr, err := mail.ParseAddress(full)
	if err != nil {
		return "", err
	}
	return addr.Address, nil
}

func addressLists(lists ...[]string) ([]string, error) {
	length := 0
	for _, lst := range lists {
		length += len(lst)
	}
	combined := make([]string, 0, length)

	for _, lst := range lists {
		for _, full := range lst {
			addr, err := emailOnly(full)
			if err != nil {
				return nil, err
			}
			combined = append(combined, addr)
		}
	}

	return combined, nil
}
