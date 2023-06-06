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


## Installation
As this project is still very much a WIP, it's not advised that anyone tries to clone/run this project with the expectation of _using_ it. However, contributions
are always welcome; below are some instructions for getting started with Thea for development!


#### Pre-requisites
First, lets get some *pre-requisites* out of the way. As you may have pieced together by now, you will need to have installed _AND configured:_
 - The Go language tools
 - The Docker runtime
 - Node runtime (for Svelte frontend)


#### Configuration

Next, we need to give Thea some configuration to tell it:
 - Where to look for ingestable media
 - Your OMDB API key ([generate one here](https://www.omdbapi.com/apikey.aspx))
 - What format we want to output media to (this may be going away soon)
 - Where the ffmpeg & ffprobe commands are, and how often to poll for
 - What Docker services to spawn and manage, if any
 - The DB username/password

Thea configuration is looked for in `$HOME/.config/thea/config.yaml`, this file is loaded during initial startup and is used to populate the below configuration options.

Below is an reference .yaml file containing all the possible configuration options. For each configuration marked `# Optional`, the value provided is the default
that Thea will use if a value is not expressly provided.

For each configuratuin marked `# REQUIRED`, a value MUST be provided - else Thea will refuse to start.

Each option is documented below, along with it's environment variable override. You can change the config from it's default by either providing an alternative value in your config.yaml, or by setting the corresponding environment variable (env vars will override any values in the config.yaml)

```yaml
omdb_api_key: "mykey" # REQUIRED, see note above for generation.

config_dir: "$HOME/.config/thea/" # Optional
cache_dir: "$HOME/.cache/thea/" # Optional

formatter:
  import_path: "dir_to_ingest_from" # REQUIRED
  default_output_dir: "dir_to_output_to" # REQUIRED

  target_format: "mp4" # Optional
  ffmpeg_binary: "/usr/bin/ffmpeg" # Optional
  ffprobe_binary: "/usr/bin/ffprobe" # Optional
  import_polling_delay: 3600 # Optional

concurrency:
  title_threads: 1 # Optional
  omdb_threads: 1 # Optional
  ffmpeg_threads: 1 # Optional

database:
  username: "foo" # REQUIRED
  password: "bar" # REQUIRED

  name: "THEA_DB" # Optional
  host: "0.0.0.0" # Optional
  port: "5432" # Optional

# All optional
docker_services:
  enable_postgres: true # Allows Thea to self-manage a Postgres instance using Docker
  enable_pg_admin: false # Best to leave this disabled unless you need it
  enable_frontend: false # NYI
```

#### Building and Running

Once those tools are complete, go ahead and `git clone` this repository (ideally inside of your `$HOME/go/src/` directory).

Once cloned, `cd` in to it. We're now ready to build and run the server and client.

**Server** Run `go run` -> Voila

**Client** Run `npm run dev` -> This will launch a web server on `0.0.0.0:5000`. The HOST and PORT can be overriden using the `WS_HOST` and `WS_POST` environment variables.

#### Known Issues
There are a _lot_ of rough edges here, so expect to run in to a lot of trouble. If you find something, feel free to log an issue with steps to reproduce and error logs at the very least.

Occasionally Thea can outright crash, which leaves the Docker network in an invalid state. Run this command to fix this... **However, BE WARNED:** This command will _DESTROY and DELETE ALL_ Docker containers/images on the system.

`docker kill $(docker ps -aq); docker rm $(docker ps -aq); docker network rm thea_network`