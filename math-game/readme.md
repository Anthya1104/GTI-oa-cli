# Math Game CLI Application

This is a CLI (Command Line Interface) math game application developed in Go. In this game, a teacher continuously poses questions, and multiple students concurrently answer them until a preset maximum number of rounds is reached. This project focuses on Go's concurrency model, graceful shutdown, and testability.

## 1. How to Use

### Prerequisites

- Go 1.18 or higher

### Installation and Build

#### 1. Clone the project:

```
git clone <Your project link, if any>
cd math-game-cli # Or your project's root directory
```

#### 2. Build the executable:

```
go build -o math_game_app cmd/main.go
```

### Running the Game

Execute the built application. You can specify the maximum number of rounds for the game using CLI flags (--rounds or -r).

- Run with default rounds (1 rounds):

```
./math_game_app
```

- Run with a specified number of rounds (e.g., 10 rounds):

```
./math_game_app --rounds 10
```

- Or shorthand

```
./math_game_app -r 10
```

### Running Tests

To run all tests in the project, execute the following command from the project root directory:

```
go test ./...
```

This will run tests in all packages.

## 2. Dependencies Credit

This project utilizes the following third-party Go modules:

`github.com/sirupsen/logrus`: A structured logging library for clear output of events and states during game play.

`github.com/spf13/cobra`: A powerful CLI application building library for handling command-line arguments and subcommands.

`github.com/stretchr/testify/assert`: A testing library providing rich assertion functions to simplify and strengthen test writing.

Thanks to these open-source projects for their contributions to the Go ecosystem!

## 3. Design and Architecture Choices

This project's design adheres to several core principles to ensure code modularity, maintainability, and scalability.

## Domain-Driven Design (DDD) Concepts

- **Ubiquitous Language:** Core terms used in the codebase, such as `Student`, `Teacher`, `Question`, `Operator`, `RoundResult`, `Game`, etc., directly map to the game's business domain. This ensures consistent communication among developers and between the code and business concepts.

- **Domain Model:** These core concepts are modeled as Go `structs` and interfaces, encapsulating not only data (e.g., student names, question parameters) but also behavior related to these concepts (e.g., operator application, student answering logic).

- **Entities:** `Student` and `Teacher` possess unique identifiers (e.g., `StudentID`) and maintain mutable states throughout their lifecycle, making them typical domain entities.

- **Value Objects:** `Operator` is a value object, its characteristics defined by its attributes without a unique identity. `AnswerEvent` and `RoundResult` also serve as value objects for conveying events and results.

- **Aggregates and Aggregate Root:** `Game` is designed as the aggregate root for this game domain. It is responsible for coordinating the overall game flow, such as generating questions, managing student behavior, and collecting round results. Any changes to the game's state should occur through the `Game` object to maintain business invariants.

- **Domain Services:** Methods like `Start()` and `PlayQuestion()` within the `Game` type, along with the `StudentActioner` interface, represent domain services that execute business logic spanning multiple entities or value objects.

## Object-Oriented Programming (OOP) in Go

While Go is not a traditional object-oriented language, it achieves core OOP principles through Composition and Interfaces:

- **Encapsulation:** Each `struct` encapsulates its internal data and related behaviors, reducing external direct dependencies.

