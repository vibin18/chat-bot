# Go LLM Chat Bot

This is a Go application built with hexagonal architecture that provides a web UI for interacting with LLM models. It uses [langchaingo](https://github.com/tmc/langchaingo) for LLM integration and connects to an Ollama model.

## Features

- Hexagonal architecture for clean separation of concerns
- Structured logging using Go's slog package
- Integration with Ollama LLM models via langchaingo
- Modern web UI for chat interactions
- RESTful API for chat operations
- In-memory chat storage (can be extended to persistent storage)

## Architecture

The application follows the hexagonal architecture pattern:

- **Core domain**: Contains the business logic, domain models, and interfaces (ports)
- **Adapters**: Implements the interfaces defined in the core
  - **Primary adapters**: Handle incoming requests (HTTP)
  - **Secondary adapters**: Connect to external systems (LLM, repository)

## Configuration

The application can be configured through a JSON file or environment variables. Default configuration:

```json
{
  "server": {
    "port": 8080
  },
  "llm": {
    "provider": "ollama",
    "ollama": {
      "enabled": true,
      "endpoint": "http://192.168.1.222:11434",
      "model": "gemma3:1b",
      "max_tokens": 256,
      "timeout_seconds": 100
    }
  }
}
```

## Running the application

1. Ensure you have Go 1.22+ installed
2. Clone the repository
3. Run the application:

```bash
go run ./cmd/server/main.go
```

To run with custom configuration:

```bash
go run ./cmd/server/main.go -config=/path/to/config.json
```

To run with debug logging:

```bash
go run ./cmd/server/main.go -debug
```

## API Endpoints

- `GET /api/chats` - List all chats
- `POST /api/chats` - Create a new chat
- `GET /api/chats/{chatID}` - Get a specific chat
- `POST /api/chats/{chatID}/messages` - Send a message to a chat
- `DELETE /api/chats/{chatID}` - Delete a chat
- `GET /api/model` - Get information about the current LLM model

## Web UI

The application provides a web UI accessible at:

- `GET /` - Home page with list of chats
- `GET /chat/{chatID}` - Chat interface for a specific chat

## Dependencies

- [Chi router](https://github.com/go-chi/chi) for HTTP routing
- [langchaingo](https://github.com/tmc/langchaingo) for LLM integration

## License

MIT
