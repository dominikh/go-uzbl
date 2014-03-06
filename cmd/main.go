package main

import (
	"honnef.co/go/uzbl"
	"honnef.co/go/uzbl/follow"
	"honnef.co/go/uzbl/progress"
	"honnef.co/go/uzbl/scroll"
)

func main() {
	u := uzbl.NewUzbl()
	progress.New(u) // dangling value, sort of ugly
	scroll.New(u)
	follow.New(u)
	u.Start()
}
