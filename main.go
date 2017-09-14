/*
Uptime
Game (+IGN rating, if available)
Quotes (quote/addquote)
Custom commands (add/remove)
Welcome subs
*/

package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

var (
	CHANNEL       = os.Getenv("BOT_CHANNEL")
	USER          = os.Getenv("BOT_USER")
	PASSWORD      = os.Getenv("BOT_PASSWORD")
	MASHAPE_KEY   = os.Getenv("BOT_MASHAPE_KEY")
	CLIENT_ID     = os.Getenv("BOT_CLIENT_ID")
	CLIENT_SECRET = os.Getenv("BOT_CLIENT_SECRET")
	GITHUB_SECRET = os.Getenv("BOT_GITHUB_SECRET")
	CURRENCY_NAME = os.Getenv("BOT_CURRENCY_NAME")
)

const IRCIdleConnectionTimeout = 5 * time.Minute

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	log.Printf("PASSWORD=%v\n", PASSWORD)
	log.Printf("MASHAPE_KEY=%v\n", MASHAPE_KEY)
	log.Printf("CLIENT_ID=%v\n", CLIENT_ID)
	log.Printf("CLIENT_SECRET=%v\n", CLIENT_SECRET)
	log.Printf("GITHUB_SECRET=%v\n", GITHUB_SECRET)

	log.Print("Let's do this thing!\n")
	c, err := net.Dial("tcp", "irc.chat.twitch.tv:6667")
	must(err)

	in := bufio.NewReader(c)
	out := make(chan string, 1000)

	fmt.Fprintf(c, "CAP REQ :twitch.tv/tags\r\n")
	fmt.Fprintf(c, "PASS oauth:%s\r\n", PASSWORD)
	fmt.Fprintf(c, "USER %s\r\n", USER)
	fmt.Fprintf(c, "NICK %s\r\n", USER)
	fmt.Fprintf(c, "JOIN #%s\r\n", CHANNEL)

	go func() {
		for m := range out {
			//log.Printf("[OUT] %s", m)
			fmt.Fprint(c, m)
			time.Sleep(time.Second)
		}
	}()

	go func() {
		for {
			c.SetReadDeadline(time.Now().Add(IRCIdleConnectionTimeout))
			line, err := in.ReadSlice('\n')
			must(err)
			//log.Printf("[IN]  %s", line)
			go handle(out, parse(line))
		}
	}()

	http.ListenAndServe(":4200", nil)
}
