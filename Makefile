APP := easy-cmd

.PHONY: build test sync-shell-script

build:
	go build -o tmp/ ./cmd/easy-cmd

test:
	go test ./...

sync-shell-script:
	cp shell/easy-cmd.zsh cmd/easy-cmd/assets/easy-cmd.zsh

clean:
	rm ~/.local/bin/easy-cmd
	rm -rf ~/.local/share/easy-cmd
	rm ~/.easy-cmd/config.json
