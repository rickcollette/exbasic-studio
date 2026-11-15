# Extension API v1 — design scaffold

The starter **discovers manifest files only**. It deliberately DOES NOT load or execute WebAssembly.

Planned interfaces: `RegisterCommands`, `ReadWorkspaceFile` (scope-checked), `RegisterPanel`, `SubscribeBuild`, `ReadBuildArtifact`. Permissions are deny-by-default, host-owned and not inferred from a manifest alone.

Develop a WASI host, capability grants, UI panel schema, resource quotas, and an integration test harness before enabling execution.
