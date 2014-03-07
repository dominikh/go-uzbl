package adblock

import (
	"io"
	"strings"
)

func parseHide(in string) (*Rule, string) {
	// TODO support exception rules, #@#
	if strings.Index(in, "#@#") > -1 {
		return nil, ""
	}
	h := &Rule{Hide: true}
	parts := strings.SplitN(in, "##", 2)
	if len(parts) == 0 || len(parts) == 1 {
		return nil, ""
		// panic("not a valid element hiding rule")
	}

	if len(parts[0]) > 0 {
		h.Domains = strings.Split(parts[0], ",")
	}

	h.Selector = parts[1]

	return h, ""
}

type Domain []string

func reverse(in []string) []string {
	if len(in) < 2 {
		return in
	}

	for i := 0; i < len(in)/2; i++ {
		in[i], in[len(in)-1-i] = in[len(in)-1-i], in[i]
	}

	return in
}

func NewDomain(s string) Domain {
	out := strings.Split(s, ".")
	return reverse(out)
}

// Check if other matches d. Note that c.a.b matches a.b, but not the
// other way around.
func (d Domain) Match(other Domain) bool {
	if len(d) > len(other) {
		return false
	}
	for i, e := range d {
		if e != other[i] {
			return false
		}
	}
	return true
}

type Domains []Domain

func (ds Domains) Match(d Domain) bool {
	for _, e := range ds {
		if e.Match(d) {
			return true
		}
	}
	return false
}

type Hide struct {
	Domains  Domains
	Exclude  Domains
	Selector string
}

func (hg *Hide) Match(d Domain) bool {
	if len(hg.Exclude) == 0 && len(hg.Domains) == 0 {
		return true
	}

	if hg.Exclude.Match(d) {
		return false
	}

	if len(hg.Domains) == 0 || hg.Domains.Match(d) {
		return true
	}

	return false
}

type Hides []*Hide

func (hs Hides) Find(d Domain) Hides {
	var out Hides
	for _, h := range hs {
		if h.Match(d) {
			out = append(out, h)
		}
	}
	return out
}

func (hs Hides) WriteStylesheet(w io.Writer) (n int, err error) {
	for _, h := range hs {
		w.Write([]byte(h.Selector))
		w.Write([]byte("{display: none !important;}\n"))
	}
	return n, err
}
