BINARY    := wifi-attendance
APP       := WiFiAttendance.app
BUNDLE    := $(APP)/Contents
MACOS_DIR := $(BUNDLE)/MacOS
RES_DIR   := $(BUNDLE)/Resources

.PHONY: all build app install run clean

all: app

build:
	go build -ldflags="-s -w" -o $(BINARY) .

app: build
	mkdir -p $(MACOS_DIR) $(RES_DIR)
	cp $(BINARY) $(MACOS_DIR)/$(BINARY)
	cp -r assets $(MACOS_DIR)/assets
	cp Info.plist $(BUNDLE)/Info.plist
	@echo "Built $(APP)"

install: app
	pkill wifi-attendance 2>/dev/null; true
	rm -rf /Applications/$(APP)
	cp -r $(APP) /Applications/$(APP)
	@echo "Installed to /Applications/$(APP)"

run: app
	open $(APP)

clean:
	rm -rf $(BINARY) $(APP)
