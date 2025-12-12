#!/bin/bash
# =============================================================================
# Paw Diary - Deployment Script for Render
# =============================================================================

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=======================================${NC}"
echo -e "${BLUE}   Paw Diary - Render Deployment${NC}"
echo -e "${BLUE}=======================================${NC}"
echo ""

# -----------------------------------------------------------------------------
# Step 1: Check prerequisites
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[1/5] Checking prerequisites...${NC}"

if ! command -v git &> /dev/null; then
    echo -e "${RED}❌ Git is not installed. Please install Git first.${NC}"
    exit 1
fi

if ! command -v gh &> /dev/null; then
    echo -e "${YELLOW}⚠️  GitHub CLI (gh) not found. Will use manual method.${NC}"
    USE_GH_CLI=false
else
    USE_GH_CLI=true
fi

echo -e "${GREEN}✅ Prerequisites OK${NC}"
echo ""

# -----------------------------------------------------------------------------
# Step 2: Initialize Git if needed
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[2/5] Setting up Git repository...${NC}"

if [ ! -d ".git" ]; then
    echo "Initializing Git repository..."
    git init
    git branch -M main
fi

echo -e "${GREEN}✅ Git repository ready${NC}"
echo ""

# -----------------------------------------------------------------------------
# Step 3: Add and commit files
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[3/5] Staging and committing files...${NC}"

git add .

# Check if there are changes to commit
if git diff --cached --quiet; then
    echo "No changes to commit, continuing..."
else
    COMMIT_MSG="Deploy to Render - $(date '+%Y-%m-%d %H:%M:%S')"
    git commit -m "$COMMIT_MSG"
    echo -e "${GREEN}✅ Changes committed: $COMMIT_MSG${NC}"
fi

echo ""

# -----------------------------------------------------------------------------
# Step 4: Set up GitHub remote
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[4/5] Setting up GitHub remote...${NC}"

# Check if remote exists
if git remote get-url origin &> /dev/null; then
    REMOTE_URL=$(git remote get-url origin)
    echo "Remote already exists: $REMOTE_URL"
else
    echo ""
    echo -e "${YELLOW}No remote found. Please enter your GitHub repository URL:${NC}"
    echo "Format: https://github.com/USERNAME/REPO_NAME.git"
    echo "Or: git@github.com:USERNAME/REPO_NAME.git"
    echo ""
    read -p "GitHub URL: " GITHUB_URL
    
    if [ -z "$GITHUB_URL" ]; then
        echo -e "${RED}❌ No URL provided. Exiting.${NC}"
        exit 1
    fi
    
    git remote add origin "$GITHUB_URL"
    echo -e "${GREEN}✅ Remote added: $GITHUB_URL${NC}"
fi

echo ""

# -----------------------------------------------------------------------------
# Step 5: Push to GitHub
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[5/5] Pushing to GitHub...${NC}"

# Get current branch
CURRENT_BRANCH=$(git branch --show-current)

# Push with force if first push
git push -u origin "$CURRENT_BRANCH" 2>/dev/null || git push -u origin "$CURRENT_BRANCH" --force

echo -e "${GREEN}✅ Code pushed to GitHub successfully!${NC}"
echo ""

# -----------------------------------------------------------------------------
# Next Steps
# -----------------------------------------------------------------------------
echo -e "${BLUE}=======================================${NC}"
echo -e "${BLUE}   🎉 Deployment Preparation Complete!${NC}"
echo -e "${BLUE}=======================================${NC}"
echo ""
echo -e "${GREEN}Next Steps:${NC}"
echo ""
echo "1. Go to https://dashboard.render.com"
echo ""
echo "2. Click 'New +' → 'Blueprint'"
echo ""
echo "3. Connect your GitHub repository"
echo ""
echo "4. Render will auto-detect render.yaml and set up the service"
echo ""
echo "5. ⚠️  IMPORTANT: Set the AI_API_KEY environment variable:"
echo "   - Go to your service → Environment"
echo "   - Add: AI_API_KEY = your_google_gemini_api_key"
echo ""
echo -e "${YELLOW}Or use 'New +' → 'Web Service' → select Docker → connect repo${NC}"
echo ""
echo -e "${BLUE}Your app will be available at: https://paw-diary.onrender.com${NC}"
echo ""
