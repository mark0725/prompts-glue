#!/usr/bin/env bash
# 构建 Web/Manager 前端，把产物放到 hub-server 的运行时静态目录，再编译后端。
# 用法: ./build.sh
# 跳过 npm ci（复用已有 node_modules）: SKIP_NPM_INSTALL=1 ./build.sh
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

for app in web manager; do
  echo "==> 构建 $app"
  (
    cd "$root/$app"
    if [[ "${SKIP_NPM_INSTALL:-0}" != "1" ]]; then
      npm ci
    fi
    npm run build
  )
done

echo "==> 复制前端产物到 hub-server"
rm -rf "$root/hub-server/web" "$root/hub-server/manager"
cp -R "$root/web/dist" "$root/hub-server/web"
cp -R "$root/manager/dist" "$root/hub-server/manager"

echo "==> 编译 hub-server"
mkdir -p "$root/bin"
(
  cd "$root/hub-server"
  go build -o "$root/bin/hub-server" .
)

echo "完成: $root/bin/hub-server"
echo "静态目录: $root/hub-server/web, $root/hub-server/manager"
