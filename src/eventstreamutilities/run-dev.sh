#!/bin/bash

export GO_ENV=development

COMMAND=$1
ARG_1=$2

go run $PROJECTS_DIR/grodt/slack-trading/src/eventstreamutilities/main.go $COMMAND $ARG_1
