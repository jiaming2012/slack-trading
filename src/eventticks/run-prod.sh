#!/bin/bash

export GO_ENV=production

go run $PROJECTS_DIR/grodt/slack-trading/src/eventticks/main.go