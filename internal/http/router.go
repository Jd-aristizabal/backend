package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Jd-aristizabal/backend/internal/config"
	mw "github.com/Jd-aristizabal/backend/internal/middleware"
	"github.com/Jd-aristizabal/backend/internal/modules/chatbot"
	"github.com/Jd-aristizabal/backend/internal/modules/focus"
	"github.com/Jd-aristizabal/backend/internal/modules/games"
	"github.com/Jd-aristizabal/backend/internal/modules/portfolio"
	"github.com/Jd-aristizabal/backend/internal/shared"
	"github.com/Jd-aristizabal/backend/internal/transport"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type handler func(http.ResponseWriter, *http.Request) error

func wrap(fn handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			transport.Error(w, err)
		}
	}
}
func respond(w http.ResponseWriter, status int, v any, err error) error {
	if err != nil {
		return err
	}
	transport.Data(w, status, v)
	return nil
}
func param(r *http.Request) (uuid.UUID, error) { return transport.UUID(chi.URLParam(r, "id")) }
func New(c config.Config, db *pgxpool.Pool, logger *slog.Logger, provider chatbot.AIProvider) (http.Handler, *mw.Limiter, error) {
	p, err := portfolio.New()
	if err != nil {
		return nil, nil, err
	}
	f, g := focus.New(db), games.New(db)
	chat := chatbot.New(db, provider, p.Context(), c.AITimeout, c.MaxTokens)
	limiter := &mw.Limiter{DB: db, Trusted: c.TrustedProxies, ChatDailyLimit: c.ChatDailyLimit}
	r := chi.NewRouter()
	r.Use(mw.Base(logger, c.Origins))
	r.NotFound(wrap(func(w http.ResponseWriter, r *http.Request) error { return shared.NotFound }))
	r.MethodNotAllowed(wrap(func(w http.ResponseWriter, r *http.Request) error {
		return &shared.Error{Status: 405, Code: "METHOD_NOT_ALLOWED", Message: "Método no permitido"}
	}))
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		transport.JSON(w, 200, map[string]string{"status": "ok"})
	})
	r.Get("/ready", wrap(func(w http.ResponseWriter, r *http.Request) error {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			return shared.Unavailable
		}
		transport.JSON(w, 200, map[string]string{"status": "ok"})
		return nil
	}))
	r.Route("/api/v1", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(limiter.Middleware("public"))
			r.Get("/portfolio/profile", func(w http.ResponseWriter, r *http.Request) { transport.Data(w, 200, p.Profile) })
			r.Get("/portfolio/projects", func(w http.ResponseWriter, r *http.Request) { transport.Data(w, 200, p.Projects) })
			r.Get("/portfolio/skills", func(w http.ResponseWriter, r *http.Request) { transport.Data(w, 200, p.Skills) })
			r.Get("/portfolio/contact", func(w http.ResponseWriter, r *http.Request) { transport.Data(w, 200, p.Contact) })
			r.Get("/games/{game}/leaderboard", wrap(func(w http.ResponseWriter, r *http.Request) error {
				limit, _, err := transport.Pagination(r, 10, 100)
				if err != nil {
					return err
				}
				v, err := g.LeaderboardGame(r.Context(), chi.URLParam(r, "game"), limit)
				return respond(w, 200, v, err)
			}))
		})
		r.Group(func(r chi.Router) {
			r.Use(mw.Identity)
			r.Use(limiter.Middleware("private"))
			r.Get("/focus/tasks", wrap(func(w http.ResponseWriter, r *http.Request) error {
				limit, offset, err := transport.Pagination(r, 50, 100)
				if err != nil {
					return err
				}
				v, err := f.Tasks(r.Context(), mw.Client(r), limit, offset)
				return respond(w, 200, v, err)
			}))
			r.Post("/focus/tasks", wrap(func(w http.ResponseWriter, r *http.Request) error {
				var req focus.CreateTask
				if err := transport.Decode(w, r, &req); err != nil {
					return err
				}
				v, err := f.CreateTask(r.Context(), mw.Client(r), req)
				return respond(w, 201, v, err)
			}))
			r.Patch("/focus/tasks/{id}", wrap(func(w http.ResponseWriter, r *http.Request) error {
				id, err := param(r)
				if err != nil {
					return err
				}
				var req focus.PatchTask
				if err = transport.Decode(w, r, &req); err != nil {
					return err
				}
				v, err := f.UpdateTask(r.Context(), mw.Client(r), id, req)
				return respond(w, 200, v, err)
			}))
			r.Delete("/focus/tasks/{id}", wrap(func(w http.ResponseWriter, r *http.Request) error {
				id, err := param(r)
				if err != nil {
					return err
				}
				if err = f.DeleteTask(r.Context(), mw.Client(r), id); err != nil {
					return err
				}
				transport.JSON(w, 204, nil)
				return nil
			}))
			r.Get("/focus/sessions", wrap(func(w http.ResponseWriter, r *http.Request) error {
				limit, offset, err := transport.Pagination(r, 50, 100)
				if err != nil {
					return err
				}
				v, err := f.Sessions(r.Context(), mw.Client(r), limit, offset)
				return respond(w, 200, v, err)
			}))
			r.Post("/focus/sessions", wrap(func(w http.ResponseWriter, r *http.Request) error {
				var req focus.CreateSession
				if err := transport.Decode(w, r, &req); err != nil {
					return err
				}
				v, err := f.StartSession(r.Context(), mw.Client(r), req)
				return respond(w, 201, v, err)
			}))
			for _, action := range []struct{ path, status string }{{"complete", "completed"}, {"cancel", "cancelled"}} {
				r.Patch("/focus/sessions/{id}/"+action.path, wrap(func(w http.ResponseWriter, r *http.Request) error {
					id, err := param(r)
					if err != nil {
						return err
					}
					v, err := f.FinishSession(r.Context(), mw.Client(r), id, action.status)
					return respond(w, 200, v, err)
				}))
			}
			r.Get("/focus/stats", wrap(func(w http.ResponseWriter, r *http.Request) error {
				v, err := f.Stats(r.Context(), mw.Client(r), r.URL.Query().Get("time_zone"))
				return respond(w, 200, v, err)
			}))
			r.Get("/games/{game}/me", wrap(func(w http.ResponseWriter, r *http.Request) error {
				limit, offset, err := transport.Pagination(r, 10, 100)
				if err != nil {
					return err
				}
				v, err := g.MeGame(r.Context(), mw.Client(r), chi.URLParam(r, "game"), limit, offset)
				return respond(w, 200, v, err)
			}))
			r.With(limiter.Middleware("score")).Post("/games/{game}/scores", wrap(func(w http.ResponseWriter, r *http.Request) error {
				var req games.SubmitScore
				if err := transport.Decode(w, r, &req); err != nil {
					return err
				}
				v, err := g.SubmitGame(r.Context(), mw.Client(r), chi.URLParam(r, "game"), req)
				return respond(w, 201, v, err)
			}))
			r.With(limiter.Middleware("chat")).Post("/chat", wrap(func(w http.ResponseWriter, r *http.Request) error {
				var req chatbot.Request
				if err := transport.Decode(w, r, &req); err != nil {
					return err
				}
				v, err := chat.Chat(r.Context(), mw.Client(r), req)
				return respond(w, 200, v, err)
			}))
			r.With(limiter.Middleware("chat")).Post("/chat/conversations", wrap(func(w http.ResponseWriter, r *http.Request) error {
				v, err := chat.Create(r.Context(), mw.Client(r))
				return respond(w, 201, v, err)
			}))
			r.Get("/chat/conversations/{id}/messages", wrap(func(w http.ResponseWriter, r *http.Request) error {
				id, err := param(r)
				if err != nil {
					return err
				}
				limit, offset, err := transport.Pagination(r, 50, 100)
				if err != nil {
					return err
				}
				v, err := chat.History(r.Context(), mw.Client(r), id, limit, offset)
				return respond(w, 200, v, err)
			}))
			r.Delete("/chat/conversations/{id}", wrap(func(w http.ResponseWriter, r *http.Request) error {
				id, err := param(r)
				if err != nil {
					return err
				}
				if err = chat.Delete(r.Context(), mw.Client(r), id); err != nil {
					return err
				}
				transport.JSON(w, 204, nil)
				return nil
			}))
		})
	})
	return r, limiter, nil
}
