#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
CSV="${CSV:-testdata/inventory.csv}"

tokens="$(go run ./tools/devtoken)"
user="$(sed -n '2p' <<<"$tokens")"
admin="$(sed -n '5p' <<<"$tokens")"

json() { python3 -c "import json,sys; print(json.load(sys.stdin)$1)"; }

echo ">> upload $CSV"
import_id="$(curl -sf -H "Authorization: Bearer $admin" -F "file=@$CSV" "$BASE_URL/api/v1/imports?confirm=true" | json "['import_id']")"
echo "   import $import_id"

for _ in $(seq 1 60); do
  status="$(curl -sf -H "Authorization: Bearer $admin" "$BASE_URL/api/v1/imports/$import_id" | json "['status']")"
  [[ "$status" == COMPLETED* || "$status" == FAILED ]] && break
  sleep 2
done
echo "   status $status"
[[ "$status" == COMPLETED* ]]

echo ">> dashboard"
curl -sf -H "Authorization: Bearer $admin" "$BASE_URL/api/v1/dashboard"
echo

echo ">> findings by type"
curl -sf -H "Authorization: Bearer $admin" "$BASE_URL/api/v1/compliance/summary" | json "['by_type']"

echo ">> user cannot read the license balance"
code="$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $user" "$BASE_URL/api/v1/balance")"
echo "   http $code"
[[ "$code" == 403 ]]

echo "smoke test passed"
