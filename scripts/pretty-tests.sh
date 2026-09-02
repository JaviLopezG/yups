#!/usr/bin/env bash
#
# Pretty printer for `go test -v` output. Reads the test log on stdin.
#
#   - Colours PASS (green), FAIL (red) and SKIP (yellow).
#   - Splits CamelCase test names so they are readable
#     (TestInstallTwiceReportsAlreadyInstalled -> Test Install Twice Reports
#     Already Installed). Acronyms stay together (PATH, OS).
#   - Indents nested tests by depth: subtest lines (the ones containing "/")
#     get one extra indent level per slash.
#   - Subtests are rendered by their leaf name only (TestX/sub -> sub): the
#     parent is printed once right above them, repeating it adds noise.
#     Indentation still reflects the full nesting depth.
#   - With a terminal on stdout (or REPAINT=always) it does not stream the
#     interleaved output of parallel tests; instead it keeps a live view that
#     is fully repainted on every new line. The summary only lists FAIL and
#     SKIP results; when everything passed it reports that there is nothing
#     to review.
#
# Environment:
#   REPAINT=auto|always|never   default "auto": repaint only on a tty.
set -u

GREEN=$(printf '\033[32m')
RED=$(printf '\033[31m')
YELLOW=$(printf '\033[33m')
RESET=$(printf '\033[0m')

REPAINT=${REPAINT:-auto}
case "$REPAINT" in
  always) repaint=1 ;;
  never)  repaint=0 ;;
  *)      if [ -t 1 ]; then repaint=1; else repaint=0; fi ;;
esac
colour=$repaint  # colour only when we own the terminal (or forced)

declare -A runSeen resultSeen
runOrder=()
resultOrder=()
declare -A results
misc=()

# split-camel inserts a space before capitals that follow a letter or digit,
# and before a capital followed by capital+lowercase (so PATH and OS survive
# but PATHDir becomes PATH Dir).
split-camel() {
  printf '%s' "$1" | sed -e 's/\([a-z0-9]\)\([A-Z]\)/\1 \2/g' \
                        -e 's/\([A-Z]\)\([A-Z][a-z]\)/\1 \2/g'
}

trim() { # strip leading whitespace
  printf '%s' "${1#"${1%%[![:space:]]*}"}"
}

leaf-name() { # subtests are rendered below their already-printed parent,
              # so only the last segment of the name is worth repeating
  local name
  name=$(trim "$1")
  printf '%s' "${name##*/}"
}

indent-for() { # one indent level (4 spaces) per "/" in the test name
  local slashes
  slashes=$(printf '%s' "$1" | tr -cd '/')
  printf '%*s' $(( ${#slashes} * 4 )) ''
}

colour-status() { # $1 = PASS|FAIL|SKIP
  case "$1" in
    PASS) [ "$colour" -eq 1 ] && printf '%s' "$GREEN$1$RESET" || printf '%s' "$1" ;;
    FAIL) [ "$colour" -eq 1 ] && printf '%s' "$RED$1$RESET" || printf '%s' "$1" ;;
    SKIP) [ "$colour" -eq 1 ] && printf '%s' "$YELLOW$1$RESET" || printf '%s' "$1" ;;
    *)    printf '%s' "$1" ;;
  esac
}

render-run() { # $1 = full test name
  local full name
  full=$(trim "$1")
  name=$(split-camel "$(leaf-name "$full")")
  printf '%s=== RUN %s\n' "$(indent-for "$(trim "$1")")" "$name"
}

render-result() { # $1 = status, $2 = full test name, $3 = is summary
  local status name
  status=$(colour-status "$1")
  name=$(split-camel "$(leaf-name "$2")")
  printf '%s--- %s: %s\n' "$(indent-for "$(trim "$2")")" "$status" "$name"
}

print-summary() {
  local name status shown=0
  echo '================ SUMMARY ==================='
  for name in "${resultOrder[@]}"; do
    status=${results[$name]}
    [ "$status" = PASS ] && continue   # the summary only lists trouble
    render-result "$status" "$name" 1
    shown=1
  done
  if [ "$shown" -eq 0 ]; then
    if [ "$colour" -eq 1 ]; then
      printf '%snothing to review: every test passed%s\n' "$GREEN" "$RESET"
    else
      echo 'nothing to review: every test passed'
    fi
  fi
}

stream-line() { # non-repainting mode: decorate and print as they arrive
  local line=$1
  case "$line" in
    '=== RUN '*)
      render-run "${line#'=== RUN '}" ;;
    '--- PASS: '*)
      render-result PASS "${line#'--- PASS: '}" ;;
    '--- FAIL: '*)
      render-result FAIL "${line#'--- FAIL: '}" ;;
    '--- SKIP: '*)
      render-result SKIP "${line#'--- SKIP: '}" ;;
    '=== PAUSE '*|'=== CONT '*)
      : ;;
    *)
      printf '%s\n' "$line" ;;
  esac
}

while IFS= read -r raw; do
  # go test indents subtest lines; trim leading whitespace so every line
  # matches the classifiers below (depth indentation is re-added later).
  line=${raw#"${raw%%[![:space:]]*}"}
  case "$line" in
    '=== RUN '*)
      name=${line#'=== RUN '}
      if [[ -z ${runSeen[$name]+x} ]]; then
        runSeen[$name]=1
        runOrder+=("$name")
      fi
      ;;
    '--- PASS: '*|'--- FAIL: '*|'--- SKIP: '*)
      rest=${line#'--- '}
      status=${rest%%:*}
      name=${rest#*: }
      name=${name%% (*}
      name=${name% }        # trailing space when there is no duration
      if [[ -z ${resultSeen[$name]+x} ]]; then
        resultSeen[$name]=1
        resultOrder+=("$name")
      fi
      results[$name]=$status
      ;;
    '=== PAUSE '*|'=== CONT '*)
      ;;
    '')
      ;;
    *)
      misc+=("$(printf '%s' "$line" | tr -d '\r')")
      ;;
  esac

  stream-line "$line"
done

if [ "$repaint" -eq 1 ]; then
  print-summary
fi
