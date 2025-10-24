package timeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

type Event struct {
	At     string
	Source string // orders|payments|outbox|inbox
	Type   string
	Detail string
}

func (r *Repo) Timeline(ctx context.Context, orderID string) ([]Event, error) {
	q := `
WITH o AS (
  SELECT created_at AS ts,
         'orders' AS src,
         'order.created' AS type,
         format(
           'status=%s amount=%s %s user=%s',
           status,
           to_char(total_amount, 'FM9999999990.00'),
           currency,
           user_id::text
         ) AS detail
  FROM orders
  WHERE id = $1::uuid
),
oi AS (
  SELECT COALESCE(processed_at, received_at) AS ts,
         'orders_inbox' AS src,
         COALESCE(payload->>'event','inbox') AS type,
         left(payload::text, 200) AS detail
  FROM orders_inbox
  WHERE (payload->>'order_id')::uuid = $1::uuid
),
oo AS (
  SELECT created_at AS ts,
         'orders_outbox' AS src,
         COALESCE(payload->>'event','outbox') AS type,
         concat(topic, ' ', left(payload::text, 160)) AS detail
  FROM orders_outbox
  WHERE agg_id = $1::uuid
),
p AS (
  SELECT created_at AS ts,
         'payments' AS src,
         CASE WHEN status='confirmed' THEN 'payment.confirmed'
              ELSE 'payment.failed' END AS type,
         format(
           'amount=%s %s pay_id=%s',
           to_char(amount_cents/100.0, 'FM999999990.00'),
           currency,
           id::text
         ) AS detail
  FROM payments
  WHERE order_id = $1::uuid
),
pi AS (
  SELECT COALESCE(processed_at, received_at) AS ts,
         'payments_inbox' AS src,
         COALESCE(payload->>'event','inbox') AS type,
         left(payload::text, 200) AS detail
  FROM payments_inbox
  WHERE (payload->>'order_id')::uuid = $1::uuid
),
po AS (
  SELECT created_at AS ts,
         'payments_outbox' AS src,
         COALESCE(payload->>'event','outbox') AS type,
         concat(topic, ' ', left(payload::text, 160)) AS detail
  FROM payments_outbox
  WHERE (payload->>'order_id')::uuid = $1::uuid
)
SELECT to_char(ts AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS at,
       src, type, detail
FROM (
  SELECT * FROM o
  UNION ALL SELECT * FROM oi
  UNION ALL SELECT * FROM oo
  UNION ALL SELECT * FROM p
  UNION ALL SELECT * FROM pi
  UNION ALL SELECT * FROM po
) t
ORDER BY ts ASC;

`
	rows, err := r.db.Query(ctx, q, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.At, &e.Source, &e.Type, &e.Detail); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func RenderMarkdown(events []Event) string {
	if len(events) == 0 {
		return "_Нет событий по этому order_id._"
	}
	var b strings.Builder
	b.WriteString("### Timeline\n")
	for _, e := range events {
		fmt.Fprintf(&b, "- `%s` **%s** (%s) — %s\n", e.At, e.Type, e.Source, e.Detail)
	}
	return b.String()
}
