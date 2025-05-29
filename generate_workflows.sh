#!/bin/bash

# filepath: /Users/Justin.Neubert/projects/v1flows/runner-plugins/generate_workflows.sh

set -euo pipefail

WORKFLOWS_DIR=".github/workflows"

mkdir -p "$WORKFLOWS_DIR"

for type in action-plugins endpoint-plugins; do
  echo "Processing type: $type"
  for plugin in $(find "$type" -mindepth 1 -maxdepth 1 -type d -exec basename {} \;); do
    echo "  Found plugin: $plugin"
    cat <<EOL > "$WORKFLOWS_DIR/check-image-build-${type}-${plugin}.yml"
name: Check $type Build - $plugin

on:
  pull_request:
    types: [opened, reopened, edited, synchronize]
    branches: [ "develop" ]
    paths:
      - "$type/$plugin/**"

jobs:
  build-plugin:
    name: Build Plugin
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v2
        with:
          go-version: '1.24'

      - name: Build Plugin
        working-directory: $type/$plugin
        run: go build
EOL
    echo "Generated $WORKFLOWS_DIR/check-image-build-${type}-${plugin}.yml"

    cat <<EOL > "$WORKFLOWS_DIR/release-${type}-${plugin}.yml"
name: Release $type - $plugin

on:
  workflow_dispatch:
  push:
    branches: [ "main" ]
    paths:
      - "$type/$plugin/**"

jobs:
  build-and-release:
    name: Build and Release $plugin
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Read Plugin Version
        id: read_version
        working-directory: $type/$plugin
        run: |
          VERSION=\$(cat .version)
          echo "version=\$VERSION" >> \$GITHUB_OUTPUT

      - name: Check if Tag or Release Exists
        id: check-tag-release
        env:
          GITHUB_TOKEN: \${{ secrets.ACCESS_TOKEN }}
        run: |
          TAG_EXISTS=\$(git ls-remote --tags origin | grep "refs/tags/$plugin-v\${{ steps.read_version.outputs.version }}" || true)
          RELEASE_EXISTS=\$(gh release list --repo \${{ github.repository }} | grep "Release $plugin v\${{ steps.read_version.outputs.version }}" || true)
          if [ -n "\$TAG_EXISTS" ] || [ -n "\$RELEASE_EXISTS" ]; then
            echo "skip=true" >> \$GITHUB_OUTPUT
          else
            echo "skip=false" >> \$GITHUB_OUTPUT
          fi

      - name: Setup Go
        uses: actions/setup-go@v2
        with:
          go-version: '1.24'

      - name: Build Plugin
        if: steps.check-tag-release.outputs.skip == 'false'
        working-directory: $type/$plugin
        run: |
          GOOS=darwin GOARCH=amd64 go build -o $plugin-v\${{ steps.read_version.outputs.version }}-darwin-amd64
          GOOS=darwin GOARCH=arm64 go build -o $plugin-v\${{ steps.read_version.outputs.version }}-darwin-arm64
          GOOS=linux GOARCH=amd64 go build -o $plugin-v\${{ steps.read_version.outputs.version }}-linux-amd64
          GOOS=darwin GOARCH=amd64 go build -o $plugin-latest-darwin-amd64
          GOOS=darwin GOARCH=arm64 go build -o $plugin-latest-darwin-arm64
          GOOS=linux GOARCH=amd64 go build -o $plugin-latest-linux-amd64

      - name: Create Tag
        if: steps.check-tag-release.outputs.skip == 'false'
        id: tag_version
        uses: mathieudutour/github-tag-action@v6.2
        with:
          github_token: \${{ secrets.ACCESS_TOKEN }}
          custom_tag: $plugin-v\${{ steps.read_version.outputs.version }}
          tag_prefix: ''
      
      - name: Update -latest Tag
        if: steps.check-tag-release.outputs.skip == 'false'
        run: |
          set -e
          # Delete local and remote -latest tag if it exists
          git tag -d $plugin-latest 2>/dev/null || true
          git push origin :refs/tags/$plugin-latest 2>/dev/null || true
          # Create new -latest tag at current commit
          git tag $plugin-latest
          git push origin $plugin-latest --force

      - name: Create Version Release
        if: steps.check-tag-release.outputs.skip == 'false'
        id: create_version_release
        uses: ncipollo/release-action@v1
        with:
          name: Release $plugin v\${{ steps.read_version.outputs.version }}
          tag: \${{ steps.tag_version.outputs.new_tag }}
          artifacts: $type/$plugin/$plugin-v\${{ steps.read_version.outputs.version }}-*
          skipIfReleaseExists: true
          generateReleaseNotes: true
          token: \${{ secrets.ACCESS_TOKEN }}
      
      - name: Create Latest Release
        if: steps.check-tag-release.outputs.skip == 'false'
        id: create_latest_release
        uses: ncipollo/release-action@v1
        with:
          name: Release $plugin latest
          tag: $plugin-latest
          artifacts: $type/$plugin/$plugin-latest-*
          skipIfReleaseExists: false
          generateReleaseNotes: false
          token: \${{ secrets.ACCESS_TOKEN }}
EOL

    echo "Generated $WORKFLOWS_DIR/release-${type}-${plugin}.yml"
  done
done

echo "Generated workflow files for all plugins in $WORKFLOWS_DIR"