# toy-rss

toy-rss is an RSS/Atom reader implemented in go.

Ideally, this should provide a mechanism to easily add channels,
view items, and manage incoming content.

This is mostly an excuse for me to re-learn go.

## How do I...

### ... download it?

`$ export GOPATH=\`pwd\``
`$ go get github.com/smklein/toy-rss`
`$ cd $GOPATH/src/github.com/smklein/toy-rss`

### ... build it?

`$ go build`

### ... run it?

`$ mkdir data`
`$ ./toy-rss`

### ... test it?

`$ go test ./...`

## Core Components

### view

The view package is responsible for two primary objectives:
  - Displaying information on screen.
  - Receving user input.

### feed

The feed package handles incoming RSS and Atom feeds.

### storage

The storage package is responsible for persisting state between starting and
stopping the rss reader.

## Utilities

### agingmap

agingmap is a utility library used to have a limited size map. This prevents
too much memory from being used, but still provides quick access to elements
which need to be stored in memory.
