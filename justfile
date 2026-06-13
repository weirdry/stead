set dotenv-load := false

mod := "github.com/ed/stead"
bin := "bin/stead"

import "just/build.just"
import "just/check.just"
import "just/install.just"

default:
  @just --list
