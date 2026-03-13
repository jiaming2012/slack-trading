#!/bin/bash

export GO_ENV=production

go run $PROJECTS_DIR/slack-trading/src/eventstreamviewer/main.go