-- +goose Up

CREATE TABLE series(
    id UUID NOT NULL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    tmdb_id TEXT NOT NULL,
    title TEXT NOT NULL,

    CONSTRAINT series_uk_tmdb_id UNIQUE(tmdb_id)
);

CREATE TABLE season(
    id UUID NOT NULL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    tmdb_id TEXT NOT NULL,
    season_number INT NOT NULL,
    title TEXT NOT NULL,
    series_id UUID NOT NULL,

    CONSTRAINT season_uk_tmdb_id UNIQUE(tmdb_id),
    CONSTRAINT season_fk_series_id FOREIGN KEY(series_id) REFERENCES series(id) ON DELETE CASCADE
);

CREATE TYPE media_type AS ENUM ('movie', 'episode');
CREATE TABLE media(
    id UUID NOT NULL PRIMARY KEY,
    type media_type NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    tmdb_id TEXT NOT NULL,
    title TEXT NOT NULL,
    adult BOOLEAN NOT NULL,
    source_path TEXT NOT NULL,

    -- Nullable columns which must be specified if the media t is episode
    episode_number INT CHECK (episode_number IS NULL OR episode_number >= 0),
    season_id UUID,

    -- TMDB IDs are only unique in their category (movie/tv), so only enforce uniqueness
    -- when the type matches too
    CONSTRAINT media_uk_tmdb_id_type UNIQUE(tmdb_id, type),
    CONSTRAINT media_fk_season_id FOREIGN KEY(season_id) REFERENCES season(id) ON DELETE CASCADE,
    CONSTRAINT valid_media CHECK(
        (type = 'movie' AND episode_number IS NULL AND season_id IS NULL) OR
        (type = 'episode' AND episode_number IS NOT NULL AND season_id IS NOT NULL)
    )
);

CREATE TABLE genre(
    id BIGSERIAL PRIMARY KEY,
    label TEXT UNIQUE
);

CREATE TABLE movie_genres(
    id UUID PRIMARY KEY,
    movie_id UUID NOT NULL,
    genre_id BIGSERIAL NOT NULL,

    CONSTRAINT movie_genres_fk_movie_id FOREIGN KEY(movie_id) REFERENCES media(id) ON DELETE CASCADE,
    CONSTRAINT movie_genres_fk_genre_id FOREIGN KEY(genre_id) REFERENCES genre(id) ON DELETE CASCADE,
    CONSTRAINT movie_genres_uk_movie_genre UNIQUE (movie_id, genre_id)
);

CREATE TABLE series_genres(
    id UUID PRIMARY KEY,
    series_id UUID NOT NULL,
    genre_id BIGSERIAL NOT NULL,

    CONSTRAINT series_genres_fk_series_id FOREIGN KEY(series_id) REFERENCES series(id) ON DELETE CASCADE,
    CONSTRAINT series_genres_fk_genre_id FOREIGN KEY(genre_id) REFERENCES genre(id) ON DELETE CASCADE,
    CONSTRAINT series_genres_uk_series_genre UNIQUE (series_id, genre_id)
);


CREATE TABLE transcode_target(
    id UUID NOT NULL PRIMARY KEY,
    label TEXT NOT NULL,
    ffmpeg_options JSONB NOT NULL,
    extension TEXT NOT NULL,
    
    CONSTRAINT transcode_target_uk_label UNIQUE(label)
);

CREATE TABLE workflow(
    id UUID NOT NULL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    label TEXT NOT NULL,
    enabled BOOLEAN NOT NULL,

    CONSTRAINT workflow_uk_label UNIQUE(label)
);

CREATE TABLE workflow_criteria(
    id UUID NOT NULL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    match_key INT NOT NULL,
    match_type INT NOT NULL,
    match_combine_type INT NOT NULL,
    match_value TEXT NOT NULL,
    workflow_id UUID NOT NULL,

    CONSTRAINT workflow_criteria_fk_workflow_id FOREIGN KEY(workflow_id) REFERENCES workflow(id) ON DELETE CASCADE
);

CREATE TABLE workflow_transcode_targets(
    id UUID NOT NULL PRIMARY KEY,
    workflow_id UUID NOT NULL,
    transcode_target_id UUID NOT NULL,

    CONSTRAINT workflow_transcode_targets_fk_workflow_id FOREIGN KEY(workflow_id) REFERENCES workflow(id) ON DELETE CASCADE,
    CONSTRAINT workflow_transcode_targets_fk_transcode_target_id FOREIGN KEY(transcode_target_id) REFERENCES transcode_target(id) ON DELETE CASCADE
);

CREATE TABLE media_transcodes(
    id UUID NOT NULL PRIMARY KEY,
    media_id UUID NOT NULL,
    transcode_target_id UUID NOT NULL,
    path TEXT NOT NULL,

    CONSTRAINT media_transcodes_fk_media_id FOREIGN KEY(media_id) REFERENCES media(id) ON DELETE RESTRICT,
    CONSTRAINT media_transcodes_fk_transcode_target_id FOREIGN KEY(transcode_target_id) REFERENCES transcode_target(id)
);

CREATE TABLE users(
    id UUID NOT NULL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    last_login TIMESTAMPTZ,
    last_refresh TIMESTAMPTZ,
    username BYTEA NOT NULL UNIQUE,
    password BYTEA NOT NULL,
    salt BYTEA NOT NULL
);
