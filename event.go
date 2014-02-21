package uzbl

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"strings"
)

type handlerMap map[string][]Handler

type Event struct {
	Uzbl   *Uzbl
	Name   string
	Detail string
}

func (ev *Event) ParseDetail(n int) []string {
	if n >= 0 && n <= 1 {
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
					return out
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
						return out
					}
				}
			} else {
				// we just opened a string
				if advance(i) {
					return out
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
	return out
}

type EventManager struct {
	uzbl     *Uzbl
	handlers handlerMap
}

func NewEventManager(uzbl *Uzbl) *EventManager {
	em := &EventManager{uzbl: uzbl, handlers: make(handlerMap)}
	em.AddHandler("ON_EVENT", em.evOnEvent)
	return em
}

func (em *EventManager) AddHandler(ev string, fn Handler) {
	em.handlers[ev] = append(em.handlers[ev], fn)
}

func (em *EventManager) Connect(stdout io.Reader) error {
	r := bufio.NewReader(stdout)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return err
		}
		line = line[:len(line)-1]
		log.Println(line)
		if !strings.HasPrefix(line, "EVENT [") {
			continue
		}
		if len(line) < len("EVENT [] X") {
			// not a valid event
			continue
		}
		idx := strings.Index(line, "]")
		if idx < 0 {
			// not a valid event
			continue
		}
		line = line[idx+2:]
		idx = strings.Index(line, " ")
		if idx < 0 {
			// not a valid event
			continue
		}
		ev := line[:idx]
		detail := line[idx+1:]
		event := &Event{em.uzbl, ev, detail}

		for _, fn := range em.handlers[ev] {
			err := fn(event)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func (em *EventManager) evOnEvent(ev *Event) error {
	parts := ev.ParseDetail(2)
	evName, payload := parts[0], parts[1]
	em.AddHandler(evName, func(*Event) error {
		fmt.Println(payload)
		em.uzbl.Send(payload)
		return nil
	})
	return nil
}
