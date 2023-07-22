-- +goose Up

CREATE TABLE movie (
    id UUID NOT NULL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    tmdb_id TEXT NOT NULL,
    title TEXT NOT NULL,
    adult BOOLEAN NOT NULL,
    source_path TEXT NOT NULL,

    CONSTRAINT movie_uk_tmdb_id UNIQUE(tmdb_id)
);

CREATE TABLE series(
    id UUID NOT NULL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    tmdb_id TEXT NOT NULL,
    title TEXT NOT NULL,
    adult BOOLEAN NOT NULL,

    CONSTRAINT series_uk_tmdb_id UNIQUE(tmdb_id)
);

CREATE TABLE season(
    id UUID NOT NULL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    tmdb_id TEXT NOT NULL,
    title TEXT NOT NULL,
    series_id UUID NOT NULL,

    CONSTRAINT season_uk_tmdb_id UNIQUE(tmdb_id),
    CONSTRAINT season_fk_series_id FOREIGN KEY(series_id) REFERENCES series(id) ON DELETE CASCADE
);

CREATE TABLE episode(
    id UUID NOT NULL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    tmdb_id TEXT NOT NULL,
    title TEXT NOT NULL,
    source_path TEXT NOT NULL,
    season_id UUID NOT NULL,

    CONSTRAINT episode_uk_tmdb_id UNIQUE(tmdb_id),
    CONSTRAINT episode_fk_season_id FOREIGN KEY(season_id) REFERENCES season(id) ON DELETE CASCADE
);

CREATE TABLE transcode_target(
    id UUID NOT NULL PRIMARY KEY,
    label TEXT NOT NULL,
    ffmpeg_options JSONB NOT NULL,
    extension CHAR(5) NOT NULL,
    
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
