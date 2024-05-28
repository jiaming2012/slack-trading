#!/bin/bash

export GO_ENV=development

go run $PROJECTS_DIR/slack-trading/src/eventstreamviewer/main.go
