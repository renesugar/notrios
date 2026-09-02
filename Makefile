# Notrios build targets. Run `make help` for a summary.
#
# Layout of generated output (all git-ignored):
#   bin/          notriosd, notriosctl, notrios binaries
#   web/dist/     production web assets (served by notriosd and the GUI)
#   web/node_modules/  frontend dependencies (make deps / make clobber)
#   docs-site/node_modules/  pinned Pagefind build dependency
#   _site/        documentation site with PageFind search
#   dist/         release archives from scripts/package_release.sh

.DEFAULT_GOAL := help

.PHONY: help deps build build-service build-cli web gui docs \
        test validate docgen docaudit doccheck smoke serve doctor seed-help evidence-pre-push \
        g18e-validate g18f-validate g18g-validate g19-validate g20-validate \
        install install-dry-run icons deb integration-matrix uninstall uninstall-dry-run purge clean clobber precheck

help: ## Show this target summary
	@grep -E '^[a-zA-Z-]+:.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "  %-14s %s\n", $$1, $$2}'

deps: ## Install frontend dependencies from the lockfile (network)
	cd web && npm ci

web/node_modules:
	@echo "web/node_modules missing; installing from the lockfile (one-time)"
	cd web && npm ci

docs-site/node_modules:
	@echo "docs-site/node_modules missing; installing pinned Pagefind (one-time)"
	npm ci --prefix docs-site

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

.PHONY: abi
## abi: build the version-1 C ABI shared library and run the C host acceptance test
abi:
	sh cmd/notrioslib/run_host_test.sh

docs: docs-site/node_modules ## Build the pinned offline Hugo/Ledger site into _site/
	bash scripts/build_docs_site.sh

test: ## Run all Go tests
	go test ./...

validate: ## Run tests plus scaffold/script validation
	bash scripts/validate-scaffold.sh

docgen: ## Check committed user/API documentation against source templates
	go run ./cmd/docgen --user --api --check

docaudit: web/node_modules docgen ## Audit documentation/source anchors and coverage
	go run ./cmd/docaudit

doccheck: ## Run maintainer-only local advisory prose review (needs llama-server)
	go run ./cmd/doccheck --endpoint http://127.0.0.1:8081 --triage performance/v0.7-g18f/TRIAGE.json --output performance/v0.7-g18f/ADVISORY_REPORT.json --repeats 2 --temperature 0.1

smoke: web ## Run the end-to-end REST/MCP smoke test
	bash scripts/mvp_smoke.sh

evidence-pre-push: ## Verify the signed checkpoint and external ISO reserve
	bash scripts/verify_evidence_pre_push.sh

g18e-validate: ## Validate the G18e GUI journey manifest and evidence
	python3 -m unittest discover -s performance/v0.7-g18e -p 'test_*.py'
	python3 performance/v0.7-g18e/validate_evidence.py
	GOCACHE="$${GOCACHE:-/tmp/notrios-g18e-gocache}" go run ./cmd/docjourney

g18f-validate: docgen ## Validate G18f generation and committed advisory evidence (no model)
	python3 -m unittest discover -s performance/v0.7-g18f -p 'test_*.py'
	python3 performance/v0.7-g18f/validate_evidence.py

g18g-validate: docs-site/node_modules docgen ## Build and validate pinned Hugo/Ledger docs
	bash scripts/build_docs_site.sh _site
	python3 -m unittest discover -s performance/v0.7-g18g -p 'test_*.py'
	python3 performance/v0.7-g18g/validate_evidence.py --site _site

g19-validate: ## Validate published archive-v2 schemas, goldens, and reader matrix
	GOCACHE="$${GOCACHE:-/tmp/notrios-g19-gocache}" go test ./internal/archivev2 ./cmd/notriosctl -run 'Compatibility|PublicGolden|PhysicalRefusal|DeclarationProbe|PublishedJSON|PublishedContract|PublishedSyncWire' -count=1
	python3 performance/v0.7-g19/validate_evidence.py

g20-validate: ## Validate v0.7 release identity, security boundaries, evidence, and licenses
	python3 -m unittest discover -s performance/v0.7-g20 -p 'test_*.py'
	python3 performance/v0.7-g20/check_dependency_licenses.py
	python3 performance/v0.7-g20/validate_evidence.py
	GOCACHE="$${GOCACHE:-/tmp/notrios-g20-gocache}" go test ./internal/service ./internal/httpapi ./internal/archive ./internal/synccarrier -count=1
	python3 performance/v0.7-g8/validate_evidence.py
	python3 performance/v0.7-g14e/validate_evidence.py
	python3 performance/v0.7-g17/validate_evidence.py

serve: ## Run the service from source on 127.0.0.1:8099 (the development address)
	go run ./cmd/notriosd -addr 127.0.0.1:8099

doctor: ## Run notriosctl doctor
	go run ./cmd/notriosctl doctor

seed-help: ## Mirror docs/ into the built-in Help notebook (default database)
	go run ./cmd/notriosctl seed-help docs

install: build web ## Install to an end-user location (prefix=$HOME/.local by default)
	python3 scripts/lifecycle.py install

install-dry-run: build web ## Show exactly what install would write, and write nothing
	python3 scripts/lifecycle.py install --dry-run

integration-matrix: build web ## Run the installed-integration matrix (H8) and record results
	python3 scripts/integration_matrix.py

icons: ## Regenerate the desktop and web icons from assets/notrios.png
	python3 scripts/build_icons.py

deb: build web ## Build the internal Ubuntu package into dist/deb (needs dpkg-dev)
	bash scripts/build_deb.sh

uninstall: ## Remove what install recorded installing; never touches user data
	python3 scripts/lifecycle.py uninstall

uninstall-dry-run: ## Show exactly what uninstall would remove, and remove nothing
	python3 scripts/lifecycle.py uninstall --dry-run

purge: ## Uninstall AND delete this user's Notrios data, after a verified backup
	python3 scripts/lifecycle.py purge

clean: ## Remove build/test/docs/release output (never user data in data/)
	rm -rf bin/ dist/ _site/ web/dist/ .playwright-mcp/
	rm -f coverage.out coverage.* notrios notriosd notriosctl
	rm -f notrios-*.zip
	find . -path ./web/node_modules -prune -o -type d -name __pycache__ -print | xargs -r rm -rf
	find . -path ./web/node_modules -prune -o -type f \( -name '*.pyc' -o -name '*.test' -o -name '*.prof' -o -name '*~' \) -print -exec rm -f {} +

clobber: clean ## clean plus remove UI/docs build dependencies
	rm -rf web/node_modules/ docs-site/node_modules/

precheck: ## Fail if the tree has uncommitted changes or tracked ignored artifacts
	@git status --short --untracked-files=all
	@tracked_ignored="$$(git ls-files -ci --exclude-standard)"; \
	if test -n "$$tracked_ignored"; then \
		echo "Tracked files matching .gitignore:"; \
		echo "$$tracked_ignored"; \
		exit 1; \
	fi
	@echo "precheck: no tracked ignored artifacts"
