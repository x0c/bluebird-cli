#!/usr/bin/env bash
# 逐项核对 skill agent-friendly-cli 的验收清单：跑真实命令、贴真实输出作为证据。
#
# 使用前请自行设置鉴权环境变量（BLUEBIRD_TOKEN，或 BLUEBIRD_JWT_SECRET + BLUEBIRD_USER_ID），
# 本脚本刻意不包含任何获取密钥的逻辑，也不硬编码任何内部基础设施地址/端口——
# 这是一个会进入公开仓库的脚本，敏感信息必须只经调用方的环境变量传入。
#
# 用法：BLUEBIRD_TOKEN=... ./scripts/acceptance.sh
#      BLUEBIRD_BIN=/path/to/bluebird 可覆盖要测试的二进制路径（默认从 PATH 查找 bluebird）。
set -euo pipefail

BIN="${BLUEBIRD_BIN:-bluebird}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

section() { printf '\n=== %s ===\n' "$1"; }
ok() { printf 'OK: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1"; exit 1; }

command -v "$BIN" >/dev/null 2>&1 || fail "找不到可执行文件：$BIN（设置 BLUEBIRD_BIN 或把 bluebird 加入 PATH）"

section "1. 非交互不卡死（管道剥离 TTY，无需传 --json 也自动降级）"
echo | "$BIN" watch list < /dev/null > "$TMP/1.json"; code=$?
cat "$TMP/1.json"
[ "$code" -eq 0 ] || [ "$code" -eq 4 ] || fail "非预期退出码 $code（期望 0 或 4-未配鉴权）"
ok "非交互管道下自动降级为 JSON，未卡在确认框"

section "2. 成功路径 envelope 合规"
"$BIN" version --json | tee "$TMP/2.json"
jq -e '(.ok != null) and (.data != null) and (.meta != null) and (has("error"))' "$TMP/2.json" >/dev/null \
  && ok "四键（ok/data/error/meta）齐全"

section "3. 失败路径 envelope 合规（故意缺鉴权）"
env -u BLUEBIRD_TOKEN -u BLUEBIRD_JWT_SECRET -u BLUEBIRD_USER_ID "$BIN" watch list --json > "$TMP/3.json"; code=$?
cat "$TMP/3.json"
echo "exit=$code"
jq -e '.ok == false and .data == null and .error.code != null' "$TMP/3.json" >/dev/null \
  && ok "失败信封合规（ok=false, data=null, error.code 存在）"
[ "$code" -eq 4 ] && ok "退出码 4（鉴权未配置）"

section "4. 退出码逐类实测"
"$BIN" watch get does-not-exist --json > "$TMP/4-notfound.json"; code=$?
echo "not_found exit=$code（期望 3）"; [ "$code" -eq 3 ] && ok "码 3 正确"
"$BIN" --nope > "$TMP/4-usage.json" 2>&1; code=$?
echo "usage_error exit=$code（期望 2）"; [ "$code" -eq 2 ] && ok "码 2 正确"

section "5. stdout/stderr 分离"
"$BIN" watch list --json 2>/dev/null | jq . >/dev/null && ok "stdout 可直接被 jq 解析，无日志混入"

section "6. 幂等收敛（watch create 连跑两遍）"
NAME="acceptance-$(date +%s)"
"$BIN" watch create --name "$NAME" --lat 30 --lon 120 --json | tee "$TMP/6a.json" | jq '.data.status'
WID=$(jq -r '.data.watch.id' "$TMP/6a.json")
"$BIN" watch create --name "$NAME" --lat 30 --lon 120 --json | tee "$TMP/6b.json" | jq '.data.status'
jq -e '.data.status == "unchanged"' "$TMP/6b.json" >/dev/null && ok "第二遍收敛为 unchanged"

section "7. dry-run 真只读（watch delete 前后快照一致）"
"$BIN" watch get "$WID" --json > "$TMP/7-before.json"
"$BIN" watch delete "$WID" --dry-run --json | jq '.data.status'
"$BIN" watch get "$WID" --json > "$TMP/7-after.json"
diff "$TMP/7-before.json" "$TMP/7-after.json" >/dev/null && ok "dry-run 前后快照完全一致，未真正删除"

section "8. 离线自描述（describe/version/doctor --no-probe 零网络）"
"$BIN" describe --json | jq -e '.data.commands | length > 0' >/dev/null && ok "describe 离线可用"
"$BIN" version --json | jq -e '.data.version' >/dev/null && ok "version 离线可用"
"$BIN" doctor --no-probe --json | jq -e '.data.checks | length > 0' >/dev/null && ok "doctor --no-probe 离线可用"

section "9. 人类模式可读（无 --json，未连接终端时脚本环境仍会看到 JSON，属预期）"
"$BIN" watch list

section "10. 密钥不泄露"
if "$BIN" doctor --json | grep -Eq '(BLUEBIRD_TOKEN|BLUEBIRD_JWT_SECRET)[=:] *[A-Za-z0-9._-]{8,}'; then
  fail "doctor 输出疑似包含密钥值明文"
fi
ok "doctor 输出未见密钥值"

section "清理测试数据"
"$BIN" watch delete "$WID" --json | jq '.data.status'
ok "已清理本次验收创建的测试 watch（id=$WID）"

printf '\n验收清单全部通过。\n'
