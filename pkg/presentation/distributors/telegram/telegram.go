// Copyright (c) 2021-2023, The Tor Project, Inc.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package telegram

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/internal"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/locales"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/persistence"
	pjson "gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/persistence/json"
	"gitlab.torproject.org/tpo/anti-censorship/rdsys/pkg/usecases/distributors/telegram"
	tb "gopkg.in/telebot.v3"
)

const (
	TelegramPollTimeout = 10 * time.Second
)

type TBot struct {
	bot          *tb.Bot
	dist         *telegram.TelegramDistributor
	i18nBundle   *i18n.Bundle
	updateTokens map[string]string

	// menu maps locales to their buttons
	menu map[string]*tb.ReplyMarkup
}

// InitFrontend is the entry point to telegram'ss frontend.  It connects to telegram over
// the bot API and waits for user commands.
func InitFrontend(cfg *internal.Config) {
	newBridgesStore := make(map[string]persistence.Mechanism, len(cfg.Distributors.Telegram.UpdaterTokens))
	for updater := range cfg.Distributors.Telegram.UpdaterTokens {
		newBridgesStore[updater] = pjson.New(updater, cfg.Distributors.Telegram.StorageDir)
	}

	seenIdStore := pjson.New("seen_ids", cfg.Distributors.Telegram.StorageDir)

	dist := telegram.TelegramDistributor{
		NewBridgesStore: newBridgesStore,
		IdStore:         seenIdStore,
	}
	dist.Init(cfg)

	tbot, err := newTBot(cfg.Distributors.Telegram.Token, &dist)
	if err != nil {
		log.Fatal(err)
	}
	tbot.updateTokens = cfg.Distributors.Telegram.UpdaterTokens

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT)
	signal.Notify(signalChan, syscall.SIGTERM)
	go func() {
		<-signalChan
		log.Printf("Caught SIGINT.")
		dist.Shutdown()

		log.Printf("Shutting down the telegram bot.")
		tbot.Stop()
	}()

	http.HandleFunc("/update", tbot.updateHandler)
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(cfg.Distributors.Telegram.ApiAddress, nil)

	tbot.Start()
}

func newTBot(token string, dist *telegram.TelegramDistributor) (*TBot, error) {
	var t TBot
	var err error

	t.i18nBundle, err = locales.NewBundle()
	if err != nil {
		return nil, err
	}

	t.dist = dist
	t.bot, err = tb.NewBot(tb.Settings{
		Token:  token,
		Poller: &tb.LongPoller{Timeout: TelegramPollTimeout},
	})
	if err != nil {
		return nil, err
	}

	t.bot.Handle("/start", t.getMenu)
	t.bot.Handle("/bridges", t.getBridges)
	t.bot.Handle("/lox", t.getLoxInvitation)
	t.bot.Handle("/loxhelp", t.getLoxHelp)
	t.bot.Handle("/help", t.getHelp)

	t.initializeMenus()

	return &t, nil
}

func (t *TBot) initializeMenus() {
	t.menu = make(map[string]*tb.ReplyMarkup)
	for _, language := range t.i18nBundle.LanguageTags() {
		menu := &tb.ReplyMarkup{ResizeKeyboard: true}

		localizer := i18n.NewLocalizer(t.i18nBundle, language.String(), locales.DefaultLanguage)

		msgBridges, _ := localizer.Localize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "TelegramBridgesButton",
				Other: "Bridges",
			},
		})
		btnBridges := menu.Text(msgBridges)

		//		msgLox, _ := localizer.Localize(&i18n.LocalizeConfig{
		//			DefaultMessage: &i18n.Message{
		//				ID:    "TelegramLoxInviteButton",
		//				Other: "Lox Invite *(alpha)*",
		//			},
		//		})
		//		btnLoxInvite := menu.Text(msgLox)

		msgHelp, _ := localizer.Localize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "TelegramHelpButton",
				Other: "Help",
			},
		})
		btnHelp := menu.Text(msgHelp)

		menu.Reply(
			menu.Row(btnBridges),
			//	menu.Row(btnLoxInvite),
			menu.Row(btnHelp),
		)

		t.bot.Handle(&btnHelp, t.getHelp)
		//		t.bot.Handle(&btnLoxInvite, t.getLoxInvitation)
		t.bot.Handle(&btnBridges, t.getBridges)
		t.menu[language.String()] = menu
	}
}

