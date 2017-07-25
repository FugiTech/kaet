package main

import (
	"bytes"
	"log"
	"strings"
)

type message struct {
	DisplayName string
	Mod         bool
	Sub         bool
	Command     string
	RoomID      string
	UserID      string
	Args        []string
}

func parse(line []byte) *message {
	if line[len(line)-2] != '\r' || line[len(line)-1] != '\n' {
		log.Print("INVALID LINE")
		return nil
	}

	line = line[:len(line)-2]
	m := &message{}
	i := 0
	b := bytes.NewBuffer(nil)

	// parse tags
	if line[i] == '@' {
		k, v := "", ""
		for i++; line[i] != ' '; i++ {
			switch line[i] {
			case ';':
				v = b.String()
				b.Reset()
				handleTag(m, k, v)
			case '=':
				k = b.String()
				b.Reset()
			case '\\':
				i++
				switch line[i] {
				case ':':
					b.WriteByte(';')
				case 's':
					b.WriteByte(' ')
				case '\\':
					b.WriteByte('\\')
				case 'r':
					b.WriteByte('\r')
				case 'n':
					b.WriteByte('\n')
				}
			default:
				b.WriteByte(line[i])
			}
		}
		v = b.String()
		b.Reset()
		handleTag(m, k, v)
		i++
	}

	// parse prefix
	if line[i] == ':' {
		for {
			i++
			if line[i] == ' ' {
				break
			}
			b.WriteByte(line[i])
		}
		// prefix := b.String()
		b.Reset()
		i++
	}

	// parse command
	for ; line[i] != ' '; i++ {
		b.WriteByte(line[i])
	}
	m.Command = strings.ToUpper(b.String())
	b.Reset()
	i++

	// parse args
	for i < len(line) && line[i] != ':' {
		for ; i < len(line) && line[i] != ' '; i++ {
			b.WriteByte(line[i])
		}
		m.Args = append(m.Args, b.String())
		b.Reset()
		i++
	}
	if i < len(line) && line[i] == ':' {
		i++
		b.Write(line[i:])
		m.Args = append(m.Args, b.String())
	}

	return m
}

func handleTag(m *message, k string, v string) {
	if k == "" {
		k = v
		v = ""
	}
	switch k {
	case "display-name":
		m.DisplayName = v
	case "user-id":
		m.UserID = v
	case "room-id":
		m.RoomID = v
	case "mod":
		m.Mod = v == "1"
	case "subscriber":
		m.Sub = v == "1"
	}
}
