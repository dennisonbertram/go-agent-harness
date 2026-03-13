package main

import (
	"fmt"
	"os"

	"go-agent-harness/internal/provider/catalog"
	"golang.org/x/term"
)

type pickerItem struct {
	modelKey    string // empty = non-selectable header row
	providerKey string // catalog provider key for selectable items; empty for headers
	displayLine string // rendered text (provider name or "  modelkey  $x/$y")
}

func buildPickerItems(cat *catalog.Catalog) []pickerItem {
	if cat == nil || len(cat.Providers) == 0 {
		return nil
	}
	var items []pickerItem
	for _, provName := range providerOrder(cat.Providers) {
		prov := cat.Providers[provName]
		displayName := prov.DisplayName
		if displayName == "" {
			displayName = provName
		}
		items = append(items, pickerItem{
			modelKey:    "",
			displayLine: "  [" + displayName + "]",
		})
		for _, key := range modelOrder(prov.Models) {
			m := prov.Models[key]
			pricing := ""
			if m.Pricing != nil {
				pricing = fmt.Sprintf("  ($%.2f/$%.2f per 1M)", m.Pricing.InputPer1MTokensUSD, m.Pricing.OutputPer1MTokensUSD)
			}
			items = append(items, pickerItem{
				modelKey:    key,
				providerKey: provName,
				displayLine: "    " + key + pricing,
			})
		}
	}
	return items
}

// firstSelectable walks from `from` in direction `dir` (+1 or -1) and returns
// the first index with a selectable (non-header) item, or -1 if none found.
func firstSelectable(items []pickerItem, from, dir int) int {
	for i := 0; i < len(items); i++ {
		idx := from + dir*i
		if idx < 0 || idx >= len(items) {
			return -1
		}
		if items[idx].modelKey != "" {
			return idx
		}
	}
	return -1
}

// selectModel runs an interactive terminal picker and returns the selected model key
// and provider key, or ("", "") if cancelled.
func selectModel(cat *catalog.Catalog, noColor bool) (string, string) {
	items := buildPickerItems(cat)
	selected := firstSelectable(items, 0, +1)
	if selected < 0 {
		return "", ""
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "term.MakeRaw: %v\n", err)
		return "", ""
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState) //nolint:errcheck

	// Enter alternate screen buffer and hide cursor.
	// defer runs before term.Restore (LIFO), so the terminal is still in raw
	// mode when we exit the alt screen — that's fine; term.Restore follows.
	fmt.Print("\033[?1049h\033[?25l")
	defer fmt.Print("\033[?1049l\033[?25h")

	renderPicker(items, selected, noColor)

	buf := make([]byte, 3)
	for {
		n, _ := os.Stdin.Read(buf)
		if n == 0 {
			continue
		}
		switch {
		case (n >= 3 && buf[0] == 0x1b && buf[1] == '[' && buf[2] == 'A') ||
			(n == 1 && buf[0] == 'k'):
			// Up
			if ns := firstSelectable(items, selected-1, -1); ns >= 0 {
				selected = ns
				renderPicker(items, selected, noColor)
			}
		case (n >= 3 && buf[0] == 0x1b && buf[1] == '[' && buf[2] == 'B') ||
			(n == 1 && buf[0] == 'j'):
			// Down
			if ns := firstSelectable(items, selected+1, +1); ns >= 0 {
				selected = ns
				renderPicker(items, selected, noColor)
			}
		case n == 1 && (buf[0] == '\r' || buf[0] == '\n'):
			// Enter — defers handle alt-screen exit + term restore
			return items[selected].modelKey, items[selected].providerKey
		case n == 1 && buf[0] == 0x1b:
			// Esc alone
			return "", ""
		case n == 1 && (buf[0] == 'q' || buf[0] == 3):
			// q or Ctrl-C
			return "", ""
		}
		// Clear buf for next read
		buf[0], buf[1], buf[2] = 0, 0, 0
	}
}

func renderPicker(items []pickerItem, selected int, noColor bool) {
	// In raw mode \n only moves down — must use \r\n for proper line breaks.
	// Move to top-left and clear the alternate-screen buffer.
	fmt.Print("\033[H\033[2J")
	fmt.Print("Select model  \u2191\u2193 or j/k navigate  Enter select  Esc/q/Ctrl-C cancel\r\n\r\n")
	for i, item := range items {
		if item.modelKey == "" {
			// Provider header — dim
			if noColor {
				fmt.Printf("%s\r\n", item.displayLine)
			} else {
				fmt.Printf("\033[2m%s\033[0m\r\n", item.displayLine)
			}
		} else if i == selected {
			// Selected model — reverse video
			if noColor {
				fmt.Printf("> %s\r\n", item.displayLine)
			} else {
				fmt.Printf("\033[7m> %s\033[0m\r\n", item.displayLine)
			}
		} else {
			fmt.Printf("  %s\r\n", item.displayLine)
		}
	}
}
