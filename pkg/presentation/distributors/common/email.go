// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net/mail"
	"net/smtp"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/distributors"
)

var (
	emailCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "email_processed_total",
		Help: "The total number of emails processed",
	},
		[]string{"status", "type"},
	)
)

const (
	durationIgnoreEmails = 24 * time.Hour
)

type SendFunction func(subject, body string) error
type IncomingEmailHandler func(msg *mail.Message, send SendFunction) error

type emailClient struct {
	cfg             *internal.EmailConfig
	imap            *client.Client
	dist            distributors.Distributor
	incomingHandler IncomingEmailHandler
	smtpAuth        *smtp.Auth
}

func StartEmail(emailCfg *internal.EmailConfig, distCfg *internal.Config,
	dist distributors.Distributor, incomingHandler IncomingEmailHandler) {

	dist.Init(distCfg)
	e := emailClient{
		cfg:             emailCfg,
		dist:            dist,
		incomingHandler: incomingHandler,
	}
	if emailCfg.SmtpUsername != "" && emailCfg.SmtpPassword != "" {
		smtpHost := strings.Split(emailCfg.SmtpServer, ":")[0]
		smtpAuth := smtp.PlainAuth("", emailCfg.SmtpUsername, emailCfg.SmtpPassword, smtpHost)
		e.smtpAuth = &smtpAuth
	}

	stop := make(chan struct{})
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT)
	signal.Notify(signalChan, syscall.SIGTERM)
	go func() {
		<-signalChan
		log.Printf("Caught SIGINT.")
		e.dist.Shutdown()

		close(stop)
		e.imap.Logout()
	}()

	stopped := false
	for !stopped {
		var err error
		e.imap, err = initImap(emailCfg)
		if err != nil {
			log.Println("Can't init the imap client:", err)
			time.Sleep(time.Second)
			continue
		}

		if err := e.listenImapUpdates(stop); err != nil {
			log.Println("Error listening emails:", err)
		} else {
			stopped = true
		}
		e.imap.Logout()
	}
}

func initImap(emailCfg *internal.EmailConfig) (c *client.Client, err error) {
	splitedAddress := strings.Split(emailCfg.ImapServer, "://")
	if len(splitedAddress) != 2 {
		return nil, fmt.Errorf("Malformed imap server configuration: %s", emailCfg.ImapServer)
	}
	protocol := splitedAddress[0]
	serverAddress := splitedAddress[1]

	switch protocol {
	case "imaps":
		c, err = client.DialTLS(serverAddress, nil)
	case "imap":
		c, err = client.Dial(serverAddress)
	default:
		return nil, fmt.Errorf("Unkown protocol: %s", protocol)
	}
	if err != nil {
		return nil, err
	}

	err = c.Login(emailCfg.ImapUsername, emailCfg.ImapPassword)
	return c, err
}

func (e *emailClient) listenImapUpdates(stop <-chan struct{}) error {
	mbox, err := e.imap.Select("INBOX", false)
	if err != nil {
		return err
	}
	e.fetchMessages(mbox)

	for {
		select {
		case <-stop:
			return nil
		default:
			update, err := e.waitForMailboxUpdate()
			if err != nil {
				return err
			}
			e.fetchMessages(update.Mailbox)
		}
	}
}

func (e *emailClient) waitForMailboxUpdate() (mboxUpdate *client.MailboxUpdate, err error) {
	// Create a channel to receive mailbox updates
	updates := make(chan client.Update, 1)
	e.imap.Updates = updates

	// Start idling
	done := make(chan error, 1)
	stop := make(chan struct{})
	go func() {
		done <- e.imap.Idle(stop, &client.IdleOptions{})
	}()

	// Listen for updates
waitLoop:
	for {
		select {
		case update := <-updates:
			var ok bool
			mboxUpdate, ok = update.(*client.MailboxUpdate)
			if ok {
				break waitLoop
			}
		case err := <-done:
			return nil, err
		}
	}

	// We need to nil the updates channel or the client will hang on it
	// https://github.com/emersion/go-imap-idle/issues/16
	e.imap.Updates = nil
	close(stop)
	<-done

	return mboxUpdate, nil
}

