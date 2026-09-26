PKGS := ./...
BIN := bin/agy-reader
CMD := .
PREFIX ?= /usr/local
GOFILES := $(shell find . -maxdepth 1 -name '*.go') $(shell find internal -name '*.go')

.PHONY: fmt vet test maintenance-test race build check clean install install-skills

fmt:
	gofmt -w $(GOFILES)

vet:
	go vet $(PKGS)

test:
	go test $(PKGS)

maintenance-test:
	python3 -B -m unittest discover -s scripts -p 'test_*.py'

race:
	go test -race $(PKGS)

build:
	go build -o $(BIN) $(CMD)

install: build
	install -d "$(PREFIX)/bin"
	install -m 755 "$(BIN)" "$(PREFIX)/bin/agy-reader"

check: fmt vet test maintenance-test build

clean:
	rm -rf bin

# Install first-party skills (tracked under skills/) into the agent skill dirs
# as relative symlinks. Those dirs (.claude/skills, .agents/skills) are
# gitignored install targets; skills/ is the source of truth. Idempotent.
install-skills:
	@for d in skills/*/; do \
		name=$$(basename "$$d"); \
		for agent in .claude .agents; do \
			mkdir -p "$$agent/skills"; \
			ln -sfn "../../skills/$$name" "$$agent/skills/$$name"; \
			echo "linked $$agent/skills/$$name -> ../../skills/$$name"; \
		done; \
	done
