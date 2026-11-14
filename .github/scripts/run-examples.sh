#!/usr/bin/env bash
#
# Runs every long-running program under examples/: builds it, starts it, waits
# for it to serve, sends it SIGTERM and requires it to exit with status 0.
#
# The examples listen on fixed ports (8080, 8081, 8443, 9001 and 9002), which
# must be free.

# The functions below report failure through their return status, and their
# callers test it, so none of them depends on errexit.
# shellcheck disable=SC2310

set -o errexit -o nounset -o pipefail

readonly START_TIMEOUT_SECONDS=20
readonly STOP_TIMEOUT_SECONDS=20

# Each entry is "<example>|<probe>|<arguments>". A probe is a URL that must
# answer 200, "tcp:<port>", a local port that must accept a connection, or
# "log:<text>", text the example must have logged, for one with no listener.
readonly EXAMPLES=(
  "basic-http-server|http://127.0.0.1:8080/hello|"
  "rate-limiting-http|http://127.0.0.1:8080/limited|"
  "rate-limiting-dynamic|http://127.0.0.1:8080/standard|"
  "rate-limiting-network|tcp:8080|"
  "ip-acl|tcp:9001|"
  "tls-config|https://127.0.0.1:8443/|"
  "api-server|http://127.0.0.1:8081/health/ready|-drain-delay 1s"
  "worker-daemon|log:worker started|"
)

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
readonly repo_root
bin_dir="$(mktemp -d)"
readonly bin_dir
pid=""

cleanup() {
  if [[ -n "${pid}" ]]; then
    kill -KILL "${pid}" 2>/dev/null || true
  fi
  rm -rf "${bin_dir}"
}
trap cleanup EXIT

# probe reports whether the example answers yet. log is where the running
# example writes its output.
probe() {
  local target="$1" log="$2"

  if [[ "${target}" == log:* ]]; then
    grep --quiet --fixed-strings -- "${target#log:}" "${log}"
    return
  fi

  if [[ "${target}" == tcp:* ]]; then
    (exec 3<>"/dev/tcp/127.0.0.1/${target#tcp:}") 2>/dev/null
    return
  fi

  # --insecure: tls-config serves a self-signed certificate.
  [[ "$(curl --silent --insecure --output /dev/null --write-out '%{http_code}' "${target}" || true)" == "200" ]]
}

# wait_until_serving waits for the probe to pass while the example is running.
wait_until_serving() {
  local name="$1" target="$2" log="$3" deadline=$((SECONDS + START_TIMEOUT_SECONDS))

  until probe "${target}" "${log}"; do
    if ! kill -0 "${pid}" 2>/dev/null; then
      echo "FAIL ${name}: exited before serving ${target}" >&2
      return 1
    fi
    if ((SECONDS >= deadline)); then
      echo "FAIL ${name}: not serving ${target} after ${START_TIMEOUT_SECONDS}s" >&2
      return 1
    fi
    sleep 0.2
  done
}

# stop sends SIGTERM and requires the example to exit with status 0 in time.
stop() {
  local name="$1" status=0 deadline=$((SECONDS + STOP_TIMEOUT_SECONDS))

  kill -TERM "${pid}"
  while kill -0 "${pid}" 2>/dev/null; do
    if ((SECONDS >= deadline)); then
      echo "FAIL ${name}: still running ${STOP_TIMEOUT_SECONDS}s after SIGTERM" >&2
      return 1
    fi
    sleep 0.2
  done

  wait "${pid}" || status=$?
  pid=""
  if ((status != 0)); then
    echo "FAIL ${name}: exited with status ${status} after SIGTERM" >&2
    return 1
  fi
}

run_example() {
  local name="$1" target="$2" arguments="$3" log="${bin_dir}/$1.log"
  local -a args=()

  if [[ -n "${arguments}" ]]; then
    read -r -a args <<<"${arguments}"
  fi

  if ! go build -o "${bin_dir}/${name}" "./examples/${name}"; then
    echo "FAIL ${name}: did not build" >&2
    return 1
  fi
  "${bin_dir}/${name}" "${args[@]+"${args[@]}"}" >"${log}" 2>&1 &
  pid=$!

  if wait_until_serving "${name}" "${target}" "${log}" && stop "${name}"; then
    echo "ok   ${name}"
    return 0
  fi

  cat "${log}" >&2
  return 1
}

main() {
  local entry name target arguments failed=0

  cd "${repo_root}"

  for entry in "${EXAMPLES[@]}"; do
    IFS='|' read -r name target arguments <<<"${entry}"
    run_example "${name}" "${target}" "${arguments}" || failed=1
  done

  # Every directory under examples/ must be listed above, so that a new
  # example cannot be added without being run.
  for dir in examples/*/; do
    name="$(basename "${dir}")"
    if [[ " ${EXAMPLES[*]} " != *" ${name}|"* ]]; then
      echo "FAIL ${name}: not listed in $(basename "${BASH_SOURCE[0]}")" >&2
      failed=1
    fi
  done

  return "${failed}"
}

main "$@"
