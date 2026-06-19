# LLM WebUI

A beautifully designed, local-first web UI for Large Language Models. 
It compiles into a **single, cross-platform executable** with absolutely zero runtime dependencies. 
All assets are embedded within the binary, and it uses a local SQLite database for persistence.

## Features
- **Multi-Provider Support**: Chat with Ollama, OpenAI, Anthropic (Claude), and Google Gemini.
- **Single Binary Deployment**: No Node.js, no Docker, no Python required. Just download and run.
- **Local SQLite Persistence**: Conversations, messages, settings, and API keys are stored in a local `data.db` file.
- **Vanilla JS Frontend**: Incredibly fast, responsive UI without heavy JS frameworks. Includes a custom markdown parser with syntax highlighting and smooth micro-animations.
- **Advanced Inference Controls**: Fine-tune Temperature, Top-P, Max Tokens, and System Prompts per conversation.
- **Message Workflows**: Edit your past prompts to effortlessly branch history, or Regenerate assistant responses instantly.
- **Dynamic Theming**: Seamless Light and Dark mode toggle.
- **Fast Search**: Instantly filter your local conversation history.

## Getting Started

### Prerequisites
- [Go 1.22+](https://go.dev/dl/)

### Build
To compile the project from source, simply clone the repository and run:

```bash
go build -o llm-webui .
```
Or use the provided Makefile:
```bash
make build
```

### Run
Launch the compiled binary:
```bash
./llm-webui --port 42068 --db ./data.db
```
Once it's running, open your web browser and navigate to `http://localhost:42068`.

### Configuration
You can pass the following command-line flags to customize the application behavior:
- `--host`: The IP to bind to (default: `0.0.0.0`)
- `--port`: The HTTP server port (default: `42068`)
- `--db`: Path to the local SQLite database file (default: `data.db`)
- `--ollama-url`: Default endpoint for Ollama (default: `http://localhost:11434`)

## Architecture
- **Backend**: Go (Golang) leveraging `chi` for HTTP routing and `modernc.org/sqlite` for CGO-free database interactions.
- **Frontend**: Standard HTML5, CSS3, and ES6 JavaScript utilizing native APIs and `EventSource` for SSE streaming.

## License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
