.PHONY: test validate serve doctor

test:
	go test ./...

validate:
	bash scripts/validate-scaffold.sh

serve:
	go run ./cmd/notesd -addr 127.0.0.1:8080

doctor:
	go run ./cmd/notesctl doctor
