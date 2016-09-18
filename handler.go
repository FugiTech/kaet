package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	cmdPrefixes []string
	quotes      *store
	cmds        *commands
)

type commands struct {
	sync.RWMutex
	cmds     map[string]*command
	aliases  map[string]string
	rAliases map[string][]string
	store    *store
}

type command struct {
	fn      func(string) string
	modOnly bool
}

func (c *commands) Alias(alias, actual string) {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.cmds[actual]; !ok {
		panic(fmt.Errorf("Invalid alias: %s -> %s", alias, actual))
	}
	c.aliases[alias] = actual
	c.rAliases[actual] = append(c.rAliases[actual], alias)
}

func (c *commands) Get(key string) *command {
	c.RLock()
	defer c.RUnlock()
	if nKey, ok := c.aliases[key]; ok {
		key = nKey
	}
	return c.cmds[key]
}

func init() {
	cmdPrefixes = []string{"!", USER + " ", fmt.Sprintf("@%s ", USER)}

	quotes = Store("quotes")

	cmds = &commands{
		cmds:     map[string]*command{},
		aliases:  map[string]string{},
		rAliases: map[string][]string{},
		store:    Store("commands"),
	}

	// Dynamic commands
	for _, k := range cmds.store.Keys() {
		v, _ := cmds.store.Get(k)
		cmds.cmds[k] = &command{func(_ string) string { return v }, false}
	}

	// Pleb commands
	cmds.cmds["help"] = &command{cmdHelp, false}
	cmds.cmds["uptime"] = &command{func(_ string) string { return getUptime(CHANNEL) }, false}
	cmds.cmds["game"] = &command{func(_ string) string { return getGame(CHANNEL, true) }, false}
	cmds.cmds["quote"] = &command{func(q string) string { return quotes.Random(q) }, false}

	// Mod commands
	cmds.cmds["addquote"] = &command{cmdAddQuote, true}
	cmds.cmds["addcommand"] = &command{cmdAddCommand, true}
	cmds.cmds["removecommand"] = &command{cmdRemoveCommand, true}

	// Aliases
	cmds.Alias("halp", "help")
	cmds.Alias("add", "addcommand")
	cmds.Alias("addcom", "addcommand")
	cmds.Alias("remove", "removecommand")
	cmds.Alias("removecom", "removecommand")
	cmds.Alias("del", "removecommand")
	cmds.Alias("delcom", "removecommand")
	cmds.Alias("delcommand", "removecommand")
}

func handle(out chan string, m *message) {
	switch m.Command {
	case "PING":
		out <- fmt.Sprintf("PONG :%s\r\n", strings.Join(m.Args, " "))
	case "PRIVMSG":
		msg := strings.ToLower(m.Args[1])
		for _, prefix := range cmdPrefixes {
			if strings.HasPrefix(msg, prefix) {
				p := split(m.Args[1][len(prefix):], 2)
				if c := cmds.Get(p[0]); c != nil && (!c.modOnly || m.Mod) {
					if response := c.fn(p[1]); response != "" {
						out <- fmt.Sprintf("PRIVMSG %s :\u200B%s\r\n", m.Args[0], response)
					}
				}
				return
			}
		}
	}
}

func cmdHelp(_ string) string {
	cmds.RLock()
	defer cmds.RUnlock()
	names := []string{}
	for k, _ := range cmds.cmds {
		names = append(names, k)
	}
	sort.Strings(names)
	return "Available Commands: " + strings.Join(names, " ")
}

func cmdAddQuote(quote string) string {
	g := getGame(CHANNEL, false)
	t := time.Now().Round(time.Second)
	if l, err := time.LoadLocation("America/Vancouver"); err == nil {
		t = t.In(l)
	}
	quotes.Append(fmt.Sprintf("%s [Playing %s - %s]", quote, g, t.Format(time.RFC822)))
	return ""
}

func cmdAddCommand(data string) string {
	cmds.Lock()
	defer cmds.Unlock()
	v := split(data, 2)
	trigger, msg := strings.TrimPrefix(v[0], "!"), v[1]
	cmds.store.Add(trigger, msg)
	cmds.cmds[trigger] = &command{func(_ string) string { return msg }, false}
	return ""
}

func cmdRemoveCommand(data string) string {
	cmds.Lock()
	defer cmds.Unlock()
	v := split(data, 2)
	trigger := strings.TrimPrefix(v[0], "!")
	cmds.store.Remove(trigger)
	delete(cmds.cmds, trigger)
	return ""
}

func split(s string, p int) []string {
	r := strings.SplitN(s, " ", p)
	for len(r) < p {
		r = append(r, "")
	}
	for i := 0; i < p-1; i++ {
		r[i] = strings.ToLower(r[i])
	}
	return r
}
