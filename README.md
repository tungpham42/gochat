# GoChat (Go + Fyne)

GoChat is a desktop AI chatbot written in Go. It uses the [Fyne](https://fyne.io) GUI toolkit and talks to an AI service over HTTP.

- **Endpoint:** `https://api.soft.io.vn/ai`
- **Method:** `POST`
- **Input:** JSON body `{"prompt": "your message"}`
- **Output:** JSON with a `result` field, rendered as Markdown in the chat window

## Features

- Chat window with separate cards for your messages and the AI's replies
- AI replies rendered as Markdown (headings, bold, italic, lists, links, quotes, code blocks)
- Handles double-escaped text (literal `\n`, `\t`) and preserves line breaks
- Enter or the **Send** button sends a message; the input is locked while waiting
- Progress bar while the request is in flight (90 second timeout)
- **Copy** button on every AI reply
- Trash button in the header clears the conversation (with confirmation)
- Clear error messages for network failures, bad status codes, or a missing `result` field

## Requirements

| Requirement | Notes |
|---|---|
| Go 1.21 or newer | <https://go.dev/dl/> |
| C compiler (gcc / clang) | Fyne uses cgo for its graphics driver |
| Internet access | To download modules and to reach the API |

### Platform setup

**Windows**
1. Install Go.
2. Install a C compiler, e.g. [MSYS2](https://www.msys2.org/) and then in the MSYS2 shell: `pacman -S mingw-w64-x86_64-toolchain`. Add `C:\msys64\mingw64\bin` to your `PATH`.

**macOS**
```bash
xcode-select --install
```

**Linux (Debian/Ubuntu)**
```bash
sudo apt install gcc libgl1-mesa-dev xorg-dev
```

**Linux (Fedora)**
```bash
sudo dnf install gcc libXcursor-devel libXrandr-devel mesa-libGL-devel libXi-devel libXinerama-devel libXxf86vm-devel
```

## Project structure

```
gochat/
├── go.mod      # module definition and dependencies
├── main.go     # API client + Fyne GUI
└── README.md
```

## Run

```bash
cd gochat
go mod tidy     # downloads dependencies and creates go.sum
go run .
```

The first build can take a few minutes because Fyne compiles its native graphics code. Later builds are fast.

## Build a standalone executable

```bash
# current platform
go build -o gochat .

# Windows, without a console window
go build -ldflags "-H=windowsgui" -o gochat.exe .
```

For app icons and packaged installers, use the Fyne tool:

```bash
go install fyne.io/fyne/v2/cmd/fyne@latest
fyne package -os windows   # or darwin / linux
```

## How to use

1. Start the app.
2. Type a message in the box at the bottom.
3. Press **Enter** or click **Send**.
4. Wait for the reply. A progress bar shows while the AI is working.
5. Click **Copy** under a reply to copy its text.
6. Click the trash icon at the top right to clear the chat.

## How it works

`askAI()` in `main.go` sends the request:

```http
POST https://api.soft.io.vn/ai
Content-Type: application/json

{"prompt": "Hello"}
```

and expects a response like:

```json
{"result": "Hi! How can I help you?"}
```

The request runs in a goroutine so the window stays responsive. When it finishes, the UI is updated through `fyne.Do(...)`, which is required in Fyne 2.6+ for changes made from background goroutines.

## Configuration

| What | Where | How |
|---|---|---|
| API URL | `apiURL` constant in `main.go` | Change the string |
| Timeout | `httpClient` and the `context.WithTimeout` call in `send()` | Change `90 * time.Second` |
| Request format | `apiRequest` struct and `askAI()` | Edit field names or content type |
| Window size | `w.Resize(...)` in `main()` | Change the dimensions |

## Notes and limitations

- The API takes a single prompt, so each message is sent on its own and the AI does not remember earlier turns. To add context, build the prompt from recent history inside `send()`.
- Fyne's Markdown renderer does not support tables or images. They appear as plain text.
- If the response is not JSON, the raw text is shown instead.

## Troubleshooting

| Problem | Fix |
|---|---|
| `cgo: C compiler "gcc" not found` | Install a C compiler (see Platform setup) |
| `missing go.sum entry` | Run `go mod tidy` |
| `undefined: fyne.Do` | Your Fyne version is older than 2.6. Make sure `go.mod` requires `fyne.io/fyne/v2 v2.6.3` or newer and run `go mod tidy` |
| `server returned 4xx/5xx` | Check the API URL and request format; the error message includes the server's response |
| `response has no "result" field` | The API returned a different JSON shape. Adjust `askAI()` |
| Timeout / `request failed` | Check your internet connection or proxy settings, or increase the timeout |
| Blank or broken window on Linux VMs | Install the OpenGL libraries listed above, or try `LIBGL_ALWAYS_SOFTWARE=1 go run .` |

## License

Free to use and modify for your own projects.
