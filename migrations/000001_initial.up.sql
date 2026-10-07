CREATE TABLE focus_tasks (
 id uuid PRIMARY KEY,
 client_id uuid NOT NULL,
 title varchar(120) NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 120),
 priority varchar(5) NOT NULL CHECK (priority IN ('alta','media','baja')),
 completed boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now(),
 completed_at timestamptz,
 UNIQUE (id,client_id),
 CHECK (completed = (completed_at IS NOT NULL))
);
CREATE INDEX focus_tasks_client_created ON focus_tasks(client_id,created_at DESC,id);
CREATE INDEX focus_tasks_completed ON focus_tasks(client_id,completed_at) WHERE completed;
CREATE TABLE focus_sessions (
 id uuid PRIMARY KEY,
 client_id uuid NOT NULL,
 task_id uuid,
 task_title varchar(120) NOT NULL DEFAULT 'Concentración libre',
 duration_seconds integer NOT NULL CHECK (duration_seconds BETWEEN 60 AND 7200),
 status varchar(10) NOT NULL DEFAULT 'active' CHECK (status IN ('active','completed','cancelled')),
 started_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 FOREIGN KEY (task_id,client_id) REFERENCES focus_tasks(id,client_id) ON DELETE SET NULL (task_id),
 CHECK ((status='active' AND finished_at IS NULL) OR (status<>'active' AND finished_at>=started_at))
);
CREATE INDEX focus_sessions_client_created ON focus_sessions(client_id,started_at DESC,id);
CREATE INDEX focus_sessions_completed ON focus_sessions(client_id,finished_at) WHERE status='completed';
CREATE UNIQUE INDEX focus_sessions_one_active ON focus_sessions(client_id) WHERE status='active';
CREATE TABLE pixel_sprint_scores (
 id uuid PRIMARY KEY,
 client_id uuid NOT NULL,
 score integer NOT NULL CHECK (score BETWEEN 0 AND 150000 AND score%100=0),
 max_combo integer NOT NULL CHECK (max_combo BETWEEN 0 AND 300),
 level integer NOT NULL CHECK (level BETWEEN 1 AND 8),
 duration_seconds integer NOT NULL CHECK (duration_seconds BETWEEN 29 AND 31),
 player_name varchar(40),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pixel_sprint_leaderboard ON pixel_sprint_scores(score DESC,created_at,id);
CREATE INDEX pixel_sprint_client ON pixel_sprint_scores(client_id,score DESC,created_at,id);
CREATE TABLE chat_conversations (
 id uuid PRIMARY KEY,
 client_id uuid NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_conversations_client ON chat_conversations(client_id,updated_at DESC);
CREATE TABLE chat_messages (
 id uuid PRIMARY KEY,
 conversation_id uuid NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
 role varchar(9) NOT NULL CHECK (role IN ('user','assistant')),
 content text NOT NULL CHECK (length(content) BETWEEN 1 AND 8000),
 sequence bigint GENERATED ALWAYS AS IDENTITY,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_messages_history ON chat_messages(conversation_id,sequence DESC);
CREATE TABLE rate_limit_buckets (
 key text NOT NULL,
 window_start timestamptz NOT NULL,
 hits integer NOT NULL CHECK (hits>0),
 expires_at timestamptz NOT NULL,
 PRIMARY KEY(key,window_start)
);
CREATE INDEX rate_limit_expiry ON rate_limit_buckets(expires_at);
