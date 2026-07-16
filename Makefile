.PHONY: test validate serve doctor

test:
	go test ./...

validate:
	bash scripts/validate-scaffold.sh

serve:
	go run ./cmd/notriosd -addr 127.0.0.1:8080

doctor:
	go run ./cmd/notriosctl doctor

.PHONY: gui
gui: ## Build the notrios GUI binary (needs libgtk-3-dev + libwebkit2gtk dev headers)
	cd web && npm run build
	go build -tags "gui desktop production webkit2_41" -o bin/notrios ./cmd/notrios
