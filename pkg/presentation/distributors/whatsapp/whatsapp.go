// Copyright (c) 2021-2022, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package whatsapp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/distributors/gettor"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

const (
	DistName = "whatsapp"
)

type whatsapp struct {
	client      *whatsmeow.Client
	distributor *gettor.GettorDistributor
}

func InitFrontend(cfg *internal.Config) {
	var w whatsapp
	w.distributor = &gettor.GettorDistributor{}
	w.distributor.Init(cfg)

	// Connect to the WhatsApp account and set up event handlers
	err := w.connect(cfg)
	if err != nil {
		log.Fatalf("error connecting to WhatsApp: %s", err)
	}

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(cfg.Distributors.Whatsapp.MetricsAddress, nil)

	// Wait for a signal to gracefully disconnect
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	// Disconnect from the WhatsApp account
	w.disconnect()
}

func (w *whatsapp) connect(cfg *internal.Config) error {
	// Initialize the database logger
	dbLog := waLog.Stdout("Database", "DEBUG", true)

	//Initialize the database container
	container, err := sqlstore.New("sqlite3", "file:"+cfg.Distributors.Whatsapp.SessionFile+"?_foreign_keys=on", dbLog)
	if err != nil {
		return err
	}
	// Get the first device from the container
	device, err := container.GetFirstDevice()
	if err != nil {
		return err
	}

	// Initialize the client
	clientLog := waLog.Stdout("Client", "INFO", true)
	w.client = whatsmeow.NewClient(device, clientLog)
	w.client.AddEventHandler(w.eventHandler)

	// Connect to WhatsApp
	if w.client.Store.ID == nil {
		//If the device is not logged in, get the QR code and wait for the user to scan it
		qrChan, _ := w.client.GetQRChannel(context.Background())
		err = w.client.Connect()
		if err != nil {
			return err
		}

		for channelItem := range qrChan {
			if channelItem.Event == "code" {
				// Render the QR code here
				qrterminal.GenerateHalfBlock(channelItem.Code, qrterminal.L, os.Stdout)
				// or just manually `echo 2@... | qrencode -t ansiutf8` in a terminal
				fmt.Println("Scan the QR code above to log in", channelItem.Code)
				// you can use the channelItem to render QR code or send it via HTTP, etc
			} else {
				log.Println("Login Success", channelItem.Event)
				break
			}
		}
	} else {
		// Already logged in, just connect
		err = w.client.Connect()
		log.Println("Login Success")
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *whatsapp) disconnect() {
	w.client.Disconnect()
}

func (w *whatsapp) eventHandler(evt interface{}) {
	supportedPlatforms := w.distributor.SupportedPlatforms()
	switch v := evt.(type) {
	case *events.Message:
		platform := strings.ToLower(v.Message.GetConversation())

		// Check if the platform is one of the supported platforms
		if contains(supportedPlatforms, platform) {
			log.Println("Requested platform:", platform)
			links := w.distributor.GetAliasedLinks(platform)
			// Send the links to the recipient via WhatsApp
			for _, link := range links {
				if err := w.sendMessage(link.Link, v.Info.Chat); err != nil {
					log.Println("Error sending the links message:", err)
				}
			}
		} else {
			log.Printf("Give help: '%s'", platform)
			platformList := strings.Join(supportedPlatforms, ", ")
			message := fmt.Sprintf("What Operative Systemd do you want? The supported Operative Systemds are: %s", platformList)
			if err := w.sendMessage(message, v.Info.Chat); err != nil {
				log.Println("Error sending help:", err)
			}
		}
	}
}

func (w *whatsapp) sendMessage(message string, receiver types.JID) error {
	_, err := w.client.SendMessage(context.Background(), receiver, &waProto.Message{
		Conversation: proto.String(message),
	})
	if err != nil {
		return err
	}
	return nil
}

func contains(platformSlice []string, elem string) bool {
	for _, platform := range platformSlice {
		if platform == elem {
			return true
		}
	}
	return false
}
