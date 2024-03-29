Thea
====

## About
Thea is a no compromises approach to home media management, focusing on being simple, sleek and automated to the largest extent possible.

When the project is completed, Thea aims to
- [x] Fancy pants Svelte powered web dashboard
  - [x] Live updates over websocket connection to Thea
  - [x] Monitoring ongoing transcodes
  - [x] Controlling ongoing transcodes
  - [ ] Adjusting server settings
  - [ ] Watching completed content
  - [ ] Basic username/password auth and permissions system
  - [ ] Dark mode :o
  - [ ] Lots of other cool stuff
- [x] Automated ingestion of provided media files
- [x] OMDB/IMDB integration
- [x] Transcoding of provided media to multiple formats (ffmpeg)
- [x] Embedded, managed Postgres DB instance (docker)
  - [x] Optional PgAdmin managed instance for managing above DB


# Installation, Configuration, Building, Running and More...
Refer to the Makefile to build and run Thea locally. For example:

- `make audit` checks your code complies with static code analysis and formatting rules
- `make build` builds Thea, placing the binary in `.bin/`
- `make run` runs `make build` and then executes the executable
- `make run/live` is a live-reloading version of `make run`

For more information, see the [Wiki](https://github.com/hbomb79/Thea/wiki)!

# Feel like contributing?
There's lots to do, and you'll find an organized view of what we're working on in the [Project Board](https://github.com/users/hbomb79/projects/2) :) 
