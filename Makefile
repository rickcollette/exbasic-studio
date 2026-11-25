.DEFAULT_GOAL := help

PYTHON ?= python
TASK ?= task
REMOTE ?= origin
REPO ?=
ARCH ?= amd64
REMOTE_TARGET ?= all
COMMIT_ALL ?= 1
RELEASE_TYPE ?= fix

REPO_ARG := $(if $(strip $(REPO)),--repo "$(REPO)",)
COMMIT_ARG := $(if $(filter 1 true yes,$(COMMIT_ALL)),--commit-all,)

.PHONY: help clean build check package dist release-check release release-draft release-feature release-publish

help: ## Show the available commands.
	@$(PYTHON) -c "print('EXBASIC Studio build commands\n\n  make clean            Remove generated build and distribution output\n  make build            Build the application\n  make check            Run the project validation suite\n  make package          Build local platform packages\n  make dist             Build Windows locally and Linux/macOS remotely into DIST\n  make release-check    Validate an existing DIST release without changing GitHub\n  make release          Commit changes, increment the fix build, build DIST, and draft\n  make release-feature  Commit changes, increment the minor version, build, and draft\n  make release-publish  Publish the current draft without incrementing its version\n\nRelease options: REMOTE=origin REPO=owner/repository ARCH=amd64 REMOTE_TARGET=all\n                 RELEASE_TYPE=fix|feature COMMIT_ALL=0 (require a clean worktree)')"

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
	$(PYTHON) scripts/bump_version.py --kind "$(RELEASE_TYPE)" --commit --remote "$(REMOTE)" $(COMMIT_ARG)
	$(PYTHON) scripts/publish_release.py --source-only --remote "$(REMOTE)" $(REPO_ARG) $(COMMIT_ARG)
	$(MAKE) dist
	$(PYTHON) scripts/publish_release.py --execute --remote "$(REMOTE)" $(REPO_ARG) $(COMMIT_ARG)

release-draft: release ## Alias for release.

release-feature: ## Increment the minor version and create a draft release.
	$(MAKE) release RELEASE_TYPE=feature

release-publish: ## Publish the current draft without changing its version.
	$(PYTHON) scripts/publish_release.py --execute --publish-release --remote "$(REMOTE)" $(REPO_ARG) $(COMMIT_ARG)
