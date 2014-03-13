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
	if n == 0 {
		return nil
	}
	if n == 1 {
		// FIXME should we really just return ev.Detail, without
		// parsing it at all? what if it's a string?
		return []string{ev.Detail}
	}

	var out []string
	pos := 0
	argStart := 0
	hasNonSpace := false
	stringOpen := false
	stringChar := '"'
	escape := false
	last := false
	var advance func(i int) bool
	advance = func(i int) bool {
		if hasNonSpace || last {
			out = append(out, ev.Detail[argStart:i])
			pos++
		}
		stringOpen = false
		hasNonSpace = false
		argStart = i + 1
		if n >= 0 && pos == n-1 {
			last = true
			var i int
			var c rune
			for i, c = range ev.Detail[argStart:] {
				if c != ' ' {
					break
				}
			}
			argStart += i
			advance(len(ev.Detail))
			return true
		}
		return false
	}
	// TODO determine whether strings in \@<> (and similar) cause problems
	for i, c := range ev.Detail {
		switch c {
		case ' ':
			if !stringOpen {
				// we just advanced to the next argument
				if advance(i) {
					goto Done
				}
			}
			escape = false
		case '\'', '"':
			if escape {
				hasNonSpace = true
				escape = false
				continue
			}
			if stringOpen {
				hasNonSpace = true
				if stringChar == c {
					// we just closed a string
					if advance(i) {
						goto Done
					}
				}
			} else {
				// we just opened a string
				if advance(i) {
					goto Done
				}
				stringOpen = true
				stringChar = c
			}
		case '\\':
			escape = true
		default:
			hasNonSpace = true
			escape = false
		}
	}
	advance(len(ev.Detail))
Done:
	return pad(out, n)
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
