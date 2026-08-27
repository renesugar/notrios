# Notrios build targets. Run `make help` for a summary.
#
# Layout of generated output (all git-ignored):
#   bin/          notriosd, notriosctl, notrios binaries
#   web/dist/     production web assets (served by notriosd and the GUI)
#   web/node_modules/  frontend dependencies (make deps / make clobber)
#   _site/        documentation site with PageFind search
#   dist/         release archives from scripts/package_release.sh

.DEFAULT_GOAL := help

.PHONY: help deps build build-service build-cli web gui docs \
        test validate docaudit smoke serve doctor seed-help evidence-pre-push \
        clean clobber precheck

help: ## Show this target summary
	@grep -E '^[a-zA-Z-]+:.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  %-14s %s\n", $$1, $$2}'

deps: ## Install frontend dependencies from the lockfile (network)
	cd web && npm ci

web/node_modules:
	@echo "web/node_modules missing; installing from the lockfile (one-time)"
	cd web && npm ci

build: build-service build-cli ## Build notriosd and notriosctl into bin/

build-service: ## Build the headless service binary bin/notriosd
	mkdir -p bin
	go build -o bin/notriosd ./cmd/notriosd

build-cli: ## Build the CLI binary bin/notriosctl
	mkdir -p bin
	go build -o bin/notriosctl ./cmd/notriosctl

web: web/node_modules ## Build production web assets into web/dist/
	cd web && npm run build

gui: web ## Build the desktop GUI binary bin/notrios (needs libgtk-3-dev + libwebkit2gtk-4.1-dev)
	mkdir -p bin
	go build -tags "gui desktop production webkit2_41" -o bin/notrios ./cmd/notrios

docs: ## Build the documentation site into _site/ (network: npx marked + pagefind)
	bash scripts/build_docs_site.sh

test: ## Run all Go tests
	go test ./...

validate: ## Run tests plus scaffold/script validation
	bash scripts/validate-scaffold.sh

docaudit: web/node_modules ## Audit documentation/source anchors and coverage
	go run ./cmd/docaudit

smoke: web ## Run the end-to-end REST/MCP smoke test
	bash scripts/mvp_smoke.sh

evidence-pre-push: ## Verify the signed checkpoint and external ISO reserve
	bash scripts/verify_evidence_pre_push.sh

serve: ## Run the service from source on 127.0.0.1:8080
	go run ./cmd/notriosd -addr 127.0.0.1:8080

doctor: ## Run notriosctl doctor
	go run ./cmd/notriosctl doctor

seed-help: ## Mirror docs/ into the built-in Help notebook (default database)
	go run ./cmd/notriosctl seed-help docs

clean: ## Remove build/test/docs/release output (never user data in data/)
	rm -rf bin/ dist/ _site/ web/dist/ .playwright-mcp/
	rm -f coverage.out coverage.* notrios notriosd notriosctl
	rm -f notrios-*.zip
	find . -path ./web/node_modules -prune -o -type d -name __pycache__ -print | xargs -r rm -rf
	find . -path ./web/node_modules -prune -o -type f \( -name '*.pyc' -o -name '*.test' -o -name '*.prof' -o -name '*~' \) -print -exec rm -f {} +

clobber: clean ## clean plus remove frontend dependencies (web/node_modules)
	rm -rf web/node_modules/

precheck: ## Fail if the tree has uncommitted changes or tracked ignored artifacts
	@git status --short --untracked-files=all
	@tracked_ignored="$$(git ls-files -ci --exclude-standard)"; \
	if test -n "$$tracked_ignored"; then \
		echo "Tracked files matching .gitignore:"; \
		echo "$$tracked_ignored"; \
		exit 1; \
	fi
	@echo "precheck: no tracked ignored artifacts"
