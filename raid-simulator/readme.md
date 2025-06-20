# RAID Simulator Project

This project is a RAID (Redundant Array of Independent Disks) simulator developed in Go. It aims to demonstrate the basic data handling, writing, reading, and data recovery behavior after disk failures for various RAID levels (RAID0, RAID1, RAID10, RAID5, and RAID6).

## 1. Project Scope

This simulator provides a simplified RAID system model that can:

- **Simulate Multiple Disks:** Store data in Go byte slices to represent disk data blocks.

- **Support Various RAID Levels:** Currently implements the basic logic for RAID0 (striping), RAID1 (mirroring), RAID10 (striping of mirrors), RAID5 (striping with rotating parity), and RAID6 (striping with dual parity).

- **Write Operations:** Write input data to the simulated RAID array, handling striping, mirroring, and parity calculations. It supports partial writes from any logical offset (via Read-Modify-Write, RMW).

- **Read Operations:** Read data from the simulated RAID array, capable of reading a specified length of data from any logical offset.

- **Disk Failure Simulation:** Allows clearing data on a specified disk to simulate a disk failure.

- **Data Recovery:**

  - For RAID1 and RAID10: Able to read data from a mirrored disk in case of a single disk failure.

  - For RAID5: Able to reconstruct and recover data using parity checksums in case of a single disk failure.

  - For RAID6: Able to reconstruct and recover data using dual parity checksums in case of up to two simultaneous disk failures.

- **Command Line Interface (CLI):** Provides a simple CLI using the Cobra framework to run simulations.

## 2. Credits

This project uses the following Go language libraries:

- `github.com/spf13/cobra`: Used for building powerful and flexible command-line interfaces.

- `github.com/sirupsen/logrus`: A structured, pluggable logging library for Go, used for outputting information and warnings during the simulation process.

- `github.com/stretchr/testify/assert`: A Go testing toolkit used for writing concise and expressive unit tests.

- `github.com/klauspost/reedsolomon`: An efficient Reed-Solomon encoding/decoding library, used for parity calculation and data reconstruction in RAID5 and RAID6. This greatly simplifies the complexity of parity mathematical operations.

## 3. How to Use (CLI Flags)

To run the RAID simulation, you can use the command-line interface provided by the project.

Command Structure:

```
go run main.go raid --type <RAID_TYPE> --data <INPUT_DATA>
```

### Parameter Description:

- `--type <RAID_TYPE>`: Specifies the RAID level to simulate. Currently
  supported values include:

  - `raid0`

  - `raid1`

  - `raid10`

  - `raid5`

  - `raid6`

- `--data <INPUT_DATA>`: The string data to write into the RAID array.

### Examples:

Run RAID0 simulation:

```
go run main.go raid --type raid0 --data "HelloRAID0World"
```

Run RAID1 simulation:

```
go run main.go raid --type raid1 --data "MirrorMirrorOnTheWall"
```

Run RAID10 simulation:

```
go run main.go raid --type raid10 --data "RAID10IsFastAndSafe"
```

Run RAID5 simulation:

```
go run main.go raid --type raid5 --data "RAID5WithParityProtection"
```

Run RAID6 simulation:

```
go run main.go raid --type raid6 --data "RAID6DoubleFaultTolerant"
```

Version Information:

You can also check the application's version information:

```
go run main.go version
```

## 4. TODO List (Expansion Directions)

This project is a basic simulator, and it can be expanded and improved in the following directions in the future:

### a. Modular Simulation Flow and CLI Flag Splitting:

- Currently, the `RunRAIDSimulation` function binds write, fault, and read operations together. These operations can be split into independent CLI flags (e.g., `write`, `read`, `clear-disk`, `rebuild-disk`), allowing users more flexibility to control simulation steps.
- Example:

  `bash go run main.go raid write --type raid5 --data "MySecretData" --offset 0 go run main.go raid clear-disk --type raid5 --disk 1 go run main.go raid read --type raid5 --start 0 --length 10 go run main.go raid rebuild-disk --type raid5 --disk 1`

- Expansion Direction: Consider introducing a data persistence layer, such as using Redis, to serialize and store the simulation state (e.g., disk data) after each operation. This way, different CLI commands can maintain state without needing to reinitialize the entire RAID array for each run.

### b. Expand RAID Levels:

- Although RAID5 and RAID6 already use `klauspost/reedsolomon`, their read/write logic can be further expanded and refined, especially for RAID6 parity distribution strategies (e.g., diagonal parity).
- Consider adding other less common but educationally significant RAID levels, such as RAID2, RAID3, RAID4, etc.

### c. Performance Simulation:

- Introduce parameters like I/O latency, disk RPM, seek time, etc., to simulate the actual performance of different RAID levels during read/write operations.
- Provide reports on throughput and IOPS (I/O Operations Per Second).

### d. Visualization Tools:

- Develop a simple graphical user interface (GUI) or a terminal-based visualization tool to more intuitively display data distribution on disks, parity block locations, and the reconstruction process after a failure.

### e. More Robust Error Handling and Logging:

- Enhance the clarity of error messages and provide more troubleshooting tips.
- Implement different logging levels (DEBUG, INFO, WARN, ERROR) to better track the simulation process.

### f. Persistent Simulation State:

- In addition to the Redis consideration mentioned above, the current state of the RAID array can be serialized to a file system (e.g., JSON or Protobuf) to allow loading previous simulation progress.

### g. More Complex Failure Scenarios:

- Simulate multiple disk failures at different times and recovery scenarios for consecutive failures.
- Introduce more granular failure types such as disk write errors and bad blocks.

### h. Data Integrity Verification:

- Automatically run background verification after each read/write operation to ensure data and parity consistency.

This RAID Simulator project provides a solid foundation for understanding the principles of RAID operation and has much potential for further expansion and improvement.
