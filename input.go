package uzbl

import (
	"fmt"
	"strings"
)

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

func parseBind(s string) []Key {
	var keys []Key
	for _, k := range strings.Split(s, " ") {
		// TODO handle invalid input
		parts := strings.Split(k, "-")
		key := parts[len(parts)-1]
		mods := parts[0 : len(parts)-1]
		if len(parts) == 1 {
			mods = nil
		}

		mod := 0
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
		if len(key) >= 3 && key[0] == '<' && key[len(key)-1] == '>' {
			// parse function key
			key = key[1 : len(key)-1]
		}
		keys = append(keys, Key{key: key, mod: mod})
	}

	return keys
}

type keyBind struct {
	bind        []Key
	confirm     bool
	incremental bool
	fn          func(ev *Event, input []Key) error // TODO which arguments should we give the callback?
	// TODO support the ! modifier?
}

func (b *keyBind) matches(input []Key) bool {
	if !b.incremental && len(input) != len(b.bind) {
		return false
	}
	l := len(input)
	if len(b.bind) < l {
		l = len(b.bind)
	}
	for i := 0; i < l; i++ {
		if b.bind[i].key != input[i].key || b.bind[i].mod != input[i].mod {
			return false
		}
	}
	return true
}

func NewInputManager(u *Uzbl) *InputManager {
	im := &InputManager{uzbl: u}
	u.EM.AddHandler("KEY_PRESS", im.EvKeyPress)
	u.EM.AddHandler("BIND", im.EvBind)
	u.EM.AddHandler("INSERT_MODE", im.EvInsertMode)
	u.EM.AddHandler("ESCAPE", im.EvEscape)
	u.EM.AddHandler("INSTANCE_START", im.EvInstanceStart)
	return im
}

const (
	commandMode = 0
	insertMode  = 1
)

type InputManager struct {
	// TODO support insert mode
	// TODO support : mode
	uzbl   *Uzbl
	binds  []*keyBind
	input  []Key
	prompt string
	mode   int
}

func (im *InputManager) Bind(s string, fn func(ev *Event, input []Key) error) {
	bind := &keyBind{bind: parseBind(s), fn: fn}
	im.binds = append(im.binds, bind)
}

func (im *InputManager) EvKeyPress(ev *Event) error {
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
		return nil
	}

	if key == "Return" && im.mode == commandMode {
		bind, ok := im.findBind(im.input)
		if !ok {
			return nil
		}
		err := bind.fn(ev, im.input)
		im.input = nil
		return err
	}
	// TODO way to not print spaces between characters, and not to use
	// <space>, so we can type urls etc

	if len(key) > 1 {
		key = "<" + key + ">"
	}

	im.input = append(im.input, Key{key: key, mod: mods})
	im.setKeycmd()

	bind, ok := im.findBind(im.input)
	if !ok {
		return nil
	}
	if bind.confirm && bind.incremental {
		return nil
	}

	err := bind.fn(ev, im.input)
	im.input = nil
	im.setKeycmd()

	return err
}

func (im *InputManager) setKeycmd() {
	im.uzbl.Send(fmt.Sprintf("set keycmd_prompt = %s", im.prompt))
	im.uzbl.Send(fmt.Sprintf("set keycmd = %s", keysToString(im.input)))
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

func (im *InputManager) EvBind(ev *Event) error {
	args := ev.ParseDetail(3)
	im.Bind(args[0], CommandFn(args[1])) // TODO repeat,confirm
	return nil
}

func (im *InputManager) EvInsertMode(ev *Event) error {
	im.mode = insertMode
	im.uzbl.Send("set forward_keys = 1")
	im.setModeIndicator()
	return nil
}

func (im *InputManager) EvEscape(ev *Event) error {
	if im.mode == commandMode {
		im.input = nil
		im.setKeycmd()
		return nil
	}
	im.mode = commandMode
	im.uzbl.Send("set forward_keys = 0")
	im.setModeIndicator()
	return nil
}

func (im *InputManager) EvInstanceStart(ev *Event) error {
	im.setModeIndicator()
	return nil
}

func (im *InputManager) findBind(input []Key) (*keyBind, bool) {
	// TODO if we ever end up with enough binds to make this slow,
	// consider a tree-based implementation.
	for _, b := range im.binds {
		if b.matches(input) {
			return b, true
		}
	}
	return nil, false
}

func keysToString(keys []Key) string {
	ss := make([]string, len(keys))
	for i, k := range keys {
		ss[i] = k.String()
	}
	return strings.Join(ss, " ")
}
