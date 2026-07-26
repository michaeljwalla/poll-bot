#!/bin/bash
dlv debug --build-flags="-gcflags='all=-N -l'" main.go
