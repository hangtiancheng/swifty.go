NODE ?= node
COMMANDS := coverage test clean deps lark_cache lark_cache-v2 lark_http lark_orm lark_rpc lark_rpc-v2 server v1 v2

.PHONY: help $(COMMANDS)
help:
	$(NODE) main.js --help

$(COMMANDS):
	$(NODE) main.js $@

%:
	$(NODE) main.js $@

.PHONY: feat
feat:
	git add -A
	git commit -m "feat: Introduce new features"
	git push origin main

.PHONY: init
init:
	rm -rf ./.git
	git init
	git submodule add git@github.com:hangtiancheng/ai-agent.git
	git add -A
	git commit -m "Initial commit"
	git remote add origin git@github.com:hangtiancheng/lark-go.git
	git push origin main --set-upstream --force
