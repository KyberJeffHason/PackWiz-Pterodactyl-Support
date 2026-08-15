#!/usr/bin/env bash
set -Eeuo pipefail
umask 077
: "${CLOUDFLARE_API_TOKEN:?required}" "${CF_ACCOUNT_ID:?required}" "${CF_ZONE_ID:?required}" "${CF_TUNNEL_ID:?required}" "${CF_HOSTNAME:?required}"
CF_SERVICE_URL="${CF_SERVICE_URL:-http://127.0.0.1:8091}"
[[ $CF_ACCOUNT_ID =~ ^[A-Za-z0-9_-]{16,64}$ && $CF_ZONE_ID =~ ^[A-Za-z0-9_-]{16,64}$ && $CF_TUNNEL_ID =~ ^[A-Fa-f0-9-]{32,36}$ ]]||{ echo 'Invalid Cloudflare identifier.' >&2;exit 2; }
[[ $CF_HOSTNAME =~ ^[A-Za-z0-9.-]+$ && $CF_SERVICE_URL =~ ^https?://([A-Za-z0-9-]+\.)*[A-Za-z0-9-]+(:[0-9]{1,5})?$ ]]||{ echo 'Invalid hostname or service URL.' >&2;exit 2; }
command -v curl >/dev/null;command -v jq >/dev/null
CF_TMP="$(mktemp -d)";trap 'rm -rf -- "$CF_TMP"' EXIT
printf 'header = "Authorization: Bearer %s"\nheader = "Content-Type: application/json"\nsilent\nshow-error\nfail-with-body\n' "$CLOUDFLARE_API_TOKEN" >"$CF_TMP/curl.conf"
cf(){ local method="$1" url="$2" body="${3:-}";if [[ -n $body ]];then curl --config "$CF_TMP/curl.conf" --request "$method" --data-binary "@$body" "$url";else curl --config "$CF_TMP/curl.conf" --request "$method" "$url";fi; }
API=https://api.cloudflare.com/client/v4;TUNNEL="$API/accounts/$CF_ACCOUNT_ID/cfd_tunnel/$CF_TUNNEL_ID/configurations"
cf GET "$TUNNEL" >"$CF_TMP/original.json";jq -e '.success == true and (.result.config.ingress|type=="array") and .result.config.ingress[-1].service == "http_status:404"' "$CF_TMP/original.json" >/dev/null||{ echo 'Tunnel must be remotely managed with final 404 catch-all.' >&2;exit 1; }
jq '{config:.result.config}' "$CF_TMP/original.json" >"$CF_TMP/rollback.json"
jq --arg host "$CF_HOSTNAME" --arg service "$CF_SERVICE_URL" '{config:(.result.config|.ingress=((.ingress[0:-1]|map(select(.hostname != $host)))+[{hostname:$host,service:$service}]+[.ingress[-1]]))}' "$CF_TMP/original.json" >"$CF_TMP/update.json"
cf PUT "$TUNNEL" "$CF_TMP/update.json" >"$CF_TMP/tunnel-result.json";jq -e '.success == true' "$CF_TMP/tunnel-result.json" >/dev/null||{ echo 'Tunnel update failed.' >&2;exit 1; }
rollback(){ cf PUT "$TUNNEL" "$CF_TMP/rollback.json" >/dev/null||true; };DNS="$API/zones/$CF_ZONE_ID/dns_records";TARGET="$CF_TUNNEL_ID.cfargotunnel.com"
if ! cf GET "$DNS?name=$CF_HOSTNAME" >"$CF_TMP/dns-list.json"||! jq -e '.success == true' "$CF_TMP/dns-list.json" >/dev/null;then rollback;echo 'DNS lookup failed.' >&2;exit 1;fi
if jq -e '.result[]?|select(.type != "CNAME")' "$CF_TMP/dns-list.json" >/dev/null;then rollback;echo 'Conflicting non-CNAME DNS record exists.' >&2;exit 1;fi
jq -n --arg name "$CF_HOSTNAME" --arg content "$TARGET" '{type:"CNAME",name:$name,content:$content,proxied:true,ttl:1}' >"$CF_TMP/dns.json";RECORD="$(jq -r '.result[]?|select(.type=="CNAME")|.id' "$CF_TMP/dns-list.json"|head -1)"
if [[ -n $RECORD ]];then ACTION=PATCH;DNS_URL="$DNS/$RECORD";else ACTION=POST;DNS_URL="$DNS";fi
if ! cf "$ACTION" "$DNS_URL" "$CF_TMP/dns.json" >"$CF_TMP/dns-result.json"||! jq -e '.success == true' "$CF_TMP/dns-result.json" >/dev/null;then rollback;echo 'DNS update failed; tunnel configuration restored.' >&2;exit 1;fi
echo "Cloudflare tunnel configured for $CF_HOSTNAME."