func (t *TBot) Start() {
	t.bot.Start()
}

func (t *TBot) Stop() {
	t.bot.Stop()
}

func (t *TBot) getHelp(c tb.Context) error {
	const helpmsg = "To use your bridges on Android:\n\n" +
		"1. When you start Tor Browser, " +
		"click the Settings icon.\n\n" +
		"2. Select 'Config Bridge'.\n\n" +
		"3. Make sure the 'Use a Bridge' setting is " +
		"switched on and that the 'obfs4' option " +
		"is selected.\n\n" +
		"4. Copy the message with the bridges " +
		"you received.\n\n" +
		"5. Select 'Provide a Bridge I know' and " +
		"paste the bridges into the pop-up.\n\n" +
		"6. Return to the connect page and " +
		"press the 'Connect' button.\n\n\n" +
		"To use your bridges on desktop:\n\n" +
		"1. In the menu with three bars (≡) in " +
		"the upper right corner, select " +
		"'Settings'. In the left column, " +
		"select 'Connection'. " +
		"If you launched Tor Browser without " +
		"connecting, you can also press the " +
		"'Configure Connection...' button.\n\n" +
		"2. Under the 'Bridges' section, switch on the " +
		"'Use current bridges' setting.\n\n" +
		"3. Copy the message with the bridges " +
		"you received.\n\n" +
		"4. Under 'Add a New Bridge', click the " +
		"'Add a Bridge Manually...' button.\n\n" +
		"5. Paste the bridges into the 'Add a " +
		"Bridge Manually' pop-up (one bridge per line).\n\n" +
		"6. If Tor Browser is already connected to Tor, " +
		"restart it to save your changes. " +
		"If Tor Browser is not connected to Tor, select " +
		"'Connect' at the top of the connection page."

	localizer, menu := t.newLocalizer(c)
	msg, _ := localizer.Localize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "TelegramHelp",
			Other: helpmsg,
		},
	})
	return c.Send(msg, menu)
}

func (t *TBot) getLoxHelp(c tb.Context) error {
	const helpmsg = "Lox *(alpha)* is not quite ready yet, but will be available soon!"

	localizer, menu := t.newLocalizer(c)
	msg, _ := localizer.Localize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "TelegramLoxHelp",
			Other: helpmsg,
		},
	})
	return c.Send(msg, menu)
}

func (t *TBot) getMenu(c tb.Context) error {
	const welcomemsg = "Welcome! To get bridges, type /bridges " +
		"or press the Bridges button. \n\n" +
		"To get information about how to use your bridges, " +
		"type /help or press the Help button.\n\n" +
		"We are currently alpha testing a new privacy-preserving, " +
		"reputation-based bridge distribution system called Lox. " +
		"To try out Lox and help us with testing, type /lox to get " +
		"a Lox invitation\n\n" +
		"To get information about how to use your invitation, " +
		"type /loxhelp."
	localizer, menu := t.newLocalizer(c)

	msg, _ := localizer.Localize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "TelegramWelcome",
			Other: welcomemsg,
		},
	})

	t.bot.Send(c.Sender(), msg, menu)
	return nil
}

