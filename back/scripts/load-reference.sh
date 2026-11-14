#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080/api/v1}"
DIR="${DIR:-testdata/reference}"
TOKEN="${TOKEN:-$(go run ./tools/devtoken | sed -n '5p')}"

api() {
  local method=$1 path=$2 body=${3:-}
  curl -sS -m 90 --retry 3 --retry-all-errors -o /tmp/load-reference.json -w '%{http_code}' -X "$method" \
    -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' ${body:+-d "$body"} "$BASE_URL$path"
}

report() {
  case $1 in
    200 | 201) created=$((created + 1)) ;;
    409) skipped=$((skipped + 1)) ;;
    *) failed=$((failed + 1)); echo "   $1 $2: $(jq -r '.error.message // .' /tmp/load-reference.json)" ;;
  esac
}

rows() { jq -Rc 'split("\t")' "$DIR/$1" | tail -n +2; }

field() { jq -r ".[$2] // \"\"" <<<"$1"; }

lookup() {
  api GET "$1?limit=200" >/dev/null
  jq -r ".items[] | [.id, .$2] | @tsv" /tmp/load-reference.json
}

id_of() { awk -F'\t' -v n="$2" '$2 == n { print $1; exit }' <<<"$1"; }

section() { created=0 skipped=0 failed=0; echo ">> $1"; }
summary() { echo "   created $created, already present $skipped, failed $failed"; }

section agreements
agreements=$(lookup /agreements name)
while read -r row; do
  name=$(field "$row" 0) type=$(field "$row" 1) cost=$(field "$row" 2)
  if [ -n "$(id_of "$agreements" "$name")" ]; then skipped=$((skipped + 1)); continue; fi
  report "$(api POST /agreements "$(jq -nc --arg n "$name" --arg t "$type" --arg c "$cost" \
    '{name: $n, license_type: $t, cost_model: $c}')")" "$name"
done < <(rows agreements.tsv)
summary
agreements=$(lookup /agreements name)

section products
while read -r row; do
  name=$(field "$row" 0) publisher=$(field "$row" 1) current=$(field "$row" 2) purpose=$(field "$row" 3)
  sec=$(field "$row" 4) cls=$(field "$row" 5) bpo=$(field "$row" 6) status=$(field "$row" 7)
  comment=$(field "$row" 8) agreement=$(field "$row" 9)
  report "$(api POST /software "$(jq -nc --arg n "$name" --arg pb "$publisher" --arg v "$current" --arg p "$purpose" \
    --arg s "$sec" --arg c "$cls" --argjson b "$bpo" --arg st "$status" --arg cm "$comment" \
    --arg a "$(id_of "$agreements" "$agreement")" '
    {canonical_name: $n, current_version: $v, purpose: $p, section: $s, class: $c, in_bpo: $b, status: $st}
    + (if $pb == "" then {} else {publisher: $pb} end)
    + (if $cm == "" then {} else {status_comment: $cm} end)
    + (if $a == "" then {} else {agreement_id: $a} end)')")" "$name"
done < <(rows products.tsv)
summary
products=$(lookup /software canonical_name)

section versions
while read -r row; do
  product=$(field "$row" 0) version=$(field "$row" 1)
  report "$(api POST "/software/$(id_of "$products" "$product")/versions" \
    "$(jq -nc --arg v "$version" '{version: $v, lifecycle_status: "CURRENT"}')")" "$product $version"
done < <(rows versions.tsv)
summary

section contracts
existing=$(api GET "/contracts?limit=200" >/dev/null; jq -r '.items[].purchase_reference // empty' /tmp/load-reference.json)
while read -r row; do
  product=$(field "$row" 0) agreement=$(field "$row" 1) quantity=$(field "$row" 2)
  from=$(field "$row" 3) to=$(field "$row" 4) number=$(field "$row" 5)
  if grep -qxF "$number" <<<"$existing"; then skipped=$((skipped + 1)); continue; fi
  report "$(api POST /contracts "$(jq -nc --arg p "$(id_of "$products" "$product")" \
    --arg a "$(id_of "$agreements" "$agreement")" --argjson q "$quantity" --arg f "$from" --arg t "$to" --arg n "$number" '
    {product_id: $p, agreement_id: $a, license_metric: "PER_DEVICE", quantity: $q, valid_from: $f, purchase_reference: $n}
    + (if $t == "" then {} else {expires_at: $t} end)')")" "$number"
done < <(rows contracts.tsv)
summary

section rules
while read -r row; do
  raw=$(field "$row" 0) publisher=$(field "$row" 1) version=$(field "$row" 2) product=$(field "$row" 3)
  report "$(api POST /matching/rules "$(jq -nc --arg r "$raw" --arg pb "$publisher" --arg v "$version" \
    --arg p "$(id_of "$products" "$product")" '{raw_name: $r, raw_publisher: $pb, raw_version: $v, product_id: $p}')")" "$raw"
done < <(rows rules.tsv)
summary
