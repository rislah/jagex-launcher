# Jagex OSRS Launcher

A cross-platform GUI launcher for Old School RuneScape enables launching RuneLite with different Jagex accounts. Built using Go and the Fyne GUI toolkit.


![Screenshot 1](screenshot.png)

![Screenshot 2](screenshot2.png)

![Screenshot 3](screenshot3.png)

![Screenshot 4](screenshot4.jpg)

## Why?
Jagex has explicitly stated that ["The Jagex launcher and Jagex Accounts will not be supported on the Linux OS"](https://help.jagex.com/hc/en-gb/articles/13413514881937). This leaves Wine as the only official alternative, which I don't like.

Furthermore, the official Jagex Launcher is known for its intrusive telemetry collection.

## Getting Started

### Prerequisites

*   Go
*   A Jagex Account
*   RuneLite installed on your system.

### Building from Source

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/rislah/jagex-launcher.git
    cd jagex-launcher
    ```
2.  **Build the application:**
    For Windows:
    ```bash
    go build -o jagex-launcher.exe -ldflags -H=windowsgui
    ```
    For Linux/macOS:
    ```bash
    go build -o jagex-launcher .
    ```

    Alternatively, you can build using the Fyne utility:

    For Windows:
    ```bash
        go install fyne.io/tools/cmd/fyne@latest
        fyne release -os windows -icon 5.ico
    ```
    For Linux/macOS:
    ```bash
        fyne release -icon 5.ico
    ```
