package uzbl // import "honnef.co/go/uzbl"

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"honnef.co/go/uzbl/event_manager"
)

type Event struct {
	*event_manager.Event
	Uzbl *Uzbl
}

type Handler func(*Event) error

type KeyHandler struct {
	State int
}

type geom struct {
	X      int
	Y      int
	Width  int
	Height int
}

type Registerable interface {
	Init(*Uzbl)
}

type Uzbl struct {
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	Variables  *VariableStore
	geometry   geom
	em         *event_manager.Manager
	IM         *InputManager
	registered []Registerable
}

func (u *Uzbl) Register(r ...Registerable) {
	u.registered = append(u.registered, r...)
}

type ErrUnknownType struct {
	Type  string
	Value string
}

func (e ErrUnknownType) Error() string {
	return fmt.Sprintf("Unknown variable type '%s'", e.Type)
}

type ErrInvalidValue struct {
	Type  string
	Value string
}

func (e ErrInvalidValue) Error() string {
	return fmt.Sprintf("Invalid variable value '%s' for type '%s'", e.Value, e.Type)
}

func parseString(s string) (string, error) {
	if len(s) < 2 || s[0] != '\'' || s[len(s)-1] != '\'' {
		return "", ErrInvalidValue{"str", s}
	}
	return s[1 : len(s)-1], nil
}

func parseInts(ss ...string) ([]int, error) {
	out := make([]int, len(ss))
	var err error
	for i, s := range ss {
		out[i], err = strconv.Atoi(s)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func (u *Uzbl) AddHandler(ev string, fn Handler) {
	u.em.AddHandler(ev, func(event *event_manager.Event) error {
		return fn(&Event{event, u})
	})
}

func (u *Uzbl) evOnEvent(ev *Event) error {
	parts := ev.ParseDetail(2)
	evName, payload := parts[0], parts[1]
	u.AddHandler(evName, func(*Event) error {
		u.Send(payload)
		return nil
	})
	return nil
}

func (u *Uzbl) evGeometryChanged(ev *Event) error {
	s, err := parseString(ev.Detail)
	if err != nil {
		return err
	}
	parts := strings.Split(s, "+")
	dim := strings.Split(parts[0], "x")
	x := parts[1]
	y := parts[2]
	width := dim[0]
	height := dim[1]

	ints, err := parseInts(x, y, width, height)
	if err != nil {
		return err
	}
	u.geometry.X, u.geometry.Y, u.geometry.Width, u.geometry.Height =
		ints[0], ints[1], ints[2], ints[3]

	return nil
}

func (u *Uzbl) loadConfig() error {
	fmt.Println("Loading config")
	// TODO XDG
	f, err := os.Open("/home/dominikh/.config/uzbl/config")
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(u.stdin, f) // TODO turn this into channel+sendCommands
	return err
}

func (u *Uzbl) Start() {
	cmd := exec.Command("uzbl-core", "-c", "-", "-p", "--uri", "https://google.com")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	cmd.Stderr = os.Stderr

	u.stdin = stdin
	u.stdout = stdout

	u.em = event_manager.New(stdout)
	u.Variables = NewVariableStore()
	u.IM = NewInputManager(u)
	u.AddHandler("VARIABLE_SET", u.Variables.evVariableSet)
	u.AddHandler("GEOMETRY_CHANGED", u.evGeometryChanged)
	u.AddHandler("ON_EVENT", u.evOnEvent)

	// FIXME it's really ugly that the order of this matters

	for _, r := range u.registered {
		r.Init(u)
	}

	go u.em.Listen()
	err = cmd.Start()
	if err != nil {
		panic(err)
	}

	u.loadConfig()

	cmd.Wait()
}

func (u *Uzbl) Send(cmd string) {
	u.stdin.Write([]byte(cmd))
	u.stdin.Write([]byte{'\n'})
}

func (u *Uzbl) CommandFn(cmd string) func(*Event, Keys) error {
	return func(ev *Event, input Keys) error {
		cmd := cmd
		if ok, _ := regexp.MatchString("[^%]%s", cmd); ok {
			cmd = fmt.Sprintf(cmd, input.String())
		}
		u.Send(cmd)
		return nil
	}
}
