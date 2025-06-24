# project root path
MATH_GAME_DIR := math-game
QUORUM_ELECTION_DIR := quorum-election
RAID_SIMULATOR_DIR := raid-simulator
PROJECT_ROOT_DIR := github.com/Anthya1104

# config path
CONFIG_PATH := internal/config

# binary file path
BIN_DIR := bin

# build version var
VERSION ?= dev

# supported platforms
OS_LIST := windows darwin linux
ARCH_LIST := amd64 arm64

# # init directories
init:
	@mkdir -p $(BIN_DIR)


# math-game builds
define build_math-game
build-math-game-$(1)-$(2): init
	@echo "Building math-game for $(1)/$(2) (version: $(VERSION))..."
	@mkdir -p $(BIN_DIR)/$(MATH_GAME_DIR)/log
	cd $(MATH_GAME_DIR) && GOOS=$(1) GOARCH=$(2) go build -ldflags="-X '${PROJECT_ROOT_DIR}/$(MATH_GAME_DIR)-cli/${CONFIG_PATH}.Version=$(VERSION)'" -o ../$(BIN_DIR)/$(MATH_GAME_DIR)/math_game_app_$(1)_$(2)$(if $(filter windows,$(1)),.exe,) ./cmd/main.go
endef
$(foreach OS,$(OS_LIST),$(foreach ARCH,$(ARCH_LIST),$(eval $(call build_math-game,$(OS),$(ARCH)))))



# quorum-election builds
define build_quorum_election
build-quorum-election-$(1)-$(2): init
	@echo "Building quorum-election for $(1)/$(2) (version: $(VERSION))..."
	@mkdir -p $(BIN_DIR)/$(QUORUM_ELECTION_DIR)/log
	cd $(QUORUM_ELECTION_DIR) && GOOS=$(1) GOARCH=$(2) go build -ldflags="-X '${PROJECT_ROOT_DIR}/$(QUORUM_ELECTION_DIR)-cli/${CONFIG_PATH}.Version=$(VERSION)'" -o ../$(BIN_DIR)/$(QUORUM_ELECTION_DIR)/quorum_election_app_$(1)_$(2)$(if $(filter windows,$(1)),.exe,) ./cmd/main.go
endef
$(foreach OS,$(OS_LIST),$(foreach ARCH,$(ARCH_LIST),$(eval $(call build_quorum_election,$(OS),$(ARCH)))))


# raid-simulator builds
define build_raid_simulator
build-raid-simulator-$(1)-$(2): init
	@echo "Building raid-simulator for $(1)/$(2) (version: $(VERSION))..."
	@mkdir -p $(BIN_DIR)/$(RAID_SIMULATOR_DIR)/log
	cd $(RAID_SIMULATOR_DIR) && GOOS=$(1) GOARCH=$(2) go build -ldflags="-X '${PROJECT_ROOT_DIR}/$(RAID_SIMULATOR_DIR)-cli/${CONFIG_PATH}.Version=$(VERSION)'" -o ../$(BIN_DIR)/$(RAID_SIMULATOR_DIR)/raid_simulator_app_$(1)_$(2)$(if $(filter windows,$(1)),.exe,) ./cmd/main.go
endef
$(foreach OS,$(OS_LIST),$(foreach ARCH,$(ARCH_LIST),$(eval $(call build_raid_simulator,$(OS),$(ARCH)))))


# build all platforms for all projects
all: init
	$(foreach OS,$(OS_LIST),$(foreach ARCH,$(ARCH_LIST),$(MAKE) build-math-game-$(OS)-$(ARCH);))
	$(foreach OS,$(OS_LIST),$(foreach ARCH,$(ARCH_LIST),$(MAKE) build-quorum-election-$(OS)-$(ARCH);))
	$(foreach OS,$(OS_LIST),$(foreach ARCH,$(ARCH_LIST),$(MAKE) build-raid-simulator-$(OS)-$(ARCH);))

# clean bin dir
clean:
	rm -rf $(BIN_DIR)
