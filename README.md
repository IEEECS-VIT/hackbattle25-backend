# HackBattle25 Backend

This is the backend service for our awesome project.

## Prerequisites
- Go (version 1.21 or later) installed.
- Access to the project on Firebase. Ask a teammate for an invite.

## Setup
1.  Clone the repository:
    ```bash
    git clone <your-repository-url>
    ```
2.  Navigate into the project directory:
    ```bash
    cd hackbattle25-backend
    ```
3.  Go to the Firebase console, generate your own private service account key, and save it somewhere safe on your computer. **Do not commit this file.**
4.  Install the project dependencies:
    ```bash
    go mod tidy
    ```

## Running the Server
Set the environment variable with the path to your key and run the `main.go` file.

```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/your/serviceAccountKey.json" && go run main.go
```
The server will be running on `http://localhost:8080`. 🚀