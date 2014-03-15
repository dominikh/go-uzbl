package adblock

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
)

var reKeyword = regexp.MustCompile(`[a-zA-Z0-9&?]{3,}`)

type hash struct {
	hash uint32
	pow  uint32
	sep  string
}

type pair struct {
	src string
	req string
}

type Adblock struct {
	Rules      map[hash][]*Rule
	Exceptions map[hash][]*Rule
	Cache      *LRU
	Hides      Hides
}

func New(cacheSize int) *Adblock {
	// 50,000 entries will make for approximately 15-20 MB of memory
	// usage
	return &Adblock{
		Rules:      make(map[hash][]*Rule),
		Exceptions: make(map[hash][]*Rule),
		Cache:      NewLRU(cacheSize),
	}
}

func (adblock *Adblock) addHide(rule *Rule) {
	var domains Domains
	var exclude Domains
	for _, domain := range rule.Domains {
		if domain[0] == '~' {
			exclude = append(exclude, NewDomain(domain[1:]))
		} else {
			domains = append(domains, NewDomain(domain))
		}
	}
	adblock.Hides = append(adblock.Hides,
		&Hide{Domains: domains, Exclude: exclude, Selectors: []string{rule.Selector}})
	return
}

func (adblock *Adblock) AddRule(rule *Rule) {
	if rule.Hide {
		adblock.addHide(rule)
		return
	}

	hash := hashstr(rule.Keyword)
	if rule.Exception {
		adblock.Exceptions[hash] = append(adblock.Exceptions[hash], rule)
	} else {
		adblock.Rules[hash] = append(adblock.Rules[hash], rule)
	}
}

func (adblock *Adblock) LoadRules(r io.Reader) (rules int) {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		rule := Parse(line)

		if rule != nil {
			rules++
			adblock.AddRule(rule)
		}
	}

	adblock.Hides = mergeOnSelector(adblock.Hides)
	return rules
}

// Optimize optimizes the stored rules. After calling this function,
// LoadRules must not be called anymore.
func (adblock *Adblock) Optimize() {
	adblock.Hides = mergeOnDomain(adblock.Hides)
}

// mergeOnSelector merges all hides that have the same selector. This
// ensures that for a selector, all domains and excludes are known.
//
// Calling this is required, otherwise finding hides for a domain may
// return wrong results, not being aware of an exclude for the rule.
//
// This function must not be called after mergeOnDomain has been
// called, because that function merges multiple selectors if they
// have the same domains and excludes.
func mergeOnSelector(in []*Hide) []*Hide {
	var out []*Hide

	newHides := make(map[string]*Hide)
	for _, hide := range in {
		key := strings.Join(hide.Selectors, ",")
		var h *Hide
		var ok bool
		if h, ok = newHides[key]; !ok {
			h = &Hide{Selectors: hide.Selectors}
			newHides[key] = h
		}
		h.Domains = append(h.Domains, hide.Domains...)
		h.Exclude = append(h.Domains, hide.Exclude...)
	}

	for _, hide := range newHides {
		out = append(out, hide)
	}
	return out
}

// mergeOnDomain merges all hides that have the same combination of
// domains and excludes.
//
// Calling this function is optional since all it does is reduce
// memory usage and improve matching speed. In theory it can also be
// called multiple times.
func mergeOnDomain(in []*Hide) []*Hide {
	var out []*Hide

	newHides := make(map[string]*Hide)
	for _, hide := range in {
		key := hide.Domains.String() + "|" + hide.Exclude.String()
		var h *Hide
		var ok bool
		if h, ok = newHides[key]; !ok {
			h = &Hide{Domains: hide.Domains, Exclude: hide.Exclude}
			newHides[key] = h
		}
		h.Selectors = append(h.Selectors, hide.Selectors...)
	}

	for _, hide := range newHides {
		out = append(out, hide)
	}
	return out
}

type Rule struct {
	Exception  bool
	Regexp     *regexp.Regexp
	Hash       hash
	Domains    []string
	ThirdParty bool
	FirstParty bool
	Hide       bool
	Selector   string
	Keyword    string
}

func (r *Rule) String() string {
	if r.Regexp != nil {
		return r.Regexp.String()
	}
	return fmt.Sprintf("%q", r.Hash.sep)
}

func (r *Rule) matchOrigin(src, req string) bool {
	if !r.ThirdParty && !r.FirstParty {
		return true
	}
	srcURL, err := url.Parse(src)
	if err != nil {
		return false
	}
	reqURL, err := url.Parse(req)
	if err != nil {
		return false
	}

	return srcURL.Host == reqURL.Host != r.ThirdParty
}

func (r *Rule) Match(src string, req string) (ret bool) {
	if !matchesDomain(src, r.Domains) {
		return false
	}
	if r.Regexp == nil {
		return strMatch(req, r.Hash) && r.matchOrigin(src, req)
	}
	return r.Regexp.MatchString(req) && r.matchOrigin(src, req)
}

