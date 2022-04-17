Thea Processing API
=====================

## About

## Installation

### Configuration
TPA configuration is looked for in `$HOME/.config/tpa/config.yaml`, this file is loaded during initial startup and is used to populate the below configuration options.

Below is an reference .yaml file containing all the possible configuration options. The value of each configuration represents it's _default option_, however, if an option is marked "REQUIRED!", then it has **no default option** and the server will fail to launch unless the configuration is specified in either your config.yaml, or the environment.

Each option is documented below, along with it's environment variable override. You can change the config from it's default by either providing an alternative value in your config.yaml, or by setting the corresponding environment variable (env vars will override any values in the config.yaml)

```yaml
config_dir: ".config/tpa/"
cache_dir: ".cache/tpa"
omdb_api_key: "REQUIRED!"
host: "0.0.0.0"
port: "8080"

formatter:
  import_path: "REQUIRED!"
  default_output_dir: "REQUIRED!"
  target_format: "mp4"
  ffmpeg_binary: "/usr/bin/ffmpeg"
  ffprobe_binary: "/usr/bin/ffprobe"
  import_polling_delay: 3600

concurrency:
  title_threads: 1
  omdb_threads: 1
  ffmpeg_threads: 1

database:
  name: "REQUIRED!"
  username: "REQUIRED!"
  password: "REQUIRED!"
  host: "REQUIRED!"
  port: "REQUIRED!"
```

#### Configuration Directory
##### YAML: config_dir (ENV: CONFIG_DIR)
##### Default: `".config/tpa/"`
This path is relative to the users $HOME directory and is used by TPA to store any relevant configuration (mainly created profiles and targets). Files will be created in this directory and it's intended to be persistent storage.

#### Cache Directory
##### YAML: cache_dir (ENV: CACHE_DIR)
##### Default: `".cache/tpa/"`
This path is relative to the users $HOME directory and is used by TPA to store cache information. This is not intended to be permanent and the program will gracefully handle data loss in this directory.

#### OMDB Api Key
##### YAML: omdb_api_key (ENV: OMBD_API_KEY)
##### Default: **REQUIRED**
The OMDB API key to use when querying OMDB for information about a file being imported.

#### Host Address
##### YAML: host_addr (ENV: HOST_ADDR)
##### Default: `0.0.0.0`
The address to host the data server on; this is used by clients (web front end, CLI, scripts) to access data from TPA via the provided HTTP endpoints and websocket commands.

#### Host Port
##### YAML: host_port (ENV: HOST_PORT)
##### Default: `8080`
The port to host the data server on; similar to "Host Address" above.

#### Format Import Path
##### YAML: formatter.import_path (ENV: FORMAT_IMPORT_PATH)
##### Default: **REQUIRED**
The path to scrape for new files to import. This path is checked frequently (based on "Import Polling Frequency") for new files.

#### Default Output Directory
##### YAML: formatter.default_output_dir (ENV: FORMAT_DEFAULT_OUTPUT_DIR)
##### Default: **REQUIRED**
This path is used by TPA if the target used to transcode a file has no output set, OR it's invalid. Additionally, targets can use this path to compose their own output paths - this value is considered to be the desired output directory and targets may (and should) use this path as the base of their composed output paths.

#### Target Format
##### YAML: formatter.target_format (ENV: FORMAT_TARGET_FORMAT)
##### Default: `mp4`
The format to use when transcoding files via FFmpeg.

#### FFmpeg Binary Path
##### YAML: formatter.ffmpeg_binary (ENV: FORMAT_FFMPEG_BINARY_PATH)
##### Default: `/usr/bin/ffmpeg`
The location to look for the FFmpeg executable binary.

#### FFprobe Binary Path
##### YAML: formatter.ffprobe_binary (ENV: FORMAT_FFPROBE_BINARY_PATH)
##### Default: `/usr/bin/ffprobe`
The location to look for the FFprobe executable binary.

#### Import Polling Frequency
##### YAML: formatter.import_polling_delay (ENV: FORMAT_IMPORT_POLLING_DELAY)
##### Default: `3600`
The amount of time (in seconds) between each poll of the import directory.

#### Title Thread Count
##### YAML: concurrency.title_threads (ENV: CONCURRENCY_TITLE_THREADS)
##### Default: `1`
The amount of go-threads assigned to processing the title information of newly found files. This is stage 2 of the pipeline.

#### OMDB Thread Count
##### YAML: concurrency.omdb_threads (ENV: CONCURRENCY_OMDB_THREADS)
##### Default: `1`
The amount of go-threads assigned to querying OMDB for information. This is stage 3 of the pipeline.

#### FFmpeg Thread Count
##### YAML: concurrency.ffmpeg_threads (ENV: CONCURRENCY_FFMPEG_THREADS)
##### Default: `8`
The maximum amount of _system_ threads to be assigned to FFmpeg execution. This is configured automatically via the `-threads` flag to ffmpeg. When this limit is reached no more FFmpeg instances will be spawned until the running instances finish their work.

#### TODO Database Conf
....