# Build Directory

This directory contains the platform metadata, packaging templates, and generated release artifacts used by the Wails v3 build scripts.

The source assets are grouped by platform:

- `darwin/` contains the macOS bundle metadata and generated icon.
- `windows/` contains the Windows manifest, version metadata, icon, and NSIS templates.
- `ios/` contains the standard Wails v3 iOS project assets.
- `linux/` contains the standard Wails v3 desktop and package metadata.
- `bin/` is the ignored output directory for release artifacts.

Refresh the standard platform assets from `build/` with:

```shell
wails3 update build-assets -name LunaBox -binaryname LunaBox -config config.yml -dir .
```

Review generated changes after refreshing because `windows/nsis/` is part of the checked-in Wails v3 packaging setup.

## Packaging

From the repository root:

```text
scripts\build.bat all <version> <amd64|arm64>
./scripts/build.sh installer <version> <amd64|arm64>
```

Windows produces an NSIS installer and portable ZIP. macOS produces a DMG containing `LunaBox.app`.
