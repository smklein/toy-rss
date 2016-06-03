package rssUtilities

import (
	"log"
	"sync"
	"unicode/utf8"

	tb "github.com/nsf/termbox-go"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// View gives access to the user.
type View struct {
	maxLines      int
	redrawRequest chan bool
	input         []byte

	deathWg *sync.WaitGroup

	NewFeedRequest chan string
	ExitRequest    chan bool
}

// Start launches the view.
func (v *View) Start(deathWg *sync.WaitGroup) {
	log.Println("View started")
	v.maxLines = 50
	v.redrawRequest = make(chan bool)
	v.deathWg = deathWg

	v.NewFeedRequest = make(chan string)
	v.ExitRequest = make(chan bool)

	go v.drawLoop()
	go v.eventLoop()
}

func (v *View) inputRune(r rune) {
	var buf [utf8.UTFMax]byte
	n := utf8.EncodeRune(buf[:], r)
	log.Println("Encoded rune: ", r, ", size: ", n)
	if n != 1 {
		panic("Unexpected rune size")
	}
	v.input = append(v.input, buf[0])
	log.Println("Len of input: ", len(v.input))
}

func (v *View) eventLoop() {
	for {
		switch ev := tb.PollEvent(); ev.Type {
		case tb.EventKey:
			switch ev.Key {
			case tb.KeyEsc, tb.KeyCtrlC:
				close(v.ExitRequest)
				return
			case tb.KeyEnter:
				log.Println("ENTER")
				v.input = make([]byte, 0)
			case tb.KeyTab:
				log.Println("TAB KEY. Switch mode (input / navigate)")
			case tb.KeyArrowLeft:
				log.Println("LEFT")
			case tb.KeyArrowRight:
				log.Println("RIGHT")
			case tb.KeyArrowUp:
				log.Println("UP")
			case tb.KeyArrowDown:
				log.Println("DOWN")
			case tb.KeyBackspace, tb.KeyBackspace2:
				log.Println("DELETE DELETE DELETE")
				// TODO LOCK
				if len(v.input) > 0 {
					v.input = v.input[:len(v.input)-1]
				}
			default:
				// TODO(smklein): Restrict keys a little more, yeah?
				log.Println("You pressed: ", ev.Ch)
				v.inputRune(ev.Ch)
			}
		case tb.EventError:
			panic(ev.Err)
		}

		// Pls redraw the screen after user input. User latency 'n' all.
		v.redrawRequest <- true
	}
}

func (v *View) drawLoop() {
	// Initialize termbox-go
	check(tb.Init())
	defer func() {
		tb.Close()
		v.deathWg.Done()
	}()
	// TODO(smklein): Better cursor management! Not hidden tho...
	tb.SetCursor(0, 0)
	//	tb.SetInputMode(tb.InputAlt)

	for {
		v.redrawAll()
		select {
		case <-v.redrawRequest:
		case <-v.ExitRequest:
			return
		}
	}
}

func (v *View) redrawAll() {
	check(tb.Clear(tb.ColorDefault, tb.ColorDefault))
	w, h := tb.Size()

	log.Println("Width: ", w, ", Height: ", h)
	fgColor := tb.ColorGreen

	// TODO(smklein): Lock the copy
	inputString := string(v.input)
	log.Println("Input string: ", inputString)

	for x := 0; x <= w; x++ {
		for y := 0; y <= h; y++ {
			if y == h-1 {
				if x <= 2 {
					tb.SetCell(x, y, '>', fgColor, tb.ColorDefault)
				} else if x == 3 {
					tb.SetCell(x, y, ' ', fgColor, tb.ColorDefault)
				} else {
					if len(inputString) > 0 {
						r, size := utf8.DecodeRuneInString(inputString)
						tb.SetCell(x, y, r, fgColor, tb.ColorDefault)
						inputString = inputString[size:]
					} else {
						tb.SetCell(x, y, ' ', fgColor, tb.ColorDefault)
					}
				}
			} else {
				tb.SetCell(x, y, '@', fgColor, tb.ColorDefault)
			}
		}
	}
	tb.Flush()

	// TODO do something better than this...
}
