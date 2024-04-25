#!/bin/bash

export GO_ENV=development

go run $PROJECTS_DIR/grodt/slack-trading/src/eventstreamutilities/main.go
