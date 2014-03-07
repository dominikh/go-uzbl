package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"time"

	"honnef.co/go/uzbl/adblock"
	"honnef.co/go/uzbl/event_manager"
)

type blocker struct {
	ab        *adblock.Adblock
	c         net.Conn
	num       int
	curDomain string
}

var (
	fSocket         string
	fCache          int
	fUserStylesheet string
	fAdStylesheet   string
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr,
			`This program provides a standalone, efficient and feature-rich
adblocker for uzbl-based browsers. It supports the Adblock Plus
filter rules, including element hiding rules.

Adblock listens on a socket for ADBLOCK requests, filters them
and sends back either the original URI or about:blank.
Additionally, it will install a user stylesheet that includes
element hiding rules for the current domain.

For this to work, an instance of adblock needs to be started
before uzbl starts, so that uzbl can connect to its socket.

Adblock uses uzbl's request_handler to filter requests. In order
to use adblock, add the following line to your config:

    set request_handler request ADBLOCK

Additionally, you need to tell uzbl to connect to the adblock socket, e.g. via

    uzbl-core --connect-socket=/tmp/adblock_socket

Since webkit1 only supports a single user stylesheet, and adblock
uses it for element hiding, adblock provides an option to read
and append a file to the generated stylesheet.`)
		fmt.Fprintf(os.Stderr, "\nUsage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.StringVar(&fSocket, "socket", "", "The socket to create and listen on")
	flag.StringVar(&fUserStylesheet, "user-stylesheet", "", "Path to user stylesheet to append")
	flag.StringVar(&fAdStylesheet, "ad-stylesheet", "", "Path where to store temporary ad stylesheet")
	flag.IntVar(&fCache, "cache", 50000, "The number of filter calculations to cache")
	flag.Parse()

	if fSocket == "" {
		fmt.Fprintln(os.Stderr, "No socket given")
		flag.Usage()
		os.Exit(1)
	}

	ab := adblock.New(fCache)

	for _, path := range flag.Args() {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Could not open rule file:", err)
			f.Close()
			continue
		}
		ab.LoadRules(f)
		f.Close()
	}

	log.Printf("Loaded %d rules, %d element hiding rules, %d keywords, %d rules without keywords",
		ab.Stats.NumRules, ab.Stats.NumHides, len(ab.Rules)+len(ab.Exceptions), ab.Stats.BlankKeywords)

	addr, err := net.ResolveUnixAddr("unix", fSocket)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not parse socket address:", err)
		os.Exit(2)
	}

	l, err := net.ListenUnix("unix", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not open socket:", err)
		os.Exit(3)
	}

	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error in Accept():", err)
			os.Exit(4)
		}
		go runBlocker(&blocker{ab: ab, c: c})
	}
}

func runBlocker(b *blocker) {
	em := event_manager.New(b.c)
	em.AddHandler("REQUEST-ADBLOCK", b.evPolicyRequest)
	em.AddHandler("LOAD_COMMIT", b.evLoadCommit)
	em.Listen()
}

func (b *blocker) evPolicyRequest(ev *event_manager.Event) error {
	args := ev.ParseDetail(2)
	if len(args) != 2 {
		return fmt.Errorf("malformed POLICY_REQUEST")
	}

	uri := args[0]
	t1 := time.Now()
	_, matches := b.ab.Match(b.curDomain, uri)
	t2 := time.Now()
	log.Println("Took", t2.Sub(t1), "to filter")
	if matches {
		uri = "about:blank"
	}

	fmt.Fprintf(b.c, "REPLY-%s %s\n", ev.Cookie, uri)
	b.num++
	if b.num%20 == 0 {
		log.Println(b.ab.Stats)
	}
	return nil
}

func (b *blocker) evLoadCommit(ev *event_manager.Event) error {
	args := ev.ParseDetail(1)
	if len(args) != 1 {
		return fmt.Errorf("malformed NAVIGATION_STARTING")
	}
	u, err := url.Parse(args[0][1 : len(args[0])-1])
	if err != nil {
		return fmt.Errorf("error parsing host: %s", err)
	}
	log.Printf("old: %s, new: %s", b.curDomain, u.Host)
	if u.Host == b.curDomain {
		return nil
	}
	b.curDomain = u.Host

	if fAdStylesheet == "" {
		return nil
	}

	hides := b.ab.Hide(b.curDomain)
	log.Printf("%d hide rules", len(hides))
	f, err := os.Create(fAdStylesheet)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = hides.WriteStylesheet(f)
	if err != nil {
		return err
	}

	if fUserStylesheet != "" {
		f2, err := os.Open(fUserStylesheet)
		if err != nil {
			return err
		}
		defer f2.Close()
		io.Copy(f, f2)
	}

	fmt.Fprintln(b.c, "css clear")
	fmt.Fprintln(b.c, "css add file://"+fAdStylesheet)
	return nil
}
