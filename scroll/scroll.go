package scroll

import (
	"fmt"
	"strconv"

	"honnef.co/go/uzbl"
)

type ScrollIndicator struct{}

func New(u *uzbl.Uzbl) *ScrollIndicator {
	s := &ScrollIndicator{}
	u.EM.AddHandler("SCROLL_VERT", s.evScrollVert)
	return s
}

func (s *ScrollIndicator) evScrollVert(ev *uzbl.Event) error {
	numbers, err := parseFloats(ev.ParseDetail(4)...)
	if err != nil {
		return err
	}
	cur, _, max, size := numbers[0], numbers[1], numbers[2], numbers[3]
	out := "--"

	if max == 0 {
		// TODO right now we get [0, 0, 0, 0] when there are no scroll
		// bars. is that correct or a bug?
		out = "All"
	} else if cur == 0 {
		out = "Top"
	} else if cur+size == max {
		out = "Bot"
	} else {
		p := cur / (max - size)
		out = fmt.Sprintf("%.2f%%", float64(int((10000*p)+0.5))/100)
	}

	ev.Uzbl.Send(fmt.Sprintf("set scroll_message = %s", out))
	return nil
}

func parseFloats(ss ...string) ([]float64, error) {
	out := make([]float64, len(ss))
	var err error
	for i, s := range ss {
		out[i], err = strconv.ParseFloat(s, 64)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}
