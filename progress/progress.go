package progress

import (
	"fmt"
	"strconv"
	"strings"

	"honnef.co/go/uzbl"
)

func New(u *uzbl.Uzbl) *ProgressBar {
	pb := &ProgressBar{uzbl: u}
	u.EM.AddHandler("LOAD_COMMIT", pb.evLoadCommit)
	u.EM.AddHandler("LOAD_PROGRESS", pb.evLoadProgress)
	return pb
}

type ProgressBar struct {
	uzbl    *uzbl.Uzbl
	updates int
}

func (p *ProgressBar) evLoadCommit(ev *uzbl.Event) error {
	p.updates = 0
	return p.evLoadProgress(ev)
}

func (p *ProgressBar) evLoadProgress(ev *uzbl.Event) error {
	p.updates++
	progress := 100
	var err error
	if ev.Detail != "" {
		progress, err = strconv.Atoi(ev.Detail)
		if err != nil {
			return err
		}
	}

	format := ev.Uzbl.Variables.GetString("progress.format", "[%d>%p]%c")
	swidth := ev.Uzbl.Variables.GetString("progress.width", "8")
	width, err := strconv.Atoi(swidth)
	if err != nil {
		return err
	}
	doneSymbol := ev.Uzbl.Variables.GetString("progress.done", "=")
	pendingSymbol := ev.Uzbl.Variables.GetString("progress.pending", " ")
	if pendingSymbol == "" {
		pendingSymbol = " "
	}

	spinner := ev.Uzbl.Variables.GetString("progress.spinner", "-\\|/")
	index := 0
	if progress != 100 {
		index = p.updates % len(spinner)
	}
	spinner = string(spinner[index])
	if spinner == `\` {
		spinner = `\\`
	}

	sprites := ev.Uzbl.Variables.GetString("progress.sprites", "loading")
	index = int(((float64(progress)/100.0)*float64(len(sprites)))+0.5) - 1
	sprite := string(sprites[index])
	if sprite == `\` {
		sprite = `\\`
	}

	count := strings.Count(format, "%c") + strings.Count(format, "%i")
	width += (3 - len(strconv.Itoa(progress))) * count

	count = strings.Count(format, "%t") + strings.Count(format, "%o")
	width += (3 - len(strconv.Itoa(100-progress))) * count

	done := int(((float64(progress) / 100.0) * float64(width)) + 0.5)
	pending := width - done

	// FIXME string concat is silly, but for a progress bar it
	// shouldn't be that bad
	output := ""
	inFormat := false
	for _, c := range format {
		if inFormat {
			switch c {
			case 'd':
				output += strings.Repeat(doneSymbol, done)
			case 'p':
				output += strings.Repeat(pendingSymbol, pending)
			case 'c':
				output += strconv.Itoa(progress) + "%"
			case 'i':
				output += strconv.Itoa(progress)
			case 't':
				output += strconv.Itoa(100-progress) + "%"
			case 'o':
				output += strconv.Itoa(100 - progress)
			case 's':
				output += spinner
			case 'r':
				output += sprite
			case '%':
				output += "%"
			}
			inFormat = false
			continue
		}
		if c == '%' {
			inFormat = true
			continue
		}
		output += string(c)
	}

	ev.Uzbl.Send(fmt.Sprintf("set progress.output = %s", output))
	return nil
}
