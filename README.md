# depup

CLI tool that shows how long ago each dependency in a project was last updated, by walking the git history of lock/manifest files.

## Supported package managers

- Go (`go.sum` / `go.mod`)
- npm (`package-lock.json` / `package.json`)
- Yarn (`yarn.lock`)
- Composer (`composer.lock`)
- Maven (`pom.xml`)
- Docker (`Dockerfile`)
- uv (`uv.lock`)

## Requirements

- Go 1.25+
- A git repository with committed dependency files

## Usage

```sh
# Auto-detect all supported dependency files
depup auto <directory>

# Analyze a specific package manager
depup gomod <directory>
depup npm <directory>
depup composer <directory>

# JSON output
depup auto -f json <directory>

# Write to file
depup auto -o report.txt <directory>
```

## Build

```sh
go build -o depup .
```

## Run without building

```sh
go run . auto <directory>
```
