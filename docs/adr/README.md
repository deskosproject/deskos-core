# Architecture decision records

Every decision behind DeskOS, oldest first. Each ADR is short (context, decision,
consequences), and where another page disagrees with one, the ADR wins.

The number of public kinds grew over time, so some ADR *titles* say "the tenth
kind" while the API now has **fifteen**; the count is in
[resources.md](../resources.md#resource-kinds).

| ADR | Status |
|---|---|
| [0001: Artifact factory, not configuration management](0001-artifact-factory-not-configuration-management.md) | accepted |
| [0002: Declarative resources and a typed IR](0002-resource-model-and-typed-ir.md) | accepted |
| [0003: Explicit composition precedence](0003-explicit-composition-precedence.md) | accepted |
| [0004: DeskOS Core is platform-independent intent](0004-core-is-platform-independent.md) | accepted |
| [0005: Managed baseline boundary](0005-managed-baseline-boundary.md) | accepted |
| [0006: BootProfile as the tenth public kind](0006-bootprofile-tenth-kind.md) | accepted |
| [0007: UpdatePolicy as the eleventh public kind](0007-updatepolicy.md) | accepted |
| [0008: GnomeProfile shell, clock and keyboard settings](0008-gnome-shell-clock-keyboard.md) | accepted |
| [0009: TrustAnchor as the twelfth public kind](0009-trustanchor.md) | accepted |
| [0010: Artifact records and the supply-chain check](0010-artifact-evidence-and-supply-chain.md) | accepted |
| [0011: The agentic contribution layer](0011-agentic-contribution-layer.md) | accepted |
| [0012: Vulnerability-severity and VEX policy](0012-vulnerability-severity-and-vex-policy.md) | accepted |
| [0013: Theme as the thirteenth public kind](0013-theme.md) | accepted |
| [0014: Platform base ownership and support tiers](0014-platform-base-ownership.md) | accepted |
| [0015: A second package manager, and where the package database lives](0015-package-manager-and-database.md) | accepted |
| [0016: A platform that keeps its vendor's desktop, and no assumed app menu](0016-vendor-desktop-and-app-menu.md) | accepted |
| [0017: Extensions a profile may turn on, a Flatpak remote a workstation may retire, and the power button](0017-extensions-flatpak-remote-and-power-button.md) | accepted |
| [0018: `deskos`, the runtime companion to `deskosctl`](0018-the-runtime-companion.md) | accepted |
| [0019: A platform says whether its base can fetch](0019-a-platform-says-whether-its-base-can-fetch.md) | accepted |
| [0020: Ubuntu lives in its own repository](0020-ubuntu-lives-in-its-own-repository.md) | accepted |
| [0021: An extension the platform's repositories do not carry as a pinned artifact](0021-extensions-can-be-delivered-as-pinned-artifacts.md) | accepted |
| [0022: Removing packages a platform must not carry](0022-removing-packages-a-platform-must-not-carry.md) | accepted |
| [0023: Scheduled image apply is an explicit, time-bound opt-in](0023-scheduled-apply-window.md) | accepted |
| [0024: What a desktop platform must expose](0024-what-a-desktop-platform-must-expose.md) | proposed |
| [0025: The terminal font is the platform's fact](0025-the-terminal-font-is-the-platforms-fact.md) | accepted |
| [0026: The families are the workstation's choice, not the platform's](0026-the-families-are-the-workstations-choice.md) | accepted |
| [0027: A network profile is a typed resource, and its secrets are not](0027-a-network-profile-is-typed-and-its-secrets-are-not.md) | accepted |
