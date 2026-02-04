# GitHub Pages Setup

This document describes the GitHub Pages configuration for git-wt.

## Overview

The project uses GitHub Pages with Jekyll to serve a landing page at
https://ahmedelgabri.github.io/git-wt/

## Configuration

### Jekyll Setup

- **Theme**: `pages-themes/minimal` (remote theme)
- **Source**: `docs/` directory
- **Main file**: `docs/index.md`

### Files

```
docs/
├── _config.yml    # Jekyll configuration
├── index.md       # Landing page content
└── *.md           # Other docs (excluded from site)
```

### GitHub Actions Workflow

The site is deployed via `.github/workflows/pages.yml`:

- Triggers on pushes to `main` that modify `docs/` or the workflow
- Uses `actions/jekyll-build-pages` to build
- Deploys with `actions/deploy-pages`

## GitHub Repository Settings

To enable GitHub Pages:

1. Go to repository Settings > Pages
2. Under "Build and deployment", select "GitHub Actions"
3. The workflow will handle the rest

## Local Development

To preview locally:

```bash
cd docs
bundle install  # First time only
bundle exec jekyll serve
```

Or with Docker:

```bash
docker run --rm -v "$PWD/docs:/srv/jekyll" -p 4000:4000 jekyll/jekyll jekyll serve
```

## Updating Content

Edit `docs/index.md` to update the landing page. Changes pushed to `main` will
automatically deploy.
