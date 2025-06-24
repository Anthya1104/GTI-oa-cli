# GTI-oa-CLI Monorepo Project

Welcome to the **GTI-oa-CLI Monorepo Project**!  
This repository contains multiple Command Line Interface (CLI) applications developed in Go, designed to showcase Go's capabilities in various domains, such as concurrent game simulation, distributed system consensus mechanisms, and data storage simulation.

## Project Structure

This Monorepo includes the following three main sub-projects:

- **math-game**: A CLI math game application that simulates a teacher posing questions and multiple students concurrently answering them. It emphasizes Go's concurrency model, graceful shutdown, and testability.
- **quorum-election**: A CLI application simulating a Quorum (Deductive Majority) mechanism in a distributed system, including inter-member heartbeat monitoring, fault detection, majority-vote-based member removal, and leader election. It aims to demonstrate liveness and consistency in a distributed system.
- **raid-simulator**: A RAID (Redundant Array of Independent Disks) simulator that demonstrates data handling, writing, reading, and data recovery behavior after disk failures for RAID0, RAID1, RAID10, RAID5, and RAID6.

> **Each sub-project contains its own `README.md` file** with more in-depth instructions and design considerations.

---

## How to Use

### Prerequisites

- Go 1.18 or higher.

### 🧭 Installation and Build

Clone and build all sub-projects:

```bash
git clone <Your project link, if any>
cd GTI-oa-cli
make all
```

This will generate executables for each sub-project and platform under the `bin/` directory.

#### Build Specific Projects

You can also build a specific sub-project for a specific platform:

```bash
make build-math-game-linux-amd64
make build-quorum-election-windows-amd64
make build-raid-simulator-darwin-arm64
```

---

### Makefile Targets

| Target                            | Description                               |
| --------------------------------- | ----------------------------------------- |
| init                              | Creates `bin/` and log directories        |
| build-math-game-<OS>-<ARCH>       | Builds the math-game CLI                  |
| build-quorum-election-<OS>-<ARCH> | Builds the quorum-election CLI            |
| build-raid-simulator-<OS>-<ARCH>  | Builds the raid-simulator CLI             |
| all                               | Builds all sub-projects for all platforms |
| clean                             | Deletes the `bin/` directory              |

---

### Running Applications

Examples:

```bash
# Math Game
./bin/math-game/math_game_app_linux_amd64 play --rounds 10

# Quorum Election
./bin/quorum-election/quorum_election_app_linux_amd64 play --members 5

# RAID Simulator
./bin/raid-simulator/raid_simulator_app_linux_amd64 raid --type raid5 --data "MySecretData"
```

---

### Running Tests

Run all tests:

```bash
go test ./...
```

---

## GitHub Releases

Each Release will include:

- Source code at that version
- Pre-built binaries for multiple platforms
- Tags for historical milestones

The tags marked the versions for each significant stage, especially versions prior to the **initial due date**.

---

## Project Dependencies

This project uses:

- `github.com/sirupsen/logrus`
- `github.com/spf13/cobra`
- `github.com/stretchr/testify/assert`
- `github.com/klauspost/reedsolomon`
