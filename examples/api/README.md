# API Server Example

This example demonstrates a REST API server with hot reload (no browser reload).

## Running

```bash
# From this directory
hot --mode api

# Or specify the path
hot --mode api --watch ./examples/api
```

## What it does

1. Starts an API server on port 8080
2. Watches for changes to Go files
3. Automatically rebuilds and restarts the server
4. Does NOT start a browser reload server (lighter and faster)

## Try it

1. Run the command above
2. Test the API endpoints:
   ```bash
   # Check status
   curl http://localhost:8080/api/status
   
   # Get users
   curl http://localhost:8080/api/users
   ```
3. Edit `main.go` (e.g., change the version number or add a new user)
4. Save the file
5. The server will automatically restart
6. Test the endpoint again to see your changes!

## Expected Output

Status endpoint:
```json
{
  "status": "ok",
  "message": "API is running!",
  "timestamp": "2024-01-15T10:30:45Z",
  "version": "1.0.0"
}
```

Users endpoint:
```json
[
  {"id": 1, "name": "Alice", "email": "alice@example.com"},
  {"id": 2, "name": "Bob", "email": "bob@example.com"},
  {"id": 3, "name": "Charlie", "email": "charlie@example.com"}
]
```
