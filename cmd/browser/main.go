package main // import "honnef.co/go/uzbl/browser"

import (
	"honnef.co/go/uzbl"
	"honnef.co/go/uzbl/follow"
	"honnef.co/go/uzbl/progress"
	"honnef.co/go/uzbl/scroll"
)

func main() {
	u := &uzbl.Uzbl{}
	u.Register(
		&progress.Bar{},
		&scroll.Indicator{},
		&follow.Follow{},
	)
	u.Start()
}
