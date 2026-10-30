# What we watch upstream, and when it forces a change

DeskOS inherits defaults from a base image, a desktop and a set of packages. This
page lists the sources that can change under us, what each one can break here, the
check that would catch it, and the threshold that turns a change into work. "No
action" is a valid outcome and the common one.

The recent cases are the reason the page exists: GNOME moved its default family to
Adwaita Sans while EL10 still shipped Cantarell and RHEL overrode it to Red Hat
Text; Ptyxis replaced GNOME Terminal and kept its font on a schema of its own;
`ostreecontainer` gained a `bootc` successor. Each was found by reading, not by a
check, and one of them shipped as a gap.

## Sources

| Source | What can change | Caught by | Threshold for action |
|---|---|---|---|
| [`gsettings-desktop-schemas`](https://gitlab.gnome.org/GNOME/gsettings-desktop-schemas) | every GNOME default DeskOS mirrors: families, sizes, monospace, text scaling, rendering | the session check reads the plan's defaults back; a changed schema default does not fail it | a key DeskOS writes changes its default, its range or disappears: re-measure and update the ADR and the reference |
| [GNOME HIG](https://developer.gnome.org/hig/) | the design guidance the image follows: typography, palette, accessibility, notifications | nothing; it is read | a page listed in [design/gnome-hig.md](gnome-hig.md) changes its recommendation: re-open the field it maps to |
| [GNOME Shell and its extension API](https://gitlab.gnome.org/GNOME/gnome-shell) | the shell version an extension declares, and the shell's own settings | `vm-bootcheck` loads the extensions and reads `gnome-extensions info` | a shell major falls outside an extension's `shell-version`: pin a newer artifact or drop the extension (ADR 0021) |
| [Ptyxis](https://gitlab.gnome.org/chergert/ptyxis) | the terminal's schema path and font keys, which the platform declares | the session check reads `appearance.fonts.terminal` back through the platform's fact | the path or a key is renamed: update `Platform.gnome.terminal` and the fact's documentation (ADR 0025) |
| [NetworkManager](https://networkmanager.dev/) | keyfile sections, 802.1X properties, secret flags | the session check asks `nmcli` for the connections it lists | a property or a flag value changes: update `NetworkProfile` and re-measure the keyfile (ADR 0027) |
| [fontconfig](https://gitlab.freedesktop.org/fontconfig/fontconfig) | the metric-alias table that substitutes Georgia, Calibri and Cambria | the session check asserts a declared family is installed; substitution is not asserted | an alias changes or disappears: re-verify the substitution and update the fonts ADR |
| [adwaita-fonts](https://gitlab.gnome.org/GNOME/adwaita-fonts) | the family, its files and the sizes GNOME ships | the pinned artifacts and their checksums | a new release changes a file: re-pin, and check whether the default size moved with it |
| [CentOS Stream 10](https://gitlab.com/redhat/centos-stream) and [RHEL 10](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/10) | base digests, package names, group contents, gschema overrides | `validate` checks the pinned base digest; the weekly build fails on a missing package | a package DeskOS requires disappears or an override changes: `unavailablePackages`, a pinned artifact, or a field that stops being true |
| [bootc and image mode](https://containers.github.io/bootc/) | install and update semantics, kickstart verbs | `lifecycle.yml` and the VM harness | a kickstart verb or a flag is deprecated: update the recipe before the deprecation lands in a base |
| [Fedora](https://fedoraproject.org/) (example platform) | everything above, earlier | `examples-weekly.yml` | the example stops being representative of a supported platform: say so in `example-platforms.md` |

## How a change becomes work

1. **Read the source, not the summary.** A release note that says "new default font"
   does not say which schema key or which package. The measurement is a
   `gsettings get`, a `fc-match`, or a `rpm -ql` on the base.
2. **Say what it breaks.** A default DeskOS does not write is not ours. A default
   DeskOS overrides is a decision that may now be wrong.
3. **Add the check that would have caught it**, if the answer is "nothing would
   have". That is how the family assertion and the `nmcli` read were born.
4. **Record the threshold here** if it is new, and the decision in an ADR if it
   changes the model.

## Who and when

The maintainer owns this list. The cadence is not a calendar: Renovate moves the
pinned bases and artifacts, the weekly build fails when a package or a digest
moves, and a GNOME release is read when a platform picks it up. What the cadence
buys is that a change is read before it is shipped, not after.

## What this does not cover

- **Vendor support and licensing**, which belong to the vendor and to an
  organization's procurement.
- **Upstream security advisories**, which are [supply-chain.md](../supply-chain.md)
  and the weekly scan, not a watch list.
- **A vendor's own desktop**, which an organization that keeps it owns (ADR 0016).
