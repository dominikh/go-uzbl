package adblock

import (
	"fmt"
	"io"
	"strings"
)

func parseHide(in string) *Rule {
	h := &Rule{Hide: true}
	var parts []string
	var exception bool
	if strings.Index(in, "#@#") > -1 {
		exception = true
		parts = strings.SplitN(in, "#@#", 2)
	} else {
		parts = strings.SplitN(in, "##", 2)
	}
	if len(parts) == 0 || len(parts) == 1 {
		return nil
	}

	if len(parts[0]) > 0 {
		domains := strings.Split(parts[0], ",")
		if exception {
			for i, s := range domains {
				domains[i] = "~" + s
			}
		}
		h.Domains = domains
	}

	h.Selector = parts[1]

	return h
}

type Domain []string

func (d Domain) String() string {
	return strings.Join(reverseCopy(d), ".")
}

func reverse(in []string) {
	if len(in) < 2 {
		return
	}

	for i := 0; i < len(in)/2; i++ {
		in[i], in[len(in)-1-i] = in[len(in)-1-i], in[i]
	}
}

func reverseCopy(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for i := len(in) - 1; i >= 0; i-- {
		out = append(out, in[i])
	}
	return out
}

func NewDomain(s string) Domain {
	out := strings.Split(s, ".")
	reverse(out)
	return out
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

func (ds Domains) String() string {
	ss := make([]string, len(ds))
	for i, d := range ds {
		ss[i] = d.String()
	}
	return strings.Join(ss, ",")
}

func (ds Domains) Match(d Domain) bool {
	for _, e := range ds {
		if e.Match(d) {
			return true
		}
	}
	return false
}

type Hide struct {
	Domains   Domains
	Exclude   Domains
	Selectors []string
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
		d := h.Domains.String()
		e := h.Exclude.String()
		fmt.Fprintf(w, "/* domains: %q, exclude: %q */\n", d, e)
		for _, s := range h.Selectors {
			fmt.Fprint(w, s)
			fmt.Fprintln(w, "{display: none !important;}")
		}
		fmt.Fprintln(w)
	}
	return n, err
}
