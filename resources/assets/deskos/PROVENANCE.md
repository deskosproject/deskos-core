# Asset provenance

| File | Origin | Status |
|---|---|---|
| `deskos-light.svg`, `deskos-dark.svg` | drawn in this repository (gradient and circles) | placeholder wallpapers |
| `deskos-login-logo.svg` | drawn in this repository (outline square and text "DeskOS") | provisional placeholder; not an approved DeskOS logo |
| `deskos-splash-watermark.png` | rendered from `deskos-login-logo.svg` with `rsvg-convert -w 240 -h 48 -f png` (librsvg2-tools 2.57.1-9.el10, font Red Hat Display) in `localhost/deskos-core-centos10:freeze`, then cropped to its content plus a 2 px margin (`magick -crop 126x48+0+0 +repage -strip PNG32:`) so Plymouth centers the mark; 126x48 RGBA, transparent background, sha256 `e286d8294944c47850e6cec864fb444edeca62205e2aa8c54b26f39fe79c78c2` | provisional placeholder; Plymouth splash watermark |

No third-party artwork. A permanent DeskOS identity needs a maintainer
decision, including its license.