func (t *TBot) getBridges(c tb.Context) error {
	localizer, _ := t.newLocalizer(c)
	if c.Sender().IsBot {
		msg, _ := localizer.Localize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "TelegramNoBridges",
				Other: "No bridges for bots, sorry",
			},
		})
		return c.Send(msg)
	}
	userID := c.Sender().ID
	resources := t.dist.GetResources(userID)
	msg, _ := localizer.Localize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:    "TelegramBridges",
			One:   "***Your Bridge:***",
			Other: "***Your bridges:***",
		},
		PluralCount: len(resources),
	})
	t.bot.Send(c.Sender(), msg, tb.ModeMarkdown)
	response := ""
	for _, r := range resources {
		response += "\n" + r.String()
	}
	t.bot.Send(c.Sender(), response, tb.ModeMarkdown)

	return nil
}
func (t *TBot) getLoxInvitation(c tb.Context) error {
	localizer, _ := t.newLocalizer(c)
	if c.Sender().IsBot {
		msg, _ := localizer.Localize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "TelegramNoInvitation",
				Other: "No invitation for bots, sorry",
			},
		})
		return c.Send(msg)
	}
	userID := c.Sender().ID
	invitation, err := t.dist.GetInvitation(userID)
	if err != nil {
		var error_msg string
		switch x := err.(type) {
		case *telegram.IdFreshnessError:
			error_msg, _ = localizer.Localize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "IdFreshnessError",
					Other: "You account is too new, invitation can not be issued.",
				},
			})
		case *telegram.InvitationLimitError:
			localized_err, _ := localizer.Localize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "InvitationLimitError",
					Other: "You have already requested an invite, you can request again on %s",
				},
			})
			error_msg = fmt.Sprintf(localized_err, x.Error())
		case *telegram.LoxRequestError:
			error_msg, _ = localizer.Localize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "LoxRequestError",
					Other: "There was a problem making the invite request. Try again in a while",
				},
			})
		default:
			error_msg, _ = localizer.Localize(&i18n.LocalizeConfig{
				DefaultMessage: &i18n.Message{
					ID:    "LoxErrorMessage",
					Other: "Unknown Lox Error, please try again",
				},
			})
		}
		t.bot.Send(c.Sender(), error_msg, tb.ModeMarkdown)
		return nil
	}
	msg, _ := localizer.Localize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID:  "LoxInvitation",
			One: "***Your Lox Invitation:***",
		},
	})
	t.bot.Send(c.Sender(), msg, tb.ModeMarkdown)
	var v map[string]string
	err = json.Unmarshal(invitation, &v)
	if err != nil {
		error_msg, _ := localizer.Localize(&i18n.LocalizeConfig{
			DefaultMessage: &i18n.Message{
				ID:    "LoxErrorMessage",
				Other: "Unknown Lox Error, please try again",
			},
		})
		t.bot.Send(c.Sender(), error_msg, tb.ModeMarkdown)
	}
	response := string(v["invite"])
	t.bot.Send(c.Sender(), response, tb.ModeMarkdown)

	return nil
}

func (t *TBot) updateHandler(w http.ResponseWriter, r *http.Request) {
	name := t.getTokenName(w, r)
	if name == "" {
		return
	}
	defer r.Body.Close()

	err := t.dist.LoadNewBridges(name, r.Body)
	if err != nil {
		log.Printf("Error loading bridges: %v", err)
		http.Error(w, "error while loading bridges", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (t *TBot) getTokenName(w http.ResponseWriter, r *http.Request) string {
	tokenLine := r.Header.Get("Authorization")
	if tokenLine == "" {
		log.Printf("Request carries no 'Authorization' HTTP header.")
		http.Error(w, "request carries no 'Authorization' HTTP header", http.StatusBadRequest)
		return ""
	}
	if !strings.HasPrefix(tokenLine, "Bearer ") {
		log.Printf("Authorization header contains no bearer token.")
		http.Error(w, "authorization header contains no bearer token", http.StatusBadRequest)
		return ""
	}
	fields := strings.Split(tokenLine, " ")
	givenToken := fields[1]

	for name, savedToken := range t.updateTokens {
		if givenToken == savedToken {
			return name
		}
	}

	log.Printf("Invalid authentication token.")
	http.Error(w, "invalid authentication token", http.StatusUnauthorized)
	return ""
}

func (t *TBot) newLocalizer(c tb.Context) (*i18n.Localizer, *tb.ReplyMarkup) {
	var lang string
	user := c.Sender()
	if user != nil {
		lang = user.LanguageCode
	}

	menu, ok := t.menu[lang]
	if !ok {
		menu = t.menu[locales.DefaultLanguage]
	}

	return i18n.NewLocalizer(t.i18nBundle, lang), menu
}
