#!/bin/bash

# Helper script to create and push a version tag
# Usage: ./scripts/release.sh v1.0.0

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v1.0.0"
    exit 1
fi

VERSION=$1

# Validate version format
if ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: Version must be in format vX.Y.Z"
    echo "Example: v1.0.0"
    exit 1
fi

echo "Creating release for version: $VERSION"

# Check if tag already exists
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    echo "Error: Tag $VERSION already exists"
    exit 1
fi

# Check if working directory is clean (including untracked files)
if [ -n "$(git status --porcelain)" ]; then
    echo "Error: Working directory has uncommitted or untracked changes"
    echo "Please commit, stash, or clean changes before releasing"
    exit 1
fi

echo "✓ Validation passed"

# Create annotated tag
echo "Creating annotated tag: $VERSION"
git tag -a "$VERSION" -m "Release version $VERSION"

# Push tag to remote
echo "Pushing tag to remote..."
git push origin "$VERSION"

echo ""
echo "✅ Release tag created and pushed successfully!"
echo ""
echo "GitHub Actions will now:"
echo "  1. Validate the version format"
echo "  2. Build binaries for all platforms"
echo "  3. Create a GitHub release"
echo "  4. Generate release notes automatically"
echo ""
echo "Monitor the release at:"
echo "  https://github.com/amalgamated-tools/enlace/releases/tag/$VERSION"
