CREATE TABLE arcade_scores (
 id uuid PRIMARY KEY,
 client_id uuid NOT NULL,
 game_id varchar(20) NOT NULL CHECK (game_id IN ('orbit-match','pulse-orbit')),
 score integer NOT NULL CHECK (score BETWEEN 0 AND 240000),
 max_combo integer NOT NULL CHECK (max_combo BETWEEN 0 AND 300),
 level integer NOT NULL CHECK (level BETWEEN 1 AND 8),
 duration_seconds integer NOT NULL CHECK (duration_seconds BETWEEN 1 AND 60),
 player_name varchar(40) NOT NULL CHECK (length(btrim(player_name)) BETWEEN 2 AND 40),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX arcade_scores_leaderboard ON arcade_scores(game_id,score DESC,created_at,id);
CREATE INDEX arcade_scores_client ON arcade_scores(game_id,client_id,score DESC,created_at,id);
CREATE INDEX arcade_scores_names ON arcade_scores(game_id,client_id,created_at DESC) INCLUDE(player_name);
CREATE INDEX pixel_sprint_names ON pixel_sprint_scores(client_id,created_at DESC) INCLUDE(player_name) WHERE player_name IS NOT NULL;
