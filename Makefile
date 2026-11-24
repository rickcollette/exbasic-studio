.DEFAULT_GOAL := help

PYTHON ?= python
TASK ?= task
REMOTE ?= origin
REPO ?=
ARCH ?= amd64
REMOTE_TARGET ?= all
COMMIT_ALL ?= 0

REPO_ARG := $(if $(strip $(REPO)),--repo "$(REPO)",)
COMMIT_ARG := $(if $(filter 1 true yes,$(COMMIT_ALL)),--commit-all,)

.PHONY: help clean build check package dist release-check release release-draft release-publish

help: ## Show the available commands.
	@$(PYTHON) -c "print('EXBASIC Studio build commands\n\n  make clean            Remove generated build and distribution output\n  make build            Build the application\n  make check            Run the project validation suite\n  make package          Build local platform packages\n  make dist             Build Windows locally and Linux/macOS remotely into DIST\n  make release-check    Validate an existing DIST release without changing GitHub\n  make release          Build DIST, push the tag, and create a draft GitHub release\n  make release-publish  Build DIST, push the tag, and publish the GitHub release\n\nRelease options: REMOTE=origin REPO=owner/repository ARCH=amd64 REMOTE_TARGET=all\n                 COMMIT_ALL=1 (explicitly commit the complete worktree)')"

clean: ## Remove generated build and distribution output.
	$(PYTHON) scripts/clean.py

build: ## Build the application.
	$(TASK) build

check: ## Run the project validation suite.
	$(TASK) check

package: ## Build local platform packages.
	$(TASK) package

dist: ## Build and collect all platform release artifacts in DIST.
	$(TASK) package windows:package:portable ARCH="$(ARCH)"
	$(PYTHON) scripts/stage_windows_release.py --arch "$(ARCH)"
	$(PYTHON) scripts/remote_build.py --target "$(REMOTE_TARGET)"

release-check: ## Validate release metadata and DIST assets (dry run).
	$(PYTHON) scripts/publish_release.py --remote "$(REMOTE)" $(REPO_ARG) $(COMMIT_ARG)

release: ## Push the release tag and create a draft GitHub release.
	$(PYTHON) scripts/publish_release.py --source-only --remote "$(REMOTE)" $(REPO_ARG) $(COMMIT_ARG)
	$(MAKE) dist
	$(PYTHON) scripts/publish_release.py --execute --remote "$(REMOTE)" $(REPO_ARG) $(COMMIT_ARG)

release-draft: release ## Alias for release.

release-publish: ## Push the release tag and publish the GitHub release.
	$(PYTHON) scripts/publish_release.py --source-only --remote "$(REMOTE)" $(REPO_ARG) $(COMMIT_ARG)
	$(MAKE) dist
	$(PYTHON) scripts/publish_release.py --execute --publish-release --remote "$(REMOTE)" $(REPO_ARG) $(COMMIT_ARG)
