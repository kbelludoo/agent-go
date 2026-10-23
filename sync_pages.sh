#!/bin/bash
exec 9>/tmp/agent-go-git-sync.lock
flock -n 9 || exit 0

cd /home/ubuntu/agent-go-crawler/docs || exit 1

# Auto-heal stale git index.lock if no git process is active
if [ -f ".git/index.lock" ]; then
    if ! pgrep -x "git" >/dev/null 2>&1; then
        rm -f ".git/index.lock"
    fi
fi

if [ -n "$(git status --porcelain live_crawler_data.json distinct_sites.json quarantine_sites.json 2>/dev/null)" ]; then
    git add live_crawler_data.json distinct_sites.json quarantine_sites.json .nojekyll
    git commit -m "chore(telemetry): autonomous crawler live update"
    git push origin gh-pages
fi