- **Abstraction:** The `StudentActioner` interface defines a contract for student behavior, decoupling concrete behavior implementations (e.g., `DefaultStudentActioner`'s simulated thinking and answering) from the `Game`'s core logic. This significantly enhances code testability, allowing for injection of mock behaviors during testing.

- **Polymorphism:** Through the `StudentActioner` interface, the `Game` object can interact with different student behavior implementations at runtime without needing to know their underlying concrete types.

- **Composition:** `Teacher` and `Student` structs embed the `Player` struct to reuse common attributes. `Game` composes `Students` (slice), `Teacher`, and `StudentActioner` rather than inheriting, demonstrating Go's idiomatic design patterns.

## Go-idiomatic Concurrency Model

A core challenge of this project is handling concurrent game flow. We adhere to Go's concurrency philosophy: "Don't communicate by sharing memory; share memory by communicating."

- **goroutines:** Lightweight threads used for concurrent task execution. For example, teacher question generation, individual question round processing, and result collection all run independently in their respective `goroutines`.

- `channels`: Go's fundamental concurrency primitive for synchronized communication between `goroutines`.

  - `roundResultCh`: Used by multiple `PlayQuestion` goroutines to safely send round results to a single `collectResults` goroutine.

  - `gameDone`: As a `chan struct{}`, it's a zero-sized channel specifically used to signal that an "event has occurred," clearly indicating the completion of all internal game operations.

- `context.Context`: This is Go's standard pattern for managing `goroutine` lifecycle and propagating cancellation signals. In the game, the root `Context` is used for application graceful shutdown, and each question round also derives its own child `Context`, ensuring student goroutines can respond to round completion or cancellation.

- `sync.WaitGroup`: Used to wait for a collection of `goroutines` to finish.

  - `Game.roundsWg`: Tracks the completion of each PlayQuestion goroutine.

  - `gameMainWg` (internal): Tracks the completion of the two main `goroutines` launched by Game.Start (question generation and result collection), ultimately leading to the closing of the gameDone channel.

  - `studentWg` (internal to each round): Tracks the completion of all student `goroutines` within that round, ensuring `answerCh` can be safely closed.

This combined usage ensures the game flow is non-blocking, resources are correctly released, and `goroutine` leaks are effectively prevented.

## 4. Future Optimization Directions

The project's design considers future expansion and optimization possibilities. Below are several areas for potential improvement:

### a. Test Performance Optimization: Time Mocking for Tests

- **Current State & Challenge:** Current integration tests (e.g., `TestGamePlay_MultipleRoundFlow`) directly rely on real-time delays within the game logic (e.g., teacher posing questions every second). This results in longer test execution times, impacting development iteration speed.

- **Optimization Direction:** Consider abstracting `Game`'s direct dependency on the `time` package into an interface (e.g., a `Timer` interface). In production, use a real implementation of the `Timer` interface that leverages `time.After`. In tests, inject a mock `MockTimer`. A `MockTimer` allows test code to manually "advance" time, immediately triggering time-dependent operations, thereby significantly reducing test execution time while maintaining test determinism and reliability.

### b. Mathematical Operation Precision

- **Current State & Challenge:** Currently, mathematical operations within questions primarily use `int` types. Specifically, division (`/`) performs integer division. While this runs without programming errors, it may not align with the expected results of real-world floating-point division (e.g., `80 / 88 = 0`).

- **Optimization Direction:** Consider introducing floating-point types (`float64`) or specialized arbitrary-precision math libraries (such as `shopspring/decimal`) to handle problems involving division, ensuring the mathematical accuracy of calculation results. This may require adjustments to the question generation logic and answer validation logic.

### c. More Granular Error Handling and Custom Error Types

- **Current State & Challenge:** Current error handling primarily involves logging errors via `logrus.Errorf` and, in some cases, simply `continueing` or `returning`.

- **Optimization Direction:** Introduce Custom Error Types to more explicitly indicate the nature of different errors (e.g., `ErrDivideByZero`, `ErrInvalidOperator`, `ErrRoundTimeout`). This enables errors to be propagated, caught, and handled more precisely throughout the call stack, enhancing code robustness and debuggability.

### d. External Configuration Management

- **Current State & Challenge:** Game settings (such as `MaxRounds`, number of students, and thinking time ranges) are currently hardcoded or controlled via CLI flags.

- **Optimization Direction:** Implement a more robust external configuration management mechanism (e.g., using YAML/JSON files or environment variables). This would allow modifying game behavior without recompiling the application, improving deployment flexibility and configurability.

### e. Game State Observability

**Current State & Challenge:** Log output is currently the primary means of observing game progress.

**Optimization Direction:** Consider adding more structured logging (e.g., using `logrus.Fields` to add contextual information) or incorporating simple metrics (e.g., using a Prometheus client) to track key game indicators (such as round completion time, correct answer rate, and each student's score). This would facilitate better monitoring of game runtime health.

## 5. Key Changes with Extended Time

Initial project time constraints made `goroutine` lifecycle management a potential risk. Expanding to `Bonus i` (multiple students answering simultaneously) or even `Bonus ii` (teacher continuously posing questions, students concurrently answering multiple problems) with the original design could have led to unmanageable complexity and resource leaks in `goroutine` management.

Therefore, the extended time was invested in critical low-level refactoring and strengthening of the concurrency model, laying a solid foundation for future expansion:

- ### a. Refactor Goroutines to be Context-Based:

  - **Change:** Introduced `context.Context` as the core mechanism for propagating cancellation signals among `goroutines`. All new `goroutines` (e.g., student answering, question processing) now receive a `Context` and check `ctx.Done()`, ensuring they can respond to cancellation signals and exit gracefully.

  - **Benefit:** This is fundamental for achieving graceful shutdown and preventing `goroutine` leaks. Whether it's application termination (`Ctrl+C`) or the end of a single question round, associated `goroutines` can terminate promptly, releasing resources.

- ### b. Clarify Roles: Teacher vs. Game Round Coordination:

  - **Change:** The `Teacher`'s role shifted from "sequentially waiting for each student's answer" to that of a "question publisher." The `Game` object became the true "game flow coordinator." `Game.Start` now launches independent `goroutines` internally to concurrently handle each question round, instead of sequentially calling `PlayRound` as in the older version.

  - **Benefit:** Clear separation of concerns leads to more modular code that is easier to understand and maintain. The teacher is no longer blocked and can continuously pose questions, fulfilling the requirements of `Bonus ii`.

- ### c. Ensure Proper Goroutine Lifecycle Management:

  - **Change:** Extensive use of `sync.WaitGroup` and `channel` combinations was implemented to precisely coordinate `goroutine` lifecycles. For instance, `gameMainWg` tracks core game `goroutines`, `roundsWg` tracks each question round's `goroutine`, and `studentWg` tracks student `goroutines` within each round. The `gameDone` channel is used to signal to the `main` function that the entire game has naturally finished.

  - **Benefit:** This ensures that even in a complex concurrent environment, all launched `goroutines` are correctly counted, awaited for their completion, and triggered to exit via `Context` or channel closing when necessary. This effectively addresses potential `goroutine` leaks and enhances program stability.

- ### d. Foundation for Bonus I/II Expansion:

  - **Change:** It was precisely the `Context`-based management, role redefinition, and precise `goroutine` lifecycle management mentioned above that enabled the robust implementation of the complex concurrent logic for `Bonus i` (multiple students racing to answer, first correct answer wins) and `Bonus ii` (teacher continuously poses questions, students concurrently answer multiple problems).

  - **Benefit:** Demonstrates that the upfront investment in fundamental architectural refactoring was worthwhile, providing stable and reliable underlying support for future, more complex feature expansions.
