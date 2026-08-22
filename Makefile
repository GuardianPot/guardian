.PHONY: validate format format-check build test

validate:
	@task validate

format:
	@task format

format-check:
	@task format:check

build:
	@task build

test:
	@task test
