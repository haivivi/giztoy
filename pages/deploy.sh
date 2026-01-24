#!/bin/bash
# Deploy website to GitHub Pages
# Usage: bazel run //pages:deploy
#    or: ./pages/deploy.sh  (will auto-invoke bazel)
set -e

# If not running via bazel, re-invoke with bazel
if [[ -z "$BUILD_WORKSPACE_DIRECTORY" ]]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    WORKSPACE_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
    
    echo "🔧 Not running via bazel, invoking: bazel run //pages:deploy"
    cd "$WORKSPACE_ROOT"
    exec bazel run //pages:deploy
fi

RUNFILES="${BASH_SOURCE[0]}.runfiles"
WORKSPACE="$BUILD_WORKSPACE_DIRECTORY"

# Find the built website tarball
WWW_TAR="$RUNFILES/_main/pages/www.tar.gz"
if [[ ! -f "$WWW_TAR" ]]; then
    echo "Error: www.tar.gz not found at $WWW_TAR"
    exit 1
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🚀 Deploying to GitHub Pages"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check git status
cd "$WORKSPACE"

# Get current commit info
CURRENT_BRANCH=$(git branch --show-current)
CURRENT_COMMIT=$(git rev-parse --short HEAD)
COMMIT_MSG="Deploy from $CURRENT_BRANCH@$CURRENT_COMMIT"

echo "📍 Current: $CURRENT_BRANCH@$CURRENT_COMMIT"

# Create temp directories
SITE_DIR=$(mktemp -d)
DEPLOY_DIR=$(mktemp -d)
trap "rm -rf $SITE_DIR $DEPLOY_DIR; cd $WORKSPACE && git worktree remove $DEPLOY_DIR --force 2>/dev/null || true" EXIT

# Extract website
echo "📦 Extracting website..."
tar -xzf "$WWW_TAR" -C "$SITE_DIR"

# Setup gh-pages worktree
echo "🌿 Setting up gh-pages branch..."
cd "$WORKSPACE"

# Check if gh-pages branch exists
if git show-ref --verify --quiet refs/heads/gh-pages; then
    # Branch exists, create worktree
    git worktree add "$DEPLOY_DIR" gh-pages
else
    # Branch doesn't exist, create orphan branch
    echo "   Creating new gh-pages branch..."
    git worktree add --detach "$DEPLOY_DIR"
    cd "$DEPLOY_DIR"
    git checkout --orphan gh-pages
    git reset --hard
    git commit --allow-empty -m "Initialize gh-pages"
fi

# Copy site contents
echo "📋 Copying site contents..."
cd "$DEPLOY_DIR"
# Remove old content (except .git)
find . -mindepth 1 -maxdepth 1 ! -name '.git' -exec rm -rf {} +
# Copy new content
cp -r "$SITE_DIR"/* .

# Add .nojekyll to disable Jekyll processing
touch .nojekyll

# Add CNAME for custom domain
echo "giztoy.com" > CNAME

# Commit and push
echo "📤 Committing and pushing..."
git add -A

if git diff --staged --quiet; then
    echo "   No changes to deploy."
else
    git commit -m "$COMMIT_MSG"
    git push origin gh-pages
    echo ""
    echo "✅ Deployed successfully!"
    echo ""
    
    # Try to get the pages URL
    REMOTE_URL=$(git remote get-url origin 2>/dev/null || echo "")
    if [[ "$REMOTE_URL" == *"github.com"* ]]; then
        # Extract org/repo from URL
        if [[ "$REMOTE_URL" == git@github.com:* ]]; then
            REPO_PATH="${REMOTE_URL#git@github.com:}"
            REPO_PATH="${REPO_PATH%.git}"
        elif [[ "$REMOTE_URL" == https://github.com/* ]]; then
            REPO_PATH="${REMOTE_URL#https://github.com/}"
            REPO_PATH="${REPO_PATH%.git}"
        fi
        
        if [[ -n "$REPO_PATH" ]]; then
            ORG="${REPO_PATH%%/*}"
            REPO="${REPO_PATH#*/}"
            echo "🌐 Site URL: https://$ORG.github.io/$REPO/"
        fi
    fi
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
