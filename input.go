package uzbl

import (
	"fmt"
	"strings"
)

type Keys []Key

func (keys Keys) Display() string {
	ss := make([]string, len(keys))
	for i, k := range keys {
		ss[i] = k.String()
	}
	return strings.Join(ss, " ")
}

func (keys Keys) String() string {
	ss := make([]string, len(keys))
	for i, k := range keys {
		ss[i] = k.String()
	}
	return strings.Join(ss, "")
}

const (
	ctrl = 1 << iota
	shift
	mod1
	mod2
	mod3
	mod4
	mod5
	mod6
)

var modNames = []struct {
	mod  int
	name string
}{
	{ctrl, "C"},
	{shift, "S"},
	{mod1, "1"},
	{mod2, "2"},
	{mod3, "3"},
	{mod4, "4"},
	{mod5, "5"},
	{mod6, "6"},
}

type Key struct {
	key string
	mod int
}

func (key Key) String() string {
	var mods string
	for _, mod := range modNames {
		if (key.mod & mod.mod) > 0 {
			mods += mod.name + "-"
		}
	}
	return mods + key.key
}

func parseMod(s string) int {
	mods := 0
	for _, mod := range strings.Split(s, "|") {
		switch mod {
		case "Shift":
			mods |= shift
		case "Ctrl":
			mods |= ctrl
		case "Mod1":
			mods |= mod1
		case "Mod2":
			mods |= mod2
		case "Mod3":
			mods |= mod3
		case "Mod4":
			mods |= mod4
		case "Mod5":
			mods |= mod5
		case "Mod6":
			mods |= mod6
		}
	}
	return mods
}

func parseBind(s string) *keyBind {
	bind := &keyBind{}
	var keys Keys
	for _, k := range strings.Split(s, " ") {
		// TODO handle invalid input
		key := k
		mod := 0
		if len(k) > 1 {
			parts := strings.Split(k, "-")
			key = parts[len(parts)-1]
			mods := parts[0 : len(parts)-1]

			for _, m := range mods {
				switch m {
				case "C":
					mod |= ctrl
				case "1":
					mod |= mod1
				case "2":
					mod |= mod2
				case "3":
					mod |= mod3
				case "4":
					mod |= mod4
				case "5":
					mod |= mod5
				case "6":
					mod |= mod6
				case "S":
					mod |= shift
				}
			}
		}

		if key == "<space>" {
			key = " "
		}
		keys = append(keys, Key{key: key, mod: mod})
	}

	if keys[len(keys)-1].key == "<*>" {
		bind.incremental = true
	}

	bind.bind = keys
	return bind
}

type keyBind struct {
	bind        Keys
	fn          func(ev *Event, input Keys) error
	incremental bool
	// TODO support the ! modifier?
}

func (b *keyBind) matches(input Keys) bool {
	l := len(b.bind)
	if b.incremental {
		l -= 1
	}

	if len(input) < l {
		return false
	}

	if len(input) > len(b.bind) && !b.incremental {
		return false
	}

	for i := 0; i < l; i++ {
		if b.bind[i].key != input[i].key || b.bind[i].mod != input[i].mod {
			return false
		}
	}

	return true
}

func NewInputManager(u *Uzbl) *InputManager {
	im := &InputManager{uzbl: u, globalKeymap: &Keymap{DisplaySpaces: true}}
	im.activeKeymap = im.globalKeymap
	u.EM.AddHandler("KEY_PRESS", im.evKeyPress)
	u.EM.AddHandler("BIND", im.evBind)
	u.EM.AddHandler("INSERT_MODE", im.evInsertMode)
	u.EM.AddHandler("ESCAPE", im.evEscape)
	u.EM.AddHandler("INSTANCE_START", im.evInstanceStart)
	u.EM.AddHandler("LOAD_START", im.evLoadStart)
	u.EM.AddHandler("FOCUS_ELEMENT", im.evFocusElement)
	u.EM.AddHandler("ROOT_ACTIVE", im.evRootActive)
	return im
}

const (
	commandMode = 0
	insertMode  = 1
)

type InputManager struct {
	uzbl         *Uzbl
	globalKeymap *Keymap
	activeKeymap *Keymap
	input        Keys
	mode         int
}

func (im *InputManager) evRootActive(*Event) error {
	// FIXME there seems to be a bug in uzbl that triggers a
	// FOCUS_ELEMENT right after the first ROOT_ACTIVE.
	im.mode = commandMode
	im.uzbl.Send("set forward_keys = 0")
	im.setModeIndicator()
	return nil
}

