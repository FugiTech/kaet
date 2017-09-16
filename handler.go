package main

import (
	"fmt"
	"math"
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
	balances    *store
	cmds        *commands
)

type commands struct {
	sync.RWMutex
	cmds        map[string]*command
	aliases     map[string]string
	rAliases    map[string][]string
	store       *store
	currentBet  map[string]map[string]int
	bettingOpen bool
}

type command struct {
	fn        func(*User, string) string
	modOnly   bool
	removable bool
}

type User struct {
	ID   string
	Name string
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
	balances = Store("balances")

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
		cmds.cmds[k] = &command{func(_ *User, _ string) string { return v }, false, true}
	}

	// Pleb commands
	cmds.cmds["help"] = &command{cmdHelp, false, false}
	cmds.cmds["uptime"] = &command{func(_ *User, _ string) string { return getUptime(CHANNEL) }, false, false}
	cmds.cmds["game"] = &command{func(_ *User, _ string) string { return getGame(CHANNEL, true) }, false, false}
	cmds.cmds["quote"] = &command{cmdGetQuote, false, false}
	cmds.cmds["sourcecode"] = &command{func(_ *User, q string) string {
		return "Contribute to kaet's source code at github.com/Fugiman/kaet VoHiYo"
	}, false, false}
	cmds.cmds["bet"] = &command{cmdBet, false, false}
	cmds.cmds[CURRENCY_NAME] = &command{cmdBalance, false, false}

	// Mod commands
	cmds.cmds["addquote"] = &command{cmdAddQuote, true, false}
	cmds.cmds["addcommand"] = &command{cmdAddCommand, true, false}
	cmds.cmds["removecommand"] = &command{cmdRemoveCommand, true, false}
	cmds.cmds["increment"] = &command{cmdIncrement, true, false}
	cmds.cmds["decrement"] = &command{cmdDecrement, true, false}
	cmds.cmds["reset"] = &command{cmdReset, true, false}
	cmds.cmds["open"] = &command{cmdOpen, true, false}
	cmds.cmds["close"] = &command{cmdClose, true, false}
	cmds.cmds["payout"] = &command{cmdPayout, true, false}

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
					u := &User{m.UserID, m.DisplayName}
					if response := c.fn(u, p[1]); response != "" {
						out <- fmt.Sprintf("PRIVMSG %s :\u200B%s\r\n", m.Args[0], response)
					}
				}
				return
			}
		}
	}
}

func cmdHelp(_ *User, _ string) string {
	cmds.RLock()
	defer cmds.RUnlock()
	names := []string{}
	for k := range cmds.cmds {
		names = append(names, k)
	}
	sort.Strings(names)
	return "Available Commands: " + strings.Join(names, " ")
}

func cmdAddQuote(_ *User, quote string) string {
	g := getGame(CHANNEL, false)
	t := time.Now().Round(time.Second)
	if l, err := time.LoadLocation("America/Vancouver"); err == nil {
		t = t.In(l)
	}
	quotes.Append(fmt.Sprintf("%s [Playing %s - %s]", quote, g, t.Format(time.RFC822)))
	return ""
}

func cmdRemoveQuote(_ *User, quoteNum string) string {
	if strings.HasPrefix(quoteNum, "#") {
		quoteNum = quoteNum[1:]
	}
	// cmdAddQuote relies on continuous numbering, so blank the quotes instead of removing them
	if quotes.Blank(quoteNum) {
		return fmt.Sprintf("Removed #%s", quoteNum)
	}
	return ""
}

func cmdGetQuote(_ *User, query string) string {
	if strings.HasPrefix(query, "#") {
		if quote, found := quotes.Get(query[1:]); found && quote != "" {
			return quote
		}
		return "Not found"
	}
	return quotes.Random(query)
}

func cmdAddCommand(_ *User, data string) string {
	cmds.Lock()
	defer cmds.Unlock()
	v := split(data, 2)
	trigger, msg := strings.TrimPrefix(v[0], "!"), v[1]
	existingCmd, existingCmdFound := cmds.cmds[trigger]
	if existingCmdFound && !existingCmd.removable {
		return "I'm afraid I can't modify that command"
	}
	cmds.store.Add(trigger, msg)
	cmds.cmds[trigger] = &command{func(_ *User, _ string) string { return msg }, false, true}
	return ""
}

