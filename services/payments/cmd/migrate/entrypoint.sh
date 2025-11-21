#!/bin/sh
set -eu

: "${RETRIES:=120}"
: "${SLEEP_SECONDS:=2}"

i=1
echo "payments-migrate: wait DB by running migrate, retries=$RETRIES..."
while [ "$i" -le "$RETRIES" ]; do
  if migrate version >/dev/null 2>&1; then
    echo "payments-migrate: DB reachable, applying migrations..."
    migrate up && migrate status
    exit 0
  fi

  if [ "$i" -eq 1 ]; then
    echo "payments-migrate: first attempt failed, printing error:"
    migrate version || true
  fi

  echo "payments-migrate: not ready yet (try $i/$RETRIES), sleeping ${SLEEP_SECONDS}s"
  i=$((i+1))
  sleep "$SLEEP_SECONDS"
done

echo "payments-migrate: failed after $RETRIES retries"
migrate status || true
exit 1
