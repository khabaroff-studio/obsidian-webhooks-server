#!/bin/bash
set -e

# Get the directory where this script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$SCRIPT_DIR/.."
PLUGIN_DIR="$PROJECT_ROOT/plugin"
RELEASE_DIR="$PLUGIN_DIR/release"

echo "Building Obsidian plugin..."

# Navigate to plugin directory
cd "$PLUGIN_DIR"

# Install dependencies if needed
if [ ! -d "node_modules" ]; then
    echo "Installing dependencies..."
    npm install
fi

# Build the plugin
echo "Compiling TypeScript..."
npm run build

# Create release directory if it doesn't exist
mkdir -p "$RELEASE_DIR"

# Remove old archives
rm -f "$RELEASE_DIR/obsidian-webhooks.zip" "$RELEASE_DIR"/obsidian-webhooks-*.zip

# Create ZIP archive with required files
echo "Creating release archive..."
TEMP_DIR=$(mktemp -d)
cp main.js "$TEMP_DIR/"
cp manifest.json "$TEMP_DIR/"
[ -f styles.css ] && cp styles.css "$TEMP_DIR/" || echo "No styles.css found, skipping..."

# Create ZIP
cd "$TEMP_DIR"
if [ -f styles.css ]; then
    zip -r obsidian-webhooks.zip main.js manifest.json styles.css
else
    zip -r obsidian-webhooks.zip main.js manifest.json
fi

# Move ZIP to release directory
mv obsidian-webhooks.zip "$RELEASE_DIR/"

# Cleanup
rm -rf "$TEMP_DIR"

echo "✅ Plugin built successfully: plugin/release/obsidian-webhooks.zip"