func cmdRemoveCommand(_ *User, data string) string {
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

func cmdIncrement(_ *User, data string) string {
	cmds.Lock()
	defer cmds.Unlock()

	data = strings.Replace(strings.ToLower(data), " ", "-", -1)

	count := 0
	if v, ok := counters.Get(data); ok {
		count, _ = strconv.Atoi(v)
	} else if _, ok := cmds.cmds[data]; ok {
		return fmt.Sprintf("Can't use %q as a counter, it's already a command!", data)
	}
	count++

	counters.Add(data, strconv.Itoa(count))
	cmds.cmds[data] = &command{cmdCounter(data), false, false}
	return fmt.Sprintf("%d", count)
}

func cmdDecrement(_ *User, data string) string {
	cmds.Lock()
	defer cmds.Unlock()

	data = strings.Replace(strings.ToLower(data), " ", "-", -1)

	count := 0
	if v, ok := counters.Get(data); ok {
		count, _ = strconv.Atoi(v)
	} else if _, ok := cmds.cmds[data]; ok {
		return fmt.Sprintf("Can't use %q as a counter, it's already a command!", data)
	}
	count--

	counters.Add(data, strconv.Itoa(count))
	cmds.cmds[data] = &command{cmdCounter(data), false, false}
	return fmt.Sprintf("%d", count)
}

func cmdReset(_ *User, data string) string {
	cmds.Lock()
	defer cmds.Unlock()

	data = strings.Replace(strings.ToLower(data), " ", "-", -1)

	if _, ok := counters.Get(data); !ok {
		return "That counter doesn't exist"
	}

	counters.Remove(data)
	delete(cmds.cmds, data)
	return "Removed counter"
}

func cmdCounter(k string) func(*User, string) string {
	return func(_ *User, _ string) string {
		v, _ := counters.Get(k)
		if v == "" {
			v = "0"
		}
		return v
	}
}

func cmdOpen(_ *User, data string) string {
	cmds.Lock()
	defer cmds.Unlock()

	if cmds.currentBet != nil {
		return "A bet is already ongoing"
	}

	idx := strings.Index(data, "? ")
	if idx < 0 {
		return "Invalid !open. Make sure the reason ends in a ?"
	}
	reason, data := data[:idx+1], data[idx+2:]
	choices := []string{}
	for _, v := range strings.Split(data, " ") {
		v = strings.ToLower(strings.TrimSpace(v))
		if v != "" {
			choices = append(choices, v)
		}
	}
	if len(choices) < 2 {
		return "Invalid !open. Must have 2+ choices"
	}

	cmds.currentBet = map[string]map[string]int{}
	for _, v := range choices {
		cmds.currentBet[v] = map[string]int{}
	}
	cmds.bettingOpen = true

	return fmt.Sprintf("Betting is now open! %s Choices are: \"%s\". Use !bet <choice> <amount> to join!", reason, strings.Join(choices[:len(choices)-1], `", "`)+`", or "`+choices[len(choices)-1])
}

func cmdClose(_ *User, _ string) string {
	cmds.Lock()
	defer cmds.Unlock()

	if cmds.currentBet == nil {
		return "No bet is ongoing right now"
	}
	if !cmds.bettingOpen {
		return ""
	}

	cmds.bettingOpen = false
	return "Betting is now closed! Good luck to all the entrants!"
}

func cmdPayout(_ *User, data string) string {
	cmds.Lock()
	defer cmds.Unlock()

	if cmds.currentBet == nil {
		return "No bet is ongoing right now"
	}
	if cmds.bettingOpen {
		return "Betting isn't closed, sure hope you didn't forget to do that..."
	}

	choices := []string{}
	for k := range cmds.currentBet {
		choices = append(choices, k)
	}

	winners, ok := cmds.currentBet[strings.ToLower(data)]
	if !ok {
		return "Invalid winning choice. Valid choices: " + strings.Join(choices[:len(choices)-1], `", "`) + `", or "` + choices[len(choices)-1]
	}

	payout := 0
	for _, m := range cmds.currentBet {
		for _, amount := range m {
			payout += amount
		}
	}

	winnerTotal := 0
	for _, amount := range winners {
		winnerTotal += amount
	}

	for user, amount := range winners {
		b, _ := balances.Get(user)
		balance, _ := strconv.Atoi(b)
		if balance <= 0 {
			balance = 1000
		}

		earnings := int(math.Ceil((float64(amount) / float64(winnerTotal)) * float64(payout)))
		balances.Add(user, strconv.Itoa(balance+earnings))
	}

	cmds.currentBet = nil
	return fmt.Sprintf("Congrats and condolences: %d %s were paid out to %d winners! ", payout, CURRENCY_NAME, len(winners))
}

func cmdBet(u *User, data string) string {
	cmds.Lock()
	defer cmds.Unlock()

	if cmds.currentBet == nil {
		return "No bet is ongoing right now"
	}
	if !cmds.bettingOpen {
		return "Betting already closed, sorry!"
	}

	for choice, m := range cmds.currentBet {
		for uid, amount := range m {
			if uid == u.ID {
				return fmt.Sprintf("%s: You already bet %d %s on %q!", u.Name, amount, CURRENCY_NAME, choice)
			}
		}
	}

	v := split(data, 2)
	choice := v[0]
	amount, err := strconv.Atoi(v[1])
	if err != nil {
		return u.Name + ": Invalid amount, make sure it's a number without commas or decimals"
	}

	m, ok := cmds.currentBet[choice]
	if !ok {
		return u.Name + ": Invalid choice, double check the list of options!"
	}

	b, _ := balances.Get(u.ID)
	balance, _ := strconv.Atoi(b)
	if balance <= 0 {
		balance = 1000
	}

	if amount > balance {
		return fmt.Sprintf("%s: You don't have enough %s to bet that much! Limit yourself to %d %s", u.Name, CURRENCY_NAME, balance, CURRENCY_NAME)
	}

	balance -= amount
	m[u.ID] = amount
	balances.Add(u.ID, strconv.Itoa(balance))

	return fmt.Sprintf("%s: You bet %d %s on %q and have %d %s remaining", u.Name, amount, CURRENCY_NAME, choice, balance, CURRENCY_NAME)
}

func cmdBalance(u *User, data string) string {
	cmds.Lock()
	defer cmds.Unlock()

	b, _ := balances.Get(u.ID)
	balance, _ := strconv.Atoi(b)
	if balance <= 0 {
		balance = 1000
	}

	return fmt.Sprintf("%s has %d %s!", u.Name, balance, CURRENCY_NAME)
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
