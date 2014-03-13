package main

import (
	"honnef.co/go/uzbl"
	"honnef.co/go/uzbl/follow"
	"honnef.co/go/uzbl/progress"
	"honnef.co/go/uzbl/scroll"
)

func main() {
	u := &uzbl.Uzbl{}
	u.Register(
		progress.New(u),
		scroll.New(u),
		follow.New(u),
	)
	u.Start()
}
