package follow

import (
	"fmt"
	"strings"

	"honnef.co/go/uzbl"
)

type Follow struct {
	uzbl   *uzbl.Uzbl
	keymap *uzbl.Keymap
}

func New(u *uzbl.Uzbl) *Follow {
	f := &Follow{uzbl: u}

	f.keymap = &uzbl.Keymap{
		Prompt:   "Follow:",
		OnEscape: f.evEscape,
	}
	f.keymap.Bind("<*>", f.evKeypress)

	u.EM.AddHandler("LOAD_COMMIT", f.evLoadCommit)
	u.EM.AddHandler("FOLLOW", f.evFollow)
	u.EM.AddHandler("FOLLOWING", f.evFollowing)
	return f
}

func (f *Follow) evLoadCommit(*uzbl.Event) error {
	// FIXME relative path
	// TODO see if we can use a data uri for this
	f.uzbl.Send("js page file /home/dominikh/.config/uzbl/hints.js")
	return nil
}

func (f *Follow) evEscape() {
	f.uzbl.Send("js page string uzbl.LinkHints.deactivateMode()")
}

func (f *Follow) evKeypress(ev *uzbl.Event, input uzbl.Keys) error {
	f.uzbl.Send(fmt.Sprintf("event FOLLOWING @< uzbl.LinkHints.Blegh('%s') >@", input))
	return nil
}

func (f *Follow) evFollow(ev *uzbl.Event) error {
	f.uzbl.IM.SetKeymap(f.keymap)
	f.uzbl.Send("event FOLLOWING @< uzbl.LinkHints.Blegh('') >@")
	return nil
}

func (f *Follow) evFollowing(ev *uzbl.Event) error {
	parts := strings.SplitN(ev.Detail, " ", 2)
	if len(parts) == 0 {
		return nil
	}
	if parts[0] == "select" || parts[0] == "click" {
		f.uzbl.Send("js page string uzbl.LinkHints.deactivateMode()")
		f.uzbl.IM.SetGlobalKeymap()
	}

	if parts[0] == "select" {
		f.uzbl.Send("event INSERT_MODE")
	}
	return nil
}
