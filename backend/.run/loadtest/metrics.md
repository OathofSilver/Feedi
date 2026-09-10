# Feed 读路径压测量化指标（JMeter 5.6.3）

- 日期：2026-09-10 ｜ 环境：本机（Windows，api.exe 与 JMeter 同机）
- 目标：Go + Gin + Redis + MySQL 读路径；JWT 鉴权；`limit=10`
- 场景：50/100 并发持续 60s，每轮混合 4 个接口（latest / detail / popularity / likes_count 各约 1/4 流量）
- 结果文件：`fix_50.jtl`、`fix_100.jtl`、HTML 报告 `report_100/index.html`

## 最终指标（修复 GORM 连接池后，0 错误）

| 并发 | 指标 | feed/latest | video/detail | feed/popularity | feed/likes_count | 整体 |
|---|---|---|---|---|---|---|
| 50 | QPS | 1166 | 1165 | 1165 | 1165 | **4662** |
| 50 | P50 | 10ms | 6ms | 16ms | 5ms | 9ms |
| 50 | P99 | 25ms | 16ms | 34ms | 16ms | — |
| 100 | QPS | 1044 | 1043 | 1043 | 1042 | **4173** |
| 100 | P50 | 24ms | 15ms | 38ms | 9ms | 21ms |
| 100 | P99 | 53ms | 35ms | 77ms | 29ms | — |

- 50 并发：279,267 次请求 / 60s，错误 0
- 100 并发：249,971 次请求 / 60s，错误 0

## 修复前 vs 修复后（发现的问题）

| 并发 | 修复前吞吐 | 修复前错误率 | 修复后吞吐 | 修复后错误率 |
|---|---|---|---|---|
| 50 | 1048/s | 0.88% | 4662/s | 0% |
| 100 | 612/s | 6.66% | 4162/s | 0% |

**根因**：GORM 默认 `MaxIdleConns=2`，高并发下形成 MySQL 短连接风暴——压测期间捕获 12,374 个发往 3306 的 TIME_WAIT；本机 TCP 动态端口仅 13977 个，端口耗尽导致新建连接失败、接口 500，空闲约 2 分钟后自愈（易误判为负载瓶颈）。

**修复**（`internal/utils/mysql/mysql.go`）：`SetMaxOpenConns(100) / SetMaxIdleConns(50) / SetConnMaxLifetime(1h)`，常驻长连接池消除短连接。

## 可直接写入简历的表述

> 使用 JMeter 对读路径做容量压测（50/100 并发 × 60s × 4 接口混合流量）：定位出 GORM 默认 MaxIdleConns=2 引发的 MySQL 短连接风暴（TIME_WAIT 端口耗尽导致间歇性 500），通过配置连接池常驻长连接修复；修复后单机吞吐 4,100+ QPS（100 并发，0 错误），Feed 列表 P99 53ms、视频详情 P99 35ms。

## 复测方法

```bash
cd C:/Users/SF/tools/apache-jmeter-5.6.3/bin
./jmeter.bat -n -t feed_loadtest.jmx -Jthreads=100 -Jrampup=10 -Jduration=60 \
  -l fix_100.jtl -e -o report_100
```
