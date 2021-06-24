# Thea Processing API

This repository serves the code for the Thea Processing API, implemented in GoLang.

## TODO
- [ ] Parsing of YAML config.
- [ ] Reading of input directory
- [ ] Cache JSON file to record processed items
- [ ] FFMpeg golang bindings
- [ ] Querying OMDB for information on files concurrently
  - [ ] Implement database connection, querying and insertion of OMDB information
- [ ] Processing title to remove unwanted text (criteria from config YAML)
- [ ] Formatting of inputs using FFMpeg, concurrently
- [ ] Implement terminal-based rich user interface (including an accessible API)
  - [ ] Control queue - pause (if possible), stop, reorder pending items, skip, etc
  - [ ] Adjust configuration options from within interface
  - [ ] ...
- [ ] Implement HTTP RESTful API for querying and controlling/issuing commands to the processor
