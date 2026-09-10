#!/usr/bin/env bash
# =============================================================
# Feedi 本地一键启动/停止脚本（api + worker 同时拉起）
#
#   用法：
#     ./dev.sh            # 等同 start
#     ./dev.sh start      # 构建并启动 api + worker（后台，日志落 backend/.run/logs）
#     ./dev.sh stop       # 停止 api + worker
#     ./dev.sh restart    # 先停再启
#     ./dev.sh status     # 查看两者运行状态
#     ./dev.sh logs       # 打印两侧最近日志（api.log / worker.log）
#
#   说明：
#     - 必须在 backend/ 目录运行 api.exe / worker.exe（config.yaml 与 .run 均相对该目录）
#     - 依赖本机 MySQL(3306) / Redis(6379) / RabbitMQ(5672)，缺任一会告警
#     - PID 记录于 backend/.run/{api,worker}.pid
# =============================================================
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND="$ROOT/backend"
RUN_DIR="$BACKEND/.run"
LOG_DIR="$RUN_DIR/logs"
API_PID="$RUN_DIR/api.pid"
WORKER_PID="$RUN_DIR/worker.pid"
API_PORT=9000

mkdir -p "$LOG_DIR" "$RUN_DIR"

is_running() {
  local f="$1"
  [ -f "$f" ] && kill -0 "$(cat "$f" 2>/dev/null)" 2>/dev/null
}

wait_health() {
  for _ in $(seq 1 30); do
    curl -sf "http://localhost:${API_PORT}/healthz" >/dev/null 2>&1 && return 0
    sleep 1
  done
  return 1
}

check_deps() {
  local ports=("3306:MySQL" "6379:Redis" "5672:RabbitMQ")
  for item in "${ports[@]}"; do
    local p="${item%%:*}" name="${item##*:}"
    if ! (echo > "/dev/tcp/localhost/$p") >/dev/null 2>&1; then
      echo "[warn] $name 端口 $p 未监听 —— 请先启动中间件"
    fi
  done
}

start() {
  if is_running "$API_PID" || is_running "$WORKER_PID"; then
    echo "[skip] 已有进程在运行（存在有效 pid），如需重启请先: ./dev.sh stop"
    status
    return 0
  fi

  check_deps

  cd "$BACKEND" || { echo "[error] 无法进入 $BACKEND"; exit 1; }

  echo "[build] 构建 api.exe / worker.exe ..."
  if ! go build -o api.exe ./cmd/api; then
    echo "[error] api 构建失败"; exit 1
  fi
  if ! go build -o worker.exe ./cmd/worker; then
    echo "[error] worker 构建失败"; exit 1
  fi

  echo "[start] api  -> http://localhost:${API_PORT}  (log: $LOG_DIR/api.log)"
  nohup ./api.exe >"$LOG_DIR/api.log" 2>&1 &
  echo $! >"$API_PID"

  if ! wait_health; then
    echo "[error] api 30s 内未就绪，最近日志："
    tail -30 "$LOG_DIR/api.log"
    kill "$(cat "$API_PID")" 2>/dev/null
    rm -f "$API_PID"
    exit 1
  fi

  echo "[start] worker（timeline/like/comment/popularity 消费者）  (log: $LOG_DIR/worker.log)"
  nohup ./worker.exe >"$LOG_DIR/worker.log" 2>&1 &
  echo $! >"$WORKER_PID"

  sleep 1
  status
}

stop() {
  local any=0
  if is_running "$WORKER_PID"; then
    kill "$(cat "$WORKER_PID")" 2>/dev/null
    rm -f "$WORKER_PID"
    echo "[stop] worker 已停止"
    any=1
  fi
  if is_running "$API_PID"; then
    kill "$(cat "$API_PID")" 2>/dev/null
    rm -f "$API_PID"
    echo "[stop] api 已停止"
    any=1
  fi
  [ "$any" -eq 0 ] && echo "[stop] 没有正在运行的进程"
}

status() {
  local api_up=no worker_up=no
  is_running "$API_PID" && api_up=yes
  is_running "$WORKER_PID" && worker_up=yes
  echo "api     : $api_up   (http://localhost:${API_PORT})"
  echo "worker  : $worker_up"
  if [ "$api_up" = no ] && [ -f "$API_PID" ]; then
    echo "[hint] 存在 api pid 文件但进程未运行（可能上次会话已退出，可 ./dev.sh start 重启）"
  fi
  return 0
}

logs() {
  echo "===== api.log ====="
  tail -40 "$LOG_DIR/api.log" 2>/dev/null || echo "(无日志)"
  echo
  echo "===== worker.log ====="
  tail -40 "$LOG_DIR/worker.log" 2>/dev/null || echo "(无日志)"
}

case "${1:-start}" in
  start)   start ;;
  stop)    stop ;;
  restart) stop; sleep 1; start ;;
  status)  status ;;
  logs)    logs ;;
  *)       echo "用法: $0 [start|stop|restart|status|logs]"; exit 1 ;;
esac
