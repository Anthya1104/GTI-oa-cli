# project root path
MATH_GAME_DIR := math-game
QUORUM_ELECTION_DIR := quorum-election
RAID_SIMULATOR_DIR := raid-simulator

# config path
MATH_GAME_CONFIG_PATH := github.com/Anthya1104/math-game-cli/internal/config

# binary file path
BIN_DIR := bin

# build version var
VERSION ?= dev

.PHONY: all clean build-math-game build-quorum-election build-raid-simulator

# create bin dir
init:
	@mkdir -p $(BIN_DIR)/$(MATH_GAME_DIR)/log
	@mkdir -p $(BIN_DIR)/$(QUORUM_ELECTION_DIR)/log
	@mkdir -p $(BIN_DIR)/$(RAID_SIMULATOR_DIR)/log	

# build each project seperately
build-math-game: init
	@echo "Building math-game (version: $(VERSION))..."
	cd $(MATH_GAME_DIR) && go build -ldflags="-X '${MATH_GAME_CONFIG_PATH}.Version=$(VERSION)'" -o ../$(BIN_DIR)/$(MATH_GAME_DIR)/math_game_app ./cmd/main.go

build-quorum-election: init
	@echo "Building quorum-election (version: $(VERSION))..."
	cd $(QUORUM_ELECTION_DIR) && go build -ldflags="-X 'main.Version=$(VERSION)'" -o ../$(BIN_DIR)/$(QUORUM_ELECTION_DIR)/quorum_election_app ./cmd/main.go

build-raid-simulator: init
	@echo "Building raid-simulator (version: $(VERSION))..."
	cd $(RAID_SIMULATOR_DIR) && go build -ldflags="-X 'main.Version=$(VERSION)'" -o ../$(BIN_DIR)/$(RAID_SIMULATOR_DIR)/raid_simulator_app ./cmd/main.go

# build all
all: build-math-game build-quorum-election build-raid-simulator

# clean bin dir
clean:
	rm -rf $(BIN_DIR)
