.PHONY: wasm clean server run

WASM_DIR=wasm_example
GO_ROOT=$(shell go env GOROOT)
GO_WASM_JS=$(GO_ROOT)/misc/wasm/wasm_exec.js
HOMEBREW_GO_WASM_JS=/opt/homebrew/Cellar/go/1.24.5/libexec/lib/wasm/wasm_exec.js

wasm:
	GOOS=js GOARCH=wasm go build -o $(WASM_DIR)/shine-mp3.wasm ./$(WASM_DIR)
	if [ -f "$(GO_WASM_JS)" ]; then \
		cp "$(GO_WASM_JS)" $(WASM_DIR)/; \
	elif [ -f "$(HOMEBREW_GO_WASM_JS)" ]; then \
		cp "$(HOMEBREW_GO_WASM_JS)" $(WASM_DIR)/; \
	else \
		echo "Could not find wasm_exec.js"; \
		exit 1; \
	fi

server:
	go build -o $(WASM_DIR)/server ./cmd/server

run: wasm server
	./$(WASM_DIR)/server

clean:
	rm -f $(WASM_DIR)/shine-mp3.wasm $(WASM_DIR)/wasm_exec.js $(WASM_DIR)/server
