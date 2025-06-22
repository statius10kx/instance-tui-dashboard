# Multi-Instance TUI Dashboard Boilerplate

A lightweight terminal dashboard for spinning up and watching dozens of spam-bot instances at once. Built with Go and Bubble Tea. Uses dummy data.

## Why this exists
My original Python script handled a few testnet spammers, but scaling to 100 instances was messy. Rewriting the workers plus a tiny TUI in Go keeps everything in one terminal window.

## What’s under the hood
I kept the dependency list short:

* **Bubble Tea** – drives the TUI’s event loop and state machine.  
* **Lipgloss** – handles colours, layout, and generally makes the UI look less like `top`.  
* **Clipboard** – lets you copy data out of the dashboard without leaving the terminal.

Requires **Go 1.24 +**.

## Quick start
~~~bash
git clone https://github.com/<your-user>/instance-tui.git
cd instance-tui
go run main.go
~~~

Build a standalone binary:
~~~bash
go build -o instance_tui .
./instance_tui
~~~
Press Q to quit.
