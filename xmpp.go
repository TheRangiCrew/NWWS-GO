package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/TheRangiCrew/NWWS-GO/internal"
	"mellium.im/sasl"
	"mellium.im/xmlstream"
	"mellium.im/xmpp"
	"mellium.im/xmpp/jid"
	"mellium.im/xmpp/stanza"
)

type Message struct {
	XMLName xml.Name `xml:"message"`
	To      string   `xml:"to,attr"`
	Type    string   `xml:"type,attr"`
	From    string   `xml:"from,attr"`
	Body    string   `xml:"body"`
	HTML    struct {
		XMLName xml.Name `xml:"html"`
		Body    struct {
			XMLName xml.Name `xml:"body"`
			Text    string   `xml:",chardata"`
		} `xml:"body"`
	} `xml:"html"`
	X struct {
		XMLName xml.Name `xml:"x"`
		Text    string   `xml:",chardata"`
		Cccc    string   `xml:"cccc,attr"`
		Ttaaii  string   `xml:"ttaaii,attr"`
		Issue   string   `xml:"issue,attr"`
		AwipsID string   `xml:"awipsid,attr"`
		ID      string   `xml:"id,attr"`
	} `xml:"x"`
}

func XMPP() {

	username := os.Getenv("NWWS_USER")
	password := os.Getenv("NWWS_PASS")
	server := os.Getenv("NWWS_SERVER")
	room := os.Getenv("NWWS_ROOM")
	resource := os.Getenv("NWWS_RESOURCE")

	session, err := xmpp.DialClientSession(
		context.TODO(),
		jid.MustParse(username+"@"+server),
		xmpp.SASL(username, password, sasl.ScramSha1Plus, sasl.ScramSha1, sasl.Plain),
		xmpp.BindResource(),
	)

	if err != nil {
		log.Fatalf(err.Error())
	}

	to, err := jid.New(resource, room, username)
	if err != nil {
		log.Fatalf(err.Error())
	}
	from, err := jid.New("", server, username)
	if err != nil {
		log.Fatalf(err.Error())
	}

	decoder := xml.NewDecoder(strings.NewReader("<x></x>"))

	// Send initial presence to let the server know we want to receive messages.
	err = session.Send(context.TODO(), stanza.Presence{To: to, From: from}.Wrap(decoder))

	if err != nil {
		log.Fatalf(err.Error())
	}

	fmt.Print("\n\nConnected to NWWS-OI! Ready to receive...\n\n")

	session.Serve(xmpp.HandlerFunc(func(t xmlstream.TokenReadEncoder, start *xml.StartElement) error {
		d := xml.NewTokenDecoder(t)

		// Ignore anything that's not a message. In a real system we'd want to at
		// least respond to IQs.
		if start.Name.Local != "message" {
			return nil
		}

		msg := Message{}
		_ = d.DecodeElement(&msg, start)
		// if err != nil && err != io.EOF {
		// 	// log.Printf("Error decoding message: %q", err)
		// 	// fmt.Println(msg.Body)
		// 	// return nil
		// }

		errCh := make(chan error)
		go internal.Processor(msg.X.Text, errCh)
		err := <-errCh
		if err != nil {
			log.Println(err)
		}

		return nil
	}))

}
