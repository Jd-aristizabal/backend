package middleware

import (
	"context"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"time"

	"github.com/Jd-aristizabal/backend/internal/shared"
	"github.com/Jd-aristizabal/backend/internal/transport"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Budget struct {
	Key     string
	Limit   int
	Seconds int
}
type Limiter struct {
	DB             *pgxpool.Pool
	Trusted        []netip.Prefix
	ChatDailyLimit int
}

func (l *Limiter) Allow(ctx context.Context, budgets []Budget) (bool, int, error) {
	sort.Slice(budgets, func(i, j int) bool { return budgets[i].Key < budgets[j].Key })
	tx, err := l.DB.Begin(ctx)
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback(ctx)
	for _, b := range budgets {
		var hits, remaining int
		err = tx.QueryRow(ctx, `INSERT INTO rate_limit_buckets(key,window_start,hits,expires_at)
 VALUES($1,to_timestamp(floor(extract(epoch FROM now())/$2)*$2),1,to_timestamp((floor(extract(epoch FROM now())/$2)+1)*$2))
 ON CONFLICT(key,window_start) DO UPDATE SET hits=rate_limit_buckets.hits+1 RETURNING hits,ceil(extract(epoch FROM expires_at-now()))::integer`, b.Key, b.Seconds).Scan(&hits, &remaining)
		if err != nil {
			return false, 0, err
		}
		if hits > b.Limit {
			return false, max(1, remaining), nil
		}
	}
	return true, 0, tx.Commit(ctx)
}
func (l *Limiter) Middleware(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := IPKey(r, l.Trusted)
			budgets := []Budget{{"all:ip:" + ip, 120, 60}}
			switch scope {
			case "chat":
				budgets = []Budget{{"chat:ip:" + ip, 10, 60}, {"chat:client:" + Client(r).String(), 6, 60}, {"chat:daily", l.ChatDailyLimit, 86400}}
			case "score":
				budgets = []Budget{{"score:ip:" + ip, 12, 60}, {"score:client:" + Client(r).String(), 6, 60}}
			case "private":
				budgets = append(budgets, Budget{"private:client:" + Client(r).String(), 60, 60})
			}
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			ok, retryAfter, err := l.Allow(ctx, budgets)
			if err != nil {
				transport.Error(w, shared.Unavailable)
				return
			}
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				transport.Error(w, &shared.Error{Status: 429, Code: "RATE_LIMITED", Message: "Límite de solicitudes alcanzado"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
func (l *Limiter) Cleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
			_, _ = l.DB.Exec(bounded, "DELETE FROM rate_limit_buckets WHERE expires_at<now()-interval '1 day'")
			cancel()
		}
	}
}
