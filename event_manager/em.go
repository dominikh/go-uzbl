package event_manager

import (
	"bufio"
	"io"
	"log"
	"strconv"
	"strings"
)

type Handler func(*Event) error

type handlerMap map[string][]Handler

type Event struct {
	Name   string
	Detail string
	Cookie string
	PID    int
}

func (ev *Event) ParseDetail(n int) []string {
	var out []string

	start := 0
	inString := false
	stringDelim := rune(0)
	escape := false

	progress := func(start, i int) {
		arg := ev.Detail[start:i]
		arg = strings.Replace(arg, `\`, "", -1)
		out = append(out, arg)
		inString = false
	}

	for i, c := range ev.Detail {
		switch c {
		case ' ':
			if escape {
				escape = false
				continue
			}

			if inString {
				continue
			}

			if ev.Detail[start:i] == "" {
				// previous one was space or start of string
				start = i + 1
				continue
			}

			progress(start, i)
			start = i + 1
		case '"', '\'':
			if escape {
				escape = false
				continue
			}

			if !inString {
				start = i + 1
				inString = true
				stringDelim = c
				continue
			}

			if stringDelim == c {
				progress(start, i)
				start = i + 1
			}
			escape = false
		case '\\':
			escape = !escape
		default:
			escape = false
		}
	}
	if start < len(ev.Detail) {
		progress(start, len(ev.Detail))
	}
	if n > len(out) {
		return pad(out, n)
	}
	return out[:n]
}

func pad(s []string, n int) []string {
	if len(s) == n {
		return s
	}
	out := make([]string, n)
	copy(out, s)
	return out
}

type Manager struct {
	stdout   io.Reader
	handlers handlerMap
}

func New(stdout io.Reader) *Manager {
	em := &Manager{
		stdout:   stdout,
		handlers: make(handlerMap),
	}
	return em
}

func (em *Manager) AddHandler(ev string, fn Handler) {
	em.handlers[ev] = append(em.handlers[ev], fn)
}

func (em *Manager) Listen() error {
	r := bufio.NewReader(em.stdout)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		line = line[:len(line)-1]
		// log.Println(line)

		em.process(line)
	}
}

func (em *Manager) process(line string) {
	var cookie string
	if strings.HasPrefix(line, "REQUEST-") {
		idx := strings.Index(line, " ")
		if idx < 0 {
			return
		}
		parts := strings.SplitN(line[:idx], "-", 2)
		cookie = parts[1]
	}

	start := strings.Index(line, "[")
	end := strings.Index(line, "]")
	if start < 0 || end < 0 {
		// not a valid event
		return
	}

	pid, err := strconv.Atoi(line[start+1 : end])
	if err != nil {
		// not a valid event
		return
	}

	idx := strings.Index(line, "]")
	if idx < 0 {
		// not a valid event
		return
	}
	if idx+2 > len(line)-1 {
		// not a valid event
		return
	}
	line = line[idx+2:]
	idx = strings.Index(line, " ")
	if idx < 0 {
		// not a valid event
		return
	}
	ev := line[:idx]
	detail := line[idx+1:]
	event := &Event{ev, detail, cookie, pid}

	if cookie != "" {
		ev = "REQUEST-" + ev
	}
	for _, fn := range em.handlers[ev] {
		err := fn(event)
		if err != nil {
			log.Println(err)
		}
	}
}
