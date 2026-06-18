PKGS := ./...
BIN := bin/agy-reader
CMD := .
GOFILES := $(shell find . -maxdepth 1 -name '*.go') $(shell find internal -name '*.go')

.PHONY: fmt vet test race build check clean install-skills

fmt:
	gofmt -w $(GOFILES)

vet:
	go vet $(PKGS)

test:
	go test $(PKGS)

race:
	go test -race $(PKGS)

build:
	go build -o $(BIN) $(CMD)

check: fmt vet test build

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
