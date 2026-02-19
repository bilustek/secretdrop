## Overview

This project implements a secure text sharing API for web, mobile and desktop apps.

## Technologies Used

- React and TypeScript for web frontend
- Go for backend API server (go1.26.0)
- golangci-lint v2 for Go linting

## Project Structure

```
secretdrop/
├── backend/           # Go API server
│   ├── go.mod
│   ├── main.go
│   └── .golangci.yml
├── frontend/          # React/TypeScript (TBD)
├── .gitignore
├── .pre-commit-config.yaml
└── CLAUDE.md
```

## Running the Backend

```bash
cd backend
go run .              # starts server on :8080
```

Test with:
```bash
curl http://localhost:8080/
```

## Development Commands

```bash
# Build
cd backend && go build ./...

# Lint
cd backend && golangci-lint run ./...

# Format
cd backend && golangci-lint fmt ./...

# Test
cd backend && go test -race ./...
```

## Development Process

1. Before implementing, use DeepWiki's Ask Question tool to get latest
   information so that you write the code as per the latest library updates.
