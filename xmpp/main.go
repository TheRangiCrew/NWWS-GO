package main

import (
	"context"
	"encoding/xml"
	"io"
	"io/fs"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

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

func padLeft(text string, padString string, pad int) string {
	if len(text) < pad {
		for i := len(text); i < pad; i++ {
			text = padString + text
		}
	}
	return text
}

func readOrCreateDir(dirName string) ([]fs.DirEntry, error) {
	dir, err := os.ReadDir(dirName)
	if err != nil {
		log.Println(err)
		if len(dir) == 0 {
			err = os.Mkdir(dirName, os.ModePerm)
			if err != nil {
				return nil, err
			}
			dir, err = os.ReadDir(dirName)
			if err != nil {
				return nil, err
			}
		}
	}

	return dir, nil
}

func writeToFile(text string) error {

	dirName := os.Getenv("PRODUCT_QUEUE_DIR")

	_, err := readOrCreateDir(dirName)
	if err != nil {
		return err
	}

	t := time.Now().UTC()
	dateString := strconv.Itoa(t.Year()) + padLeft(strconv.Itoa(int(t.Month())), "0", 2) + padLeft(strconv.Itoa(t.Day()), "0", 2)

	dirName = dirName + dateString + "/"

	dir, err := readOrCreateDir(dirName)
	if err != nil {
		return err
	}

	index := 1
	// Custom sorting function
	sort.Slice(dir, func(i, j int) bool {
		// Extract numeric values from file names
		numI, errI := strconv.Atoi(strings.Split(dir[i].Name(), ".")[0])
		numJ, errJ := strconv.Atoi(strings.Split(dir[j].Name(), ".")[0])

		// If both names are numeric, compare numerically
		if errI == nil && errJ == nil {
			return numI < numJ
		}

		// Otherwise, compare lexicographically
		return dir[i].Name() < dir[j].Name()
	})

	if len(dir) > 0 {
		nameString := dir[len(dir)-1].Name()
		name := strings.Split(nameString, ".")[0]
		index, err = strconv.Atoi(name)
		if err != nil {
			return err
		}
		index++
	}

	file, err := os.Create(dirName + strconv.Itoa(index) + ".txt")
	if err != nil {
		return err
	}

	_, err = file.WriteString(text)
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}

	return nil

}

func handleConnection(session *xmpp.Session) error {
	username := os.Getenv("NWWS_USER")
	server := os.Getenv("NWWS_SERVER")
	room := os.Getenv("NWWS_ROOM")
	resource := os.Getenv("NWWS_RESOURCE")

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

	log.Printf("Connected to NWWS-OI! Ready to receive...\n\n")

	err = session.Serve(xmpp.HandlerFunc(func(t xmlstream.TokenReadEncoder, start *xml.StartElement) error {
		d := xml.NewTokenDecoder(t)

		// Ignore anything that's not a message. In a real system we'd want to at
		// least respond to IQs.
		if start.Name.Local != "message" {
			return nil
		}

		msg := Message{}
		_ = d.DecodeElement(&msg, start)
		if err != nil && err != io.EOF {
			log.Printf("Error decoding message: %q", err)
		}

		err = writeToFile(msg.X.Text)
		if err != nil {
			return err
		}

		return nil
	}))

	return err
}

func connection() (*xmpp.Session, error) {
	username := os.Getenv("NWWS_USER")
	password := os.Getenv("NWWS_PASS")
	server := os.Getenv("NWWS_SERVER")

	session, err := xmpp.DialClientSession(
		context.TODO(),
		jid.MustParse(username+"@"+server),
		xmpp.SASL(username, password, sasl.ScramSha1Plus, sasl.ScramSha1, sasl.Plain),
		xmpp.BindResource(),
	)

	if err != nil {
		return nil, err
	}

	return session, nil
}

func main() {

	// err := godotenv.Load(".env")
	// if err != nil {
	// 	log.Fatal("Error loading .env file")
	// }

	for {
		session, err := connection()
		if err != nil {
			log.Printf("Error connecting: %v\n\nWaiting 60 seconds before retrying", err)
			time.Sleep(60 * time.Second) // Wait for a while before retrying
			continue
		}

		if err := handleConnection(session); err != nil {
			log.Printf("Error in XMPP session: %v", err)
		}
	}
}