func (im *InputManager) evFocusElement(ev *Event) error {
	fmt.Println(ev.Detail)
	el, err := parseString(ev.Detail)
	if err != nil {
		return err
	}

	// FIXME this will focus the google search when the page loads,
	// which is annoying behaviour.

	switch el {
	case "INPUT", "TEXTAREA", "SELECT":
		return im.evInsertMode(ev)
	}
	return nil
}

func (im *InputManager) evLoadStart(*Event) error {
	im.mode = commandMode
	im.uzbl.Send("set forward_keys = 0")
	im.setModeIndicator()
	return nil
}

func (im *InputManager) evKeyPress(ev *Event) error {
	parts := ev.ParseDetail(-1)
	mods, key := parseMod(parts[0]), parts[1]
	if len(key) == 1 {
		mods &^= shift
	}

	if key == "Escape" {
		im.uzbl.Send("event ESCAPE")
		return nil
	}

	if im.mode == insertMode {
		return nil
	}

	if key == "BackSpace" {
		if len(im.input) == 0 {
			return nil
		}
		im.input = im.input[0 : len(im.input)-1]
		im.setKeycmd()
	} else {
		// TODO way to not print spaces between characters, and not to use
		// <space>, so we can type urls etc

		if key == "space" {
			key = " "
		}

		if len(key) > 1 {
			key = "<" + key + ">"
		}
		im.input = append(im.input, Key{key: key, mod: mods})
		im.setKeycmd()
	}

	// FIXME incremental binds + Return

	bind, ok := im.activeKeymap.findBind(im.input)
	if !ok {
		return nil
	}

	if key == "<Return>" && bind.incremental {
		im.ClearInput()
		return nil
	}

	var err error
	if bind.incremental {
		err = bind.fn(ev, im.input[len(bind.bind)-1:])
	} else {
		err = bind.fn(ev, nil)
	}

	if !bind.incremental || key == "<Return>" {
		im.ClearInput()
	}

	return err
}

func (im *InputManager) setKeycmd() {
	im.setPrompt()
	var chain string
	if im.activeKeymap.DisplaySpaces {
		chain = im.input.Display()
	} else {
		chain = im.input.String()
	}
	chain = strings.Replace(chain, " ", "\\ ", -1)
	im.uzbl.Send(fmt.Sprintf("set keycmd = %s", chain))
}

func (im *InputManager) setPrompt() {
	if im.activeKeymap.Prompt == "" {
		im.uzbl.Send(fmt.Sprintf("set keycmd_prompt = "))
		return
	}
	prompt := im.activeKeymap.Prompt + `\ \ `
	im.uzbl.Send(fmt.Sprintf("set keycmd_prompt = %s", prompt))
}

func (im *InputManager) setModeIndicator() {
	name := ""
	switch im.mode {
	case commandMode:
		name = "Cmd"
	case insertMode:
		name = "Ins"
	default:
		name = "Error!"
	}
	im.uzbl.Send(fmt.Sprintf("set mode_indicator = %s", name))
}

func (im *InputManager) evBind(ev *Event) error {
	args := ev.ParseDetail(3)
	im.globalKeymap.Bind(args[0], CommandFn(args[1])) // TODO repeat
	return nil
}

func (im *InputManager) evInsertMode(ev *Event) error {
	im.mode = insertMode
	im.uzbl.Send("set forward_keys = 1")
	im.setModeIndicator()
	return nil
}

func (im *InputManager) evEscape(ev *Event) error {
	if im.activeKeymap.OnEscape != nil {
		im.activeKeymap.OnEscape()
	}
	// TODO move this into an OnEscape, too?
	im.SetGlobalKeymap()
	im.mode = commandMode
	im.uzbl.Send("set forward_keys = 0")
	im.setModeIndicator()
	return nil
}

func (im *InputManager) evInstanceStart(ev *Event) error {
	im.setModeIndicator()
	return nil
}

func (im *InputManager) SetKeymap(k *Keymap) {
	im.activeKeymap = k
	im.ClearInput()
	im.setPrompt()
}

func (im *InputManager) SetGlobalKeymap() {
	im.SetKeymap(im.globalKeymap)
}

func (im *InputManager) ClearInput() {
	im.input = nil
	im.setKeycmd()
}

type Keymap struct {
	binds         []*keyBind
	Prompt        string
	DisplaySpaces bool
	OnEscape      func()
}

func (k *Keymap) Bind(s string, fn func(ev *Event, input Keys) error) {
	bind := parseBind(s)
	bind.fn = fn
	k.binds = append(k.binds, bind)
}

func (k *Keymap) findBind(input Keys) (*keyBind, bool) {
	// TODO if we ever end up with enough binds to make this slow,
	// consider a tree-based implementation.
	for _, b := range k.binds {
		if b.matches(input) {
			return b, true
		}
	}
	return nil, false
}
