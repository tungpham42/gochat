package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const apiURL = "https://api.soft.io.vn/ai"

var httpClient = &http.Client{Timeout: 90 * time.Second}

// ---------------------------------------------------------------- API layer

type apiRequest struct {
	Prompt string `json:"prompt"`
}

// askAI sends the prompt to the API (POST, JSON {"prompt": "..."})
// and returns the "result" field of the response.
func askAI(ctx context.Context, prompt string) (string, error) {
	payload, err := json.Marshal(apiRequest{Prompt: prompt})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("server returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	// Expected: {"result": "..."}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		// Not JSON – fall back to the raw text.
		return strings.TrimSpace(string(body)), nil
	}
	v, ok := parsed["result"]
	if !ok {
		return "", fmt.Errorf("response has no \"result\" field: %s", strings.TrimSpace(string(body)))
	}
	if s, ok := v.(string); ok {
		return s, nil
	}
	pretty, _ := json.MarshalIndent(v, "", "  ")
	return string(pretty), nil
}

// ---------------------------------------------------------------- GUI layer

type chatUI struct {
	win      fyne.Window
	messages *fyne.Container
	scroll   *container.Scroll
	entry    *widget.Entry
	sendBtn  *widget.Button
	progress *widget.ProgressBarInfinite
	busy     bool
}

func main() {
	a := app.New()
	w := a.NewWindow("GoChat")
	w.Resize(fyne.NewSize(720, 640))

	ui := &chatUI{win: w}
	w.SetContent(ui.build())
	ui.addBubble("AI", "Hello! 👋 Ask me anything.", nil)

	w.ShowAndRun()
}

func (c *chatUI) build() fyne.CanvasObject {
	c.messages = container.NewVBox()
	c.scroll = container.NewVScroll(c.messages)

	c.progress = widget.NewProgressBarInfinite()
	c.progress.Hide()

	c.entry = widget.NewEntry()
	c.entry.SetPlaceHolder("Type a message and press Enter…")
	c.entry.OnSubmitted = func(string) { c.send() }

	c.sendBtn = widget.NewButtonWithIcon("Send", theme.MailSendIcon(), c.send)
	c.sendBtn.Importance = widget.HighImportance

	clearBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		dialog.ShowConfirm("Clear chat", "Delete the whole conversation?", func(ok bool) {
			if ok {
				c.messages.Objects = nil
				c.messages.Refresh()
			}
		}, c.win)
	})

	title := canvas.NewText("GoChat", theme.Color(theme.ColorNamePrimary))
	title.TextSize = 18
	title.TextStyle = fyne.TextStyle{Bold: true}
	header := container.NewBorder(nil, widget.NewSeparator(), nil, clearBtn,
		container.NewPadded(title))

	input := container.NewBorder(nil, nil, nil, c.sendBtn, c.entry)
	footer := container.NewVBox(c.progress, container.NewPadded(input))

	return container.NewBorder(header, footer, nil, nil, c.scroll)
}

// addBubble appends a message card. For the AI it renders Markdown.
// It returns the RichText so the caller can update it later.
func (c *chatUI) addBubble(sender, text string, _ any) *widget.RichText {
	rt := widget.NewRichTextFromMarkdown(text)
	rt.Wrapping = fyne.TextWrapWord

	var content fyne.CanvasObject = rt
	if sender == "AI" {
		copyBtn := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
			c.win.Clipboard().SetContent(rt.String())
		})
		copyBtn.Importance = widget.LowImportance
		content = container.NewVBox(rt, container.NewHBox(layout.NewSpacer(), copyBtn))
	}
	card := widget.NewCard(sender, "", content)
	// Indent the bubbles so user/AI messages are visually distinct.
	var row fyne.CanvasObject
	spacer := canvas.NewRectangle(nil)
	spacer.SetMinSize(fyne.NewSize(60, 0))
	if sender == "You" {
		row = container.NewBorder(nil, nil, spacer, nil, card)
	} else {
		row = container.NewBorder(nil, nil, nil, spacer, card)
	}

	c.messages.Add(row)
	c.scroll.ScrollToBottom()
	return rt
}

func (c *chatUI) setBusy(b bool) {
	c.busy = b
	if b {
		c.sendBtn.Disable()
		c.entry.Disable()
		c.progress.Show()
		c.progress.Start()
	} else {
		c.sendBtn.Enable()
		c.entry.Enable()
		c.progress.Stop()
		c.progress.Hide()
		c.win.Canvas().Focus(c.entry)
	}
}

func (c *chatUI) send() {
	prompt := strings.TrimSpace(c.entry.Text)
	if prompt == "" || c.busy {
		return
	}
	c.entry.SetText("")
	c.addBubble("You", escapeMarkdown(prompt), nil)
	reply := c.addBubble("AI", "*Thinking…*", nil)
	c.setBusy(true)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		result, err := askAI(ctx, prompt)

		// All UI updates must happen on the Fyne main thread.
		fyne.Do(func() {
			if err != nil {
				reply.ParseMarkdown("⚠️ **Error:** " + escapeMarkdown(err.Error()))
			} else {
				reply.ParseMarkdown(normalizeMarkdown(result))
			}
			reply.Refresh()
			c.messages.Refresh()
			c.scroll.ScrollToBottom()
			c.setBusy(false)
		})
	}()
}

// escapeMarkdown keeps user text literal (no accidental formatting).
func escapeMarkdown(s string) string {
	r := strings.NewReplacer(`\`, `\\`, "*", `\*`, "_", `\_`, "`", "\\`", "#", `\#`, "[", `\[`, "]", `\]`)
	return r.Replace(s)
}

// normalizeMarkdown prepares the API text for Fyne's Markdown renderer:
//   - turns literal "\n" / "\t" / "\"" sequences into real characters
//     (some APIs double-escape their output),
//   - turns single line breaks into paragraph breaks so lines don't collapse
//     into one another (code blocks, lists, headings and quotes are untouched).
func normalizeMarkdown(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.Contains(s, "\n") && strings.Contains(s, `\n`) {
		s = strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\"`, `"`).Replace(s)
	}

	isBlock := func(l string) bool {
		t := strings.TrimSpace(l)
		if t == "" {
			return false
		}
		switch {
		case strings.HasPrefix(t, "#"), strings.HasPrefix(t, ">"),
			strings.HasPrefix(t, "- "), strings.HasPrefix(t, "* "), strings.HasPrefix(t, "+ "),
			strings.HasPrefix(t, "---"), strings.HasPrefix(t, "|"):
			return true
		}
		// ordered list: "1. " / "12) "
		i := 0
		for i < len(t) && t[i] >= '0' && t[i] <= '9' {
			i++
		}
		return i > 0 && i+1 < len(t) && (t[i] == '.' || t[i] == ')') && t[i+1] == ' '
	}

	lines := strings.Split(s, "\n")
	var out strings.Builder
	inFence := false
	for i, l := range lines {
		out.WriteString(l)
		if i == len(lines)-1 {
			break
		}
		out.WriteString("\n")
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			inFence = !inFence
			continue
		}
		if inFence || strings.TrimSpace(l) == "" || strings.TrimSpace(lines[i+1]) == "" {
			continue
		}
		// plain text line followed directly by another plain text line
		if !isBlock(l) && !isBlock(lines[i+1]) && !strings.HasPrefix(strings.TrimSpace(lines[i+1]), "```") {
			out.WriteString("\n")
		}
	}
	return out.String()
}
