#!/bin/sh
# Install repository-local exclusions without tracking a .gitignore.
set -eu
exclude=$(git rev-parse --git-path info/exclude)
if ! grep -Fqx '/assets/amiga/*' "$exclude"; then
    printf '\n/assets/amiga/*\n!/assets/amiga/GENERATED.txt\n' >> "$exclude"
fi
if ! grep -Fqx '/assets/runtime/data/*' "$exclude"; then
    printf '\n/assets/runtime/data/*\n!/assets/runtime/data/GENERATED.txt\n' >> "$exclude"
fi
if ! grep -Fqx '/assets/generated/' "$exclude"; then
    printf '\n/assets/generated/\n' >> "$exclude"
fi
previous=$(git config --get core.hooksPath || true)
if [ -n "$previous" ] && [ "$previous" != 'tools/git-hooks' ]; then
    printf '%s\n' 'Existing Git hook directory retained; original asset exclusions are installed.'
else
    git config --local core.hooksPath tools/git-hooks
fi
printf '%s\n' 'Original game assets are excluded locally; source files remain tracked.'
