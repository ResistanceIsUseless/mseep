# mseep Development TODO List

## Priority 1 - Core Functionality
- [ ] Implement health check package infrastructure
- [ ] Implement health command using health check package

## Priority 2 - User Interface
- [ ] Add bubbletea dependency for TUI
- [ ] Implement basic TUI with server list view
- [ ] Add profile management commands (list, create, delete, apply)

## Priority 3 - Health Checks
- [ ] Implement stdio health check type
- [ ] Implement http health check type
- [ ] Implement tcp health check type

## Priority 4 - Extended Features
- [ ] Add test suite with unit tests for core packages
- [ ] Add Cursor client adapter
- [ ] Improve fuzzy matching with user prompts for ambiguous matches

## Completed
- [x] Implement status command to show client and server status
- [x] Implement apply command with diff preview
- [x] Create proper unified diff implementation
- [x] Add Charm libraries (lipgloss, glamour, etc) for beautiful CLI output
- [x] Save todo list to file for persistence