func (e *emailClient) fetchMessages(mboxStatus *imap.MailboxStatus) {
	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag, imap.DeletedFlag}
	seqs, err := e.imap.Search(criteria)
	if err != nil {
		log.Println("Error getting unseen messages:", err)
		return
	}

	if len(seqs) == 0 {
		return
	}

	seqset := new(imap.SeqSet)
	seqset.AddNum(seqs...)
	items := []imap.FetchItem{imap.FetchItem("BODY.PEEK[]")}

	log.Println("fetch", len(seqs), "messages from the imap server")
	messages := make(chan *imap.Message, mboxStatus.Messages)
	go func() {
		err := e.imap.Fetch(seqset, items, messages)
		if err != nil {
			log.Println("Error fetching imap messages:", err)
		}
	}()

	for msg := range messages {
		flag := ""
		for _, literal := range msg.Body {
			email, err := mail.ReadMessage(literal)
			if err != nil {
				log.Println("Error parsing incoming email", err)
				emailCount.WithLabelValues("error", "parsing").Inc()
				continue
			}
			if dropEmail(email) {
				flag = imap.DeletedFlag
				continue
			}

			send := func(subject, body string) error {
				return e.reply(email, subject, body)
			}

			err = e.incomingHandler(email, send)
			if err != nil {
				log.Println("Error handling incoming email ", email.Header.Get("Message-ID"), ":", err)
				emailCount.WithLabelValues("error", "handling").Inc()

				date, err := email.Header.Date()
				if flag == "" &&
					(err != nil || date.Add(durationIgnoreEmails).Before(time.Now())) {

					log.Println("Give up with the email, marked as readed so it will not be processed anymore")
					flag = imap.SeenFlag
				}
			} else {
				// delete the email as it was fully processed
				flag = imap.DeletedFlag
				emailCount.WithLabelValues("success", "handling").Inc()
			}
		}
		if flag != "" {
			seqset := new(imap.SeqSet)
			seqset.AddNum(msg.SeqNum)

			item := imap.FormatFlagsOp(imap.AddFlags, true)
			flags := []interface{}{flag}
			err := e.imap.Store(seqset, item, flags, nil)
			if err != nil {
				log.Println("Error setting the delete flag", err)
			}
		}
	}

	if err := e.imap.Expunge(nil); err != nil {
		log.Println("Error expunging messages from inbox", err)
	}
}

func dropEmail(msg *mail.Message) bool {
	// automatic responses RFC 3834
	if header := msg.Header.Get("Auto-Submitted"); header != "" && header != "no" {
		log.Println("Drop autogenerated email:", msg.Header.Get("Message-ID"))
		emailCount.WithLabelValues("drop", "auto-submitted").Inc()
		return true
	}

	// reports of Mail System Administrative Messages RFC 3462
	if header := msg.Header.Get("Content-Type"); strings.Contains(header, "multipart/report") {
		log.Println("Drop report email:", msg.Header.Get("Message-ID"))
		emailCount.WithLabelValues("drop", "report").Inc()
		return true
	}
	return false
}

func (e *emailClient) reply(originalMessage *mail.Message, subject, body string) error {
	sender, err := originalMessage.Header.AddressList("From")
	if err != nil {
		return err
	}
	if len(sender) != 1 {
		return fmt.Errorf("Unexpected email from: %s", originalMessage.Header.Get("From"))
	}

	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"In-Reply-To: %s\r\n"+
		"Auto-Submitted: auto-replied\r\n"+
		"MIME-version: 1.0\r\n"+
		"Content-Type: text/plain; charset=\"utf-8\"\r\n"+
		"\r\n",
		e.cfg.Address,
		sender[0].String(),
		subject,
		originalMessage.Header.Get("Message-ID"),
	)
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		msg += scanner.Text() + "\r\n"
	}
	return e.send(sender[0].Address, msg)
}

func (e *emailClient) send(to string, msg string) error {
	c, err := smtp.Dial(e.cfg.SmtpServer)
	if err != nil {
		return err
	}

	if e.smtpAuth != nil {
		tlsConfig := tls.Config{
			ServerName: strings.Split(e.cfg.SmtpServer, ":")[0],
		}
		if err := c.StartTLS(&tlsConfig); err != nil {
			return err
		}
		if err := c.Auth(*e.smtpAuth); err != nil {
			return err
		}
	}

	if err := c.Mail(e.cfg.Address); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}

	wc, err := c.Data()
	if err != nil {
		return err
	}
	wc.Write([]byte(msg))
	err = wc.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}
