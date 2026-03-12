package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

type settingItem struct {
	label    string
	getValue func() string
	toggle   func()
}

func showSettings(display *Display) {
	items := []settingItem{
		{
			label: "Verbose mode (tool output + run IDs)",
			getValue: func() string {
				if display.Verbose {
					return "ON"
				}
				return "OFF"
			},
			toggle: func() { display.Verbose = !display.Verbose },
		},
	}

	selected := 0
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "term.MakeRaw: %v\n", err)
		return
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState) //nolint:errcheck

	fmt.Print("\033[?1049h\033[?25l")
	defer fmt.Print("\033[?1049l\033[?25h")

	renderSettings(items, selected)

	buf := make([]byte, 3)
	for {
		n, _ := os.Stdin.Read(buf)
		if n == 0 {
			continue
		}
		switch {
		case n >= 3 && buf[0] == 0x1b && buf[1] == '[' && buf[2] == 'A': // up
			if selected > 0 {
				selected--
				renderSettings(items, selected)
			}
		case n >= 3 && buf[0] == 0x1b && buf[1] == '[' && buf[2] == 'B': // down
			if selected < len(items)-1 {
				selected++
				renderSettings(items, selected)
			}
		case n == 1 && (buf[0] == '\r' || buf[0] == '\n' || buf[0] == ' '): // enter/space = toggle
			items[selected].toggle()
			renderSettings(items, selected)
		case n == 1 && (buf[0] == 0x1b || buf[0] == 'q' || buf[0] == 3): // esc/q/ctrl-c = exit
			return
		}
		buf[0], buf[1], buf[2] = 0, 0, 0
	}
}

func renderSettings(items []settingItem, selected int) {
	fmt.Print("\033[H\033[2J")
	fmt.Print("Settings  \u2191\u2193 navigate  Enter/Space toggle  Esc/q done\r\n\r\n")
	for i, item := range items {
		value := item.getValue()
		if i == selected {
			fmt.Printf("\033[7m> %-45s [%s]\033[0m\r\n", item.label, value)
		} else {
			fmt.Printf("  %-45s [%s]\r\n", item.label, value)
		}
	}
}
