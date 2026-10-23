#!/bin/bash
cd /home/ubuntu/agent-go-crawler/docs || exit 1
if [ -n "$(git status --porcelain live_crawler_data.json distinct_sites.json 2>/dev/null)" ]; then
    git add live_crawler_data.json distinct_sites.json
    git commit -m "chore: live crawler update [skip ci]"
    git push origin gh-pages
fi
