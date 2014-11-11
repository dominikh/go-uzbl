package follow // import "honnef.co/go/uzbl/follow"

import (
	"fmt"
	"strings"

	"honnef.co/go/uzbl"
)

type Follow struct {
	keymap *uzbl.Keymap
}

func (f *Follow) Init(u *uzbl.Uzbl) {
	f.keymap = &uzbl.Keymap{
		Prompt:   "Follow:",
		OnEscape: f.evEscape,
	}
	f.keymap.Bind("<*>", f.evKeypress)

	u.AddHandler("LOAD_COMMIT", f.evLoadCommit)
	u.AddHandler("FOLLOW", f.evFollow)
	u.AddHandler("FOLLOWING", f.evFollowing)
}

func (f *Follow) evLoadCommit(ev *uzbl.Event) error {
	// FIXME relative path
	// TODO see if we can use a data uri for this
	ev.Uzbl.Send("js page file /home/dominikh/.config/uzbl/hints.js")
	return nil
}

func (f *Follow) evEscape(ev *uzbl.Event) {
	ev.Uzbl.Send("js page string uzbl.LinkHints.deactivateMode()")
}

func (f *Follow) evKeypress(ev *uzbl.Event, input uzbl.Keys) error {
	ev.Uzbl.Send(fmt.Sprintf("event FOLLOWING @< uzbl.LinkHints.Blegh('%s') >@", input))
	return nil
}

func (f *Follow) evFollow(ev *uzbl.Event) error {
	ev.Uzbl.IM.SetKeymap(f.keymap)
	ev.Uzbl.Send("event FOLLOWING @< uzbl.LinkHints.Blegh('') >@")
	return nil
}

func (f *Follow) evFollowing(ev *uzbl.Event) error {
	parts := strings.SplitN(ev.Detail, " ", 2)
	if len(parts) == 0 {
		return nil
	}
	if parts[0] == "select" || parts[0] == "click" {
		ev.Uzbl.Send("js page string uzbl.LinkHints.deactivateMode()")
		ev.Uzbl.IM.SetGlobalKeymap()
	}

	if parts[0] == "select" {
		ev.Uzbl.Send("event INSERT_MODE")
	}
	return nil
}