func parseRule(in string) *Rule {
	r := &Rule{}
	var matchCase bool

	if len(in) == 0 {
		panic("not a valid rule: empty")
	}
	if in[0] == '!' {
		return nil
	}
	if len(in) >= 2 && in[0:2] == "@@" {
		r.Exception = true
		in = in[2:]
	}

	parts := strings.SplitN(in, "$", 2)
	in = parts[0]

	if len(in) == 0 {
		// FIXME with a $third-party rule, this might actually make sense
		return nil
	}

	r.Keyword = extractKeyword(in)

	if len(parts) == 2 {
		options := strings.Split(parts[1], ",")
		for _, option := range options {
			switch option {
			case "match-case":
				matchCase = true
			case "third-party":
				r.ThirdParty = true
			case "~third-party":
				r.FirstParty = true
			case "script", "image", "stylesheet", "object", "xmlhttprequest", "object-subrequest",
				"subdocument", "document", "elemhide", "other", "background", "xbl", "ping", "dtd",
				"~script", "~image", "~stylesheet", "~object", "~xmlhttprequest", "~object-subrequest",
				"~subdocument", "~document", "~elemhide", "~other", "~background", "~xbl", "~ping", "~dtd":
				// We don't know what kind of request we're working with, so reject these rules
				return nil
			default:
				if len(option) >= 6 && option[:6] == "domain" {
					parts := strings.SplitN(option, "=", 2)
					r.Domains = strings.Split(parts[1], "|")
				}
			}
		}
	}

	// TODO benchmark how much string concat is hurting us. this is
	// the most naive of implementations
	var out string
	var glob bool
	for i, c := range in {
		switch c {
		case '^':
			glob = true
			out += "(?:"
			if i == 0 {
				out += "^|"
			}
			out += `[^a-zA-Z0-9_.%-]`
			if i == len(in)-1 {
				out += "|$"
			}
			out += ")"
		case '*':
			glob = true
			out += ".*"
		case '|':
			glob = true
			if i == len(in)-1 {
				out += "$"
			}

			if i == 0 {
				out += "^"
			}

			if i == 1 && in[0] == '|' {
				out += `(?:[^:]+://(?:[^/]+\.)?)`
			}
		default:
			out += regexp.QuoteMeta(string(c))
		}
	}

	if !matchCase {
		out = "(?i)" + out
	}

	if glob {
		rex := regexp.MustCompile(out)
		r.Regexp = rex
	} else {
		r.Hash = hashstr(in)
	}
	return r
}

func Parse(in string) (rule *Rule) {
	if strings.Contains(in, "##") || strings.Contains(in, "#@#") {
		return parseHide(in)
	}
	return parseRule(in)
}

func matchesDomain(src string, domains []string) bool {
	if len(domains) == 0 {
		return true
	}
	for _, domain := range domains {
		if domain[0] == '~' {
			if domain[1:] != src {
				return true
			}
		} else if domain == src {
			return true
		}
	}
	return false
}

func matchesAny(src, req string, rules []*Rule) (*Rule, bool) {
	for _, rule := range rules {
		if rule.Match(src, req) {
			return rule, true
		}
	}
	return nil, false
}

func filterRules(req string, rules map[hash][]*Rule) []*Rule {
	var out []*Rule
	for keyword := range rules {
		if !strMatch(req, keyword) {
			continue
		}
		out = append(out, rules[keyword]...)
	}
	return out
}

func (adblock *Adblock) Hide(srcDomain string) Hides {
	d := NewDomain(srcDomain)
	return adblock.Hides.Find(d)
}

func (adblock *Adblock) Match(src string, req string) (*Rule, bool) {
	var rule *Rule
	var ret bool

	pair := pair{src, req}
	if ret, ok := adblock.Cache.Get(pair); ok {
		return nil, ret
	}

	toCheck := filterRules(req, adblock.Rules)
	rule, ret = matchesAny(src, req, toCheck)
	if !ret {
		adblock.Cache.Set(pair, false)
		return nil, false
	}

	toCheck = filterRules(req, adblock.Exceptions)
	exc, ret := matchesAny(src, req, toCheck)
	if ret {
		adblock.Cache.Set(pair, false)
		return exc, false
	}
	adblock.Cache.Set(pair, true)
	return rule, true
}

const primeRK = 16777619

func hashstr(sep string) hash {
	h := uint32(0)
	for i := 0; i < len(sep); i++ {
		h = h*primeRK + uint32(sep[i])

	}
	var pow, sq uint32 = 1, primeRK
	for i := len(sep); i > 0; i >>= 1 {
		if i&1 != 0 {
			pow *= sq
		}
		sq *= sq
	}
	return hash{h, pow, sep}
}

func strMatch(s string, hash hash) bool {
	n := len(hash.sep)
	if n > len(s) {
		return false
	}

	hashsep, pow := hash.hash, hash.pow
	var h uint32
	for i := 0; i < n; i++ {
		h = h*primeRK + uint32(s[i])
	}
	if h == hashsep && s[:n] == hash.sep {
		return true
	}
	for i := n; i < len(s); {
		h *= primeRK
		h += uint32(s[i])
		h -= pow * uint32(s[i-n])
		i++
		if h == hashsep && s[i-n:i] == hash.sep {
			return true
		}
	}
	return false
}

func extractKeyword(in string) string {
	keywords := reKeyword.FindAllString(in, -1)
	if len(keywords) == 0 {
		return ""
	}
	longest := keywords[0]
	for _, keyword := range keywords {
		if len(keyword) > len(longest) {
			longest = keyword
		}

	}
	return longest
}
