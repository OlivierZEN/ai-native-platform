#!/bin/sh
set -eu

container=ai-native-platform-pg16-test
port=55432
action=${1:-run}

start() {
	if docker container inspect "$container" >/dev/null 2>&1; then
		return
	fi
	docker run --rm -d \
		--name "$container" \
		-e POSTGRES_HOST_AUTH_METHOD=trust \
		-p "127.0.0.1:$port:5432" \
		postgres:16 >/dev/null
	attempt=0
	while ! docker exec "$container" pg_isready -U postgres >/dev/null 2>&1; do
		attempt=$((attempt + 1))
		if [ "$attempt" -ge 30 ]; then
			docker logs "$container" >&2
			return 1
		fi
		sleep 1
	done
}

stop() {
	if docker container inspect "$container" >/dev/null 2>&1; then
		docker rm -f "$container" >/dev/null
	fi
}

case "$action" in
	start)
		start
		;;
	stop)
		stop
		;;
	run)
		start
		trap stop EXIT INT TERM
		TEST_DATABASE_URL="postgres://postgres@127.0.0.1:$port/postgres?sslmode=disable" \
			GOTOOLCHAIN=go1.26.5 go test ./... -count=1
		;;
	*)
		echo "usage: scripts/test-postgres.sh [start|stop|run]" >&2
		exit 2
		;;
esac
