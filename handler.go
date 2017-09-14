package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	cmdPrefixes []string
	quotes      *store
	counters    *store
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
	fn        func(string) string
	modOnly   bool
	removable bool
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
	counters = Store("counters")

	cmds = &commands{
		cmds:     map[string]*command{},
		aliases:  map[string]string{},
		rAliases: map[string][]string{},
		store:    Store("commands"),
	}

	// Dynamic commands
	for _, k := range counters.Keys() {
		cmds.cmds[k] = &command{cmdCounter(k), false, false}
	}
	for _, k := range cmds.store.Keys() {
		v, _ := cmds.store.Get(k)
		cmds.cmds[k] = &command{func(_ string) string { return v }, false, true}
	}

	// Pleb commands
	cmds.cmds["help"] = &command{cmdHelp, false, false}
	cmds.cmds["uptime"] = &command{func(_ string) string { return getUptime(CHANNEL) }, false, false}
	cmds.cmds["game"] = &command{func(_ string) string { return getGame(CHANNEL, true) }, false, false}
	cmds.cmds["quote"] = &command{cmdGetQuote, false, false}
	cmds.cmds["sourcecode"] = &command{func(q string) string { return "Contribute to kaet's source code at github.com/Fugiman/kaet VoHiYo" }, false, false}

	// Mod commands
	cmds.cmds["addquote"] = &command{cmdAddQuote, true, false}
	cmds.cmds["addcommand"] = &command{cmdAddCommand, true, false}
	cmds.cmds["removecommand"] = &command{cmdRemoveCommand, true, false}
	cmds.cmds["increment"] = &command{cmdIncrement, true, false}
	cmds.cmds["decrement"] = &command{cmdDecrement, true, false}
	cmds.cmds["reset"] = &command{cmdReset, true, false}

	// Aliases
	cmds.Alias("halp", "help")
	cmds.Alias("add", "addcommand")
	cmds.Alias("addcom", "addcommand")
	cmds.Alias("remove", "removecommand")
	cmds.Alias("removecom", "removecommand")
	cmds.Alias("del", "removecommand")
	cmds.Alias("delcom", "removecommand")
	cmds.Alias("delcommand", "removecommand")
	cmds.Alias("source", "sourcecode")
	cmds.Alias("code", "sourcecode")
	cmds.Alias("inc", "increment")
	cmds.Alias("dec", "decrement")
}

func handle(out chan string, m *message) {
	switch m.Command {
	case "PING":
		out <- fmt.Sprintf("PONG :%s\r\n", strings.Join(m.Args, " "))
	case "RECONNECT":
		os.Exit(69)
	case "PRIVMSG":
		msg := strings.ToLower(m.Args[1])
		for _, prefix := range cmdPrefixes {
			if strings.HasPrefix(msg, prefix) {
				p := split(m.Args[1][len(prefix):], 2)
				isMod := m.Mod || m.UserID != "" && m.RoomID == m.UserID
				if c := cmds.Get(p[0]); c != nil && (!c.modOnly || isMod) {
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
	for k := range cmds.cmds {
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

func cmdRemoveQuote(quoteNum string) string {
	if strings.HasPrefix(quoteNum, "#") {
		quoteNum = quoteNum[1:]
	}
	// cmdAddQuote relies on continuous numbering, so blank the quotes instead of removing them
	if quotes.Blank(quoteNum) {
		return fmt.Sprintf("Removed #%s", quoteNum)
	}
	return ""
}

func cmdGetQuote(query string) string {
	if strings.HasPrefix(query, "#") {
		if quote, found := quotes.Get(query[1:]); found && quote != "" {
			return quote
		}
		return "Not found"
	}
	return quotes.Random(query)
}

func cmdAddCommand(data string) string {
	cmds.Lock()
	defer cmds.Unlock()
	v := split(data, 2)
	trigger, msg := strings.TrimPrefix(v[0], "!"), v[1]
	existingCmd, existingCmdFound := cmds.cmds[trigger]
	if existingCmdFound && !existingCmd.removable {
		return "I'm afraid I can't modify that command"
	}
	cmds.store.Add(trigger, msg)
	cmds.cmds[trigger] = &command{func(_ string) string { return msg }, false, true}
	return ""
}

func cmdRemoveCommand(data string) string {
	cmds.Lock()
	defer cmds.Unlock()
	v := split(data, 2)
	trigger := strings.TrimPrefix(v[0], "!")
	existingCommand, existingCommandFound := cmds.cmds[trigger]
	if existingCommandFound && !existingCommand.removable {
		return "I'm afraid I can't remove that command"
	}
	cmds.store.Remove(trigger)
	delete(cmds.cmds, trigger)
	return ""
}

func cmdIncrement(data string) string {
	cmds.Lock()
	defer cmds.Unlock()

	count := 0
	if v, ok := counters.Get(data); ok {
		count, _ = strconv.Atoi(v)
	}
	count++

	counters.Add(data, strconv.Itoa(count))
	cmds.cmds[data] = &command{cmdCounter(data), false, false}
	return fmt.Sprintf("%d", count)
}

func cmdDecrement(data string) string {
	cmds.Lock()
	defer cmds.Unlock()

	count := 0
	if v, ok := counters.Get(data); ok {
		count, _ = strconv.Atoi(v)
	}
	count--

	counters.Add(data, strconv.Itoa(count))
	cmds.cmds[data] = &command{cmdCounter(data), false, false}
	return fmt.Sprintf("%d", count)
}

func cmdReset(data string) string {
	cmds.Lock()
	defer cmds.Unlock()

	count := 0
	counters.Remove(data)
	return fmt.Sprintf("%d", count)
}

func cmdCounter(k string) func(string) string {
	return func(_ string) string {
		v, _ := counters.Get(k)
		if v == "" {
			v = "0"
		}
		return v
	}
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